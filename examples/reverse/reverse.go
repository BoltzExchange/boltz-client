package main

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"

	"github.com/BoltzExchange/boltz-client/v2/pkg/boltz"
	"github.com/btcsuite/btcd/btcec/v2"
)

const endpoint = "<Boltz API endpoint>"
const invoiceAmount = 100000
const destinationAddress = "<address to which the swap should be claimed>"

// Swap from Lightning to BTC mainchain
var toCurrency = boltz.CurrencyBtc

var network = boltz.Regtest

func printJson(v interface{}) {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	fmt.Println(string(b))
}

func reverseSwap() error {
	ourKeys, err := btcec.NewPrivateKey()
	if err != nil {
		return err
	}

	preimage := make([]byte, 32)
	_, err = rand.Read(preimage)
	if err != nil {
		return err
	}
	preimageHash := sha256.Sum256(preimage)

	boltzApi := &boltz.Api{URL: endpoint}

	reversePairs, err := boltzApi.GetReversePairs()
	if err != nil {
		return fmt.Errorf("could not get reverse pairs: %s", err)
	}

	pair := boltz.Pair{From: boltz.CurrencyBtc, To: toCurrency}
	pairInfo, err := boltz.FindPair(pair, reversePairs)
	if err != nil {
		return fmt.Errorf("could not find reverse pair: %s", err)
	}

	fees := pairInfo.Fees
	serviceFee := boltz.Percentage(fees.Percentage)
	fmt.Printf("Service Fee: %dsat\n", boltz.CalculatePercentage(serviceFee, invoiceAmount))
	fmt.Printf("Network Fee: %dsat\n", fees.MinerFees.Lockup+fees.MinerFees.Claim)

	swap, err := boltzApi.CreateReverseSwap(boltz.CreateReverseSwapRequest{
		From:           boltz.CurrencyBtc,
		To:             toCurrency,
		ClaimPublicKey: ourKeys.PubKey().SerializeCompressed(),
		PreimageHash:   preimageHash[:],
		InvoiceAmount:  invoiceAmount,
		PairHash:       pairInfo.Hash,
	})
	if err != nil {
		return fmt.Errorf("Could not create swap: %s", err)
	}

	boltzPubKey, err := btcec.ParsePubKey(swap.RefundPublicKey)
	if err != nil {
		return err
	}

	tree := swap.SwapTree.Deserialize()
	if err := tree.Init(toCurrency, false, ourKeys, boltzPubKey); err != nil {
		return err
	}

	if err := tree.Check(boltz.ReverseSwap, swap.TimeoutBlockHeight, preimageHash[:]); err != nil {
		return err
	}

	fmt.Println("Swap created")
	printJson(swap)

	boltzWs := boltzApi.NewWebsocket()
	if err := boltzWs.Connect(); err != nil {
		return fmt.Errorf("Could not connect to Boltz websocket: %w", err)
	}

	if err := boltzWs.Subscribe([]string{swap.Id}); err != nil {
		return err
	}

	for update := range boltzWs.Updates {
		parsedStatus := boltz.ParseEvent(update.Status)

		printJson(update)

		switch parsedStatus {
		case boltz.SwapCreated:
			fmt.Println("Waiting for invoice to be paid")
			break

		case boltz.TransactionMempool:
			lockupTransaction, err := boltz.NewTxFromHex(toCurrency, update.Transaction.Hex, nil)
			if err != nil {
				return err
			}

			vout, _, err := lockupTransaction.FindVout(network, swap.LockupAddress)
			if err != nil {
				return err
			}

			satPerVbyte := float64(2)
			claimTransaction, _, err := boltz.ConstructTransaction(
				network,
				boltz.CurrencyBtc,
				[]boltz.OutputDetails{
					{
						SwapId:            swap.Id,
						SwapType:          boltz.ReverseSwap,
						Address:           destinationAddress,
						LockupTransaction: lockupTransaction,
						Vout:              vout,
						Preimage:          preimage,
						PrivateKey:        ourKeys,
						SwapTree:          tree,
						Cooperative:       true,
					},
				},
				satPerVbyte,
				boltzApi,
			)
			if err != nil {
				return fmt.Errorf("could not create claim transaction: %w", err)
			}

			txHex, err := claimTransaction.Serialize()
			if err != nil {
				return fmt.Errorf("could not serialize claim transaction: %w", err)
			}

			txId, err := boltzApi.BroadcastTransaction(toCurrency, txHex)
			if err != nil {
				return fmt.Errorf("could not broadcast transaction: %w", err)
			}

			fmt.Printf("Broadcast claim transaction: %s\n", txId)
			break

		case boltz.InvoiceSettled:
			fmt.Println("Swap succeeded", swap.Id)
			if err := boltzWs.Close(); err != nil {
				return err
			}
			break
		}
	}
	return nil
}

func main() {
	if err := reverseSwap(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
