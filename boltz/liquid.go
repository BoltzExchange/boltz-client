package boltz

import (
	"bytes"
	"encoding/hex"
	"errors"
	"fmt"
	"math"

	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/txscript"
	"github.com/vulpemventures/go-elements/address"
	"github.com/vulpemventures/go-elements/psetv2"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcec/v2/schnorr"
	"github.com/vulpemventures/go-elements/confidential"
	liquidtx "github.com/vulpemventures/go-elements/transaction"
)

type LiquidTransaction struct {
	liquidtx.Transaction
	OurOutputBlindingKey *btcec.PrivateKey
}

func (transaction *LiquidTransaction) Hash() string {
	return transaction.TxHash().String()
}

func (transaction *LiquidTransaction) Serialize() (string, error) {
	claimTxBytes, err := transaction.Transaction.Serialize()

	if err != nil {
		return "", err
	}

	return hex.EncodeToString(claimTxBytes), nil
}

func NewLiquidTxFromHex(hexString string, ourOutputBlindingKey *btcec.PrivateKey) (*LiquidTransaction, error) {
	liquidTx, err := liquidtx.NewTxFromHex(hexString)
	if err != nil {
		return nil, err
	}
	return &LiquidTransaction{
		Transaction:          *liquidTx,
		OurOutputBlindingKey: ourOutputBlindingKey,
	}, nil
}

func (transaction *LiquidTransaction) FindVout(network *Network, addressToFind string) (uint32, uint64, error) {
	info, err := address.FromConfidential(addressToFind)
	if err != nil {
		return 0, 0, err
	}
	for vout, output := range transaction.Outputs {
		if bytes.Equal(info.Script, output.Script) {
			unblinded, err := confidential.UnblindOutputWithKey(output, transaction.OurOutputBlindingKey.Serialize())
			if err != nil {
				return 0, 0, errors.New("Failed to unblind lockup tx: " + err.Error())
			}
			return uint32(vout), unblinded.Value, err
		}
	}

	return 0, 0, errors.New("could not find lockup vout")

}

func (transaction *LiquidTransaction) VSize() uint64 {
	witnessSize := transaction.SerializeSize(true, true) - transaction.SerializeSize(false, true)
	return uint64(transaction.SerializeSize(false, true)) + uint64(math.Ceil(float64(witnessSize)/4))
}

func liquidTaprootHash(transaction *liquidtx.Transaction, network *Network, outputs []OutputDetails, index int, cooperative bool) [32]byte {
	var leafHash *chainhash.Hash
	if !cooperative {
		output := outputs[index]
		hash := output.SwapTree.GetLeafHash(output.IsRefund())
		leafHash = &hash
	}
	genesisHash, _ := chainhash.NewHashFromStr(network.Liquid.GenesisBlockHash)

	var scripts, assets, values [][]byte
	for i, input := range transaction.Inputs {
		lockupTx := outputs[i].LockupTransaction.(*LiquidTransaction)
		out := lockupTx.Outputs[input.Index]

		scripts = append(scripts, out.Script)
		assets = append(assets, out.Asset)
		values = append(values, out.Value)
	}

	return transaction.HashForWitnessV1(
		index,
		scripts,
		assets,
		values,
		sigHashType,
		genesisHash,
		leafHash,
		nil,
	)
}

func constructLiquidTransaction(network *Network, outputs []OutputDetails, outputAddressRaw string, fee uint64) (Transaction, error) {
	var blindingKeyCompressed []byte

	isConfidential, err := address.IsConfidential(outputAddressRaw)
	if err != nil {
		return nil, errors.New("Could not decode address: " + err.Error())
	}

	if isConfidential {
		outputAddress, err := address.FromConfidential(outputAddressRaw)
		if err != nil {
			return nil, errors.New("Could not decode address: " + err.Error())
		}
		blindingKey, err := btcec.ParsePubKey(outputAddress.BlindingKey)
		if err != nil {
			return nil, errors.New("Could not parse blinding key: " + err.Error())
		}
		blindingKeyCompressed = blindingKey.SerializeCompressed()
	}
	script, err := address.ToOutputScript(outputAddressRaw)
	if err != nil {
		return nil, errors.New("Could not generate output script: " + err.Error())
	}

	p, err := psetv2.New(nil, nil, nil)
	if err != nil {
		return nil, err
	}
	updater, err := psetv2.NewUpdater(p)
	if err != nil {
		return nil, err
	}

	var inPrivateBlindingKeys [][]byte

	for i, output := range outputs {
		lockupTx := output.LockupTransaction.(*LiquidTransaction)

		out := lockupTx.Outputs[output.Vout]
		input := psetv2.InputArgs{
			Txid:    output.LockupTransaction.Hash(),
			TxIndex: output.Vout,
			//HeightLock: output.TimeoutBlockHeight,
			Sequence: 0xfffffffd,
		}
		if !output.Cooperative {
			input.HeightLock = output.TimeoutBlockHeight
		}

		if err := updater.AddInputs([]psetv2.InputArgs{input}); err != nil {
			return nil, err
		}

		if err := updater.AddInWitnessUtxo(i, out); err != nil {
			return nil, err
		}

		if err := updater.AddInUtxoRangeProof(i, out.RangeProof); err != nil {
			return nil, err
		}

		if lockupTx.OurOutputBlindingKey != nil {
			if len(inPrivateBlindingKeys) != i {
				return nil, errors.New("Inconsistent blinding")
			}
			inPrivateBlindingKeys = append(inPrivateBlindingKeys, lockupTx.OurOutputBlindingKey.Serialize())
		}
	}

	zkpGenerator := confidential.NewZKPGeneratorFromBlindingKeys(inPrivateBlindingKeys, nil)

	ownedInputs, err := zkpGenerator.UnblindInputs(p, nil)
	if err != nil {
		return nil, errors.New("Failed to unblind inputs: " + err.Error())
	}

	var inputSum uint64
	for _, input := range ownedInputs {
		inputSum += input.Value
	}

	btcAsset := network.Liquid.AssetID

	if err := updater.AddOutputs([]psetv2.OutputArgs{
		{
			Asset:        btcAsset,
			Amount:       inputSum - fee,
			Script:       script,
			BlindingKey:  blindingKeyCompressed,
			BlinderIndex: 0,
		},
		{
			Asset:  btcAsset,
			Amount: fee,
		},
	}); err != nil {
		return nil, err
	}

	if len(inPrivateBlindingKeys) != 0 {
		if blindingKeyCompressed == nil {
			privKey, err := btcec.NewPrivateKey()
			if err != nil {
				return nil, errors.New("Failed to generate private key: " + err.Error())
			}

			if err := updater.AddOutputs([]psetv2.OutputArgs{
				{
					Asset:        btcAsset,
					Script:       []byte{txscript.OP_RETURN},
					BlindingKey:  privKey.PubKey().SerializeCompressed(),
					BlinderIndex: 0,
				},
			}); err != nil {
				return nil, err
			}
		}

		zkpValidator := confidential.NewZKPValidator()

		outputBlindingArgs, err := zkpGenerator.BlindOutputs(p, nil)
		if err != nil {
			return nil, err
		}

		blinder, err := psetv2.NewBlinder(p, ownedInputs, zkpValidator, zkpGenerator)
		if err != nil {
			return nil, err
		}

		err = blinder.BlindLast(nil, outputBlindingArgs)
		if err != nil {
			return nil, errors.New("Failed to blind transaction: " + err.Error())
		}
	}

	tx, err := p.UnsignedTx()
	if err != nil {
		return nil, err
	}

	// Construct the signature script and witnesses and sign the inputs
	for i, output := range outputs {
		var witness [][]byte
		if output.Cooperative {
			witness = append(witness, dummySignature)
		} else {
			sigHash := liquidTaprootHash(tx, network, outputs, i, false)
			signature, err := schnorr.Sign(output.PrivateKey, sigHash[:])
			if err != nil {
				return nil, err
			}

			if !signature.Verify(sigHash[:], output.PrivateKey.PubKey()) {
				return nil, errors.New("Signature verification failed")
			}

			tree := output.SwapTree
			isRefund := output.IsRefund()
			controlBlock, err := tree.GetControlBlock(isRefund)
			if err != nil {
				return nil, err
			}

			witness = [][]byte{signature.Serialize()}
			if !isRefund {
				witness = append(witness, output.Preimage)
			}
			witness = append(witness, tree.GetLeafScript(isRefund), controlBlock)
		}
		p.Inputs[i].FinalScriptWitness, err = writeTxWitness(witness...)
		if err != nil {
			return nil, err
		}
	}

	finalized, err := psetv2.Extract(p)
	if err != nil {
		return nil, fmt.Errorf("could not extract pset: %w", err)
	}

	return &LiquidTransaction{
		Transaction: *finalized,
	}, nil
}

func writeTxWitness(wit ...[]byte) ([]byte, error) {
	b := bytes.NewBuffer(nil)

	if err := b.WriteByte(byte(len(wit))); err != nil {
		return nil, err
	}

	for _, item := range wit {
		if err := b.WriteByte(byte(len(item))); err != nil {
			return nil, err
		}
		if _, err := b.Write(item); err != nil {
			return nil, err

		}
	}
	return b.Bytes(), nil
}
