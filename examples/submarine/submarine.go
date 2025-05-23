package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"

	"github.com/btcsuite/btcd/btcec/v2"

	"github.com/BoltzExchange/boltz-client/v2/pkg/boltz"
	"github.com/lightningnetwork/lnd/zpay32"
)

const endpoint = "<Boltz API endpoint>"
const invoice = "<the invoice you want to pay"

var network = boltz.Regtest

func printJson(v any) {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	fmt.Println(string(b))
}

func submarineSwap() error {
	keys, err := btcec.NewPrivateKey()
	if err != nil {
		return err
	}

	boltzApi := &boltz.Api{URL: endpoint}

	submarinePairs, err := boltzApi.GetSubmarinePairs()
	if err != nil {
		return fmt.Errorf("Could not get submarine pairs: %s", err)
	}

	pair, err := boltz.FindPair(boltz.Pair{From: boltz.CurrencyBtc, To: boltz.CurrencyBtc}, submarinePairs)
	if err != nil {
		return fmt.Errorf("Could not find submarine pair: %s", err)
	}

	decodedInvoice, err := zpay32.Decode(invoice, network.Btc)
	if err != nil {
		return fmt.Errorf("could not decode invoice: %s", err)
	}
	invoiceAmount := *decodedInvoice.MilliSat / 1000

	fees := pair.Fees
	serviceFee := boltz.Percentage(fees.Percentage)
	fmt.Printf("Service Fee: %dsat\n", boltz.CalculatePercentage(serviceFee, invoiceAmount))
	fmt.Printf("Network Fee: %dsat\n", fees.MinerFees)

	swap, err := boltzApi.CreateSwap(boltz.CreateSwapRequest{
		From:            boltz.CurrencyBtc,
		To:              boltz.CurrencyBtc,
		RefundPublicKey: keys.PubKey().SerializeCompressed(),
		Invoice:         invoice,
		PairHash:        pair.Hash,
	})
	if err != nil {
		return fmt.Errorf("Could not create swap: %s", err)
	}

	boltzPubKey, err := btcec.ParsePubKey(swap.ClaimPublicKey)
	if err != nil {
		return err
	}

	tree := swap.SwapTree.Deserialize()
	if err := tree.Init(boltz.CurrencyBtc, false, keys, boltzPubKey); err != nil {
		return err
	}

	// Check the scripts of the Taptree to make sure Boltz is not cheating
	if err := tree.Check(boltz.NormalSwap, swap.TimeoutBlockHeight, decodedInvoice.PaymentHash[:]); err != nil {
		return err
	}

	// Verify that Boltz is giving us the correct address
	if err := tree.CheckAddress(swap.Address, network, nil); err != nil {
		return err
	}

	fmt.Println("Swap created")
	printJson(swap)

	boltzWs := boltzApi.NewWebsocket()
	if err := boltzWs.Connect(); err != nil {
		return fmt.Errorf("Could not connect to Boltz websocket: %s", err)
	}

	if err := boltzWs.Subscribe([]string{swap.Id}); err != nil {
		return err
	}

	for update := range boltzWs.Updates {
		parsedStatus := boltz.ParseEvent(update.Status)

		printJson(update)

		switch parsedStatus {
		case boltz.InvoiceSet:
			fmt.Println("Waiting for onchain transaction")
			break

		case boltz.TransactionMempool:
			fmt.Println("Boltz found transaction in mempool")
			break

		case boltz.TransactionConfirmed:
			fmt.Println("Boltz found transaction in blockchain")
			break

		case boltz.TransactionClaimPending:
			// Create a partial signature to allow Boltz to do a key path spend to claim the onchain coins
			claimDetails, err := boltzApi.GetSwapClaimDetails(swap.Id)
			if err != nil {
				return fmt.Errorf("Could not get claim details from Boltz: %s", err)
			}

			// Verify that the invoice was actually paid
			preimageHash := sha256.Sum256(claimDetails.Preimage)
			if !bytes.Equal(decodedInvoice.PaymentHash[:], preimageHash[:]) {
				return fmt.Errorf("Boltz returned wrong preimage: %x", claimDetails.Preimage)
			}

			session, err := boltz.NewSigningSession(tree)
			partial, err := session.Sign(claimDetails.TransactionHash, claimDetails.PubNonce)
			if err != nil {
				return fmt.Errorf("could not create partial signature: %s", err)
			}

			if err := boltzApi.SendSwapClaimSignature(swap.Id, partial); err != nil {
				return fmt.Errorf("could not send partial signature to Boltz: %s", err)
			}
			break

		case boltz.TransactionClaimed:
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
	if err := submarineSwap(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
