package boltz

import (
	"encoding/hex"
	"errors"
	"fmt"

	"github.com/btcsuite/btcd/btcec/v2/schnorr/musig2"
	"github.com/btcsuite/btcd/wire"
	"github.com/decred/dcrd/dcrec/secp256k1/v4"
	liquidtx "github.com/vulpemventures/go-elements/transaction"
)

type MusigSession struct {
	*musig2.Session
	output *OutputDetails
}

func NewSigningSession(output *OutputDetails) (*MusigSession, error) {
	tree := output.SwapTree
	ctx, err := musig2.NewContext(
		tree.ourKey,
		false,
		musig2.WithTweakedContext(tree.taprootTweak),
		musig2.WithKnownSigners([]*secp256k1.PublicKey{tree.boltzKey, tree.ourKey.PubKey()}),
	)
	if err != nil {
		return nil, err
	}

	session, err := ctx.NewSession()
	if err != nil {
		return nil, err
	}

	return &MusigSession{session, output}, nil
}

func (session *MusigSession) Finalize(transaction Transaction, network *Network, boltzSignature *PartialSignature) error {
	partialSignature, err := hex.DecodeString(boltzSignature.PartialSignature)
	if err != nil {
		return err
	}

	nonce, err := hex.DecodeString(boltzSignature.PubNonce)
	if err != nil {
		return err
	}

	all, err := session.RegisterPubNonce([66]byte(nonce))
	if err != nil {
		return err
	}
	if !all {
		return errors.New("could not combine nonces")
	}

	var hash [32]byte
	isLiquid := session.output.SwapTree.isLiquid
	if isLiquid {
		hash = liquidTaprootHash(&transaction.(*LiquidTransaction).Transaction, network, session.output, 0, true)
	} else {
		hash, err = btcTaprootHash(transaction, session.output, 0)
	}
	if err != nil {
		return err
	}

	_, err = session.Sign(hash)
	if err != nil {
		return err
	}

	s := &secp256k1.ModNScalar{}
	s.SetByteSlice(partialSignature)
	partial := musig2.NewPartialSignature(s, nil)
	haveFinal, err := session.CombineSig(&partial)
	if err != nil {
		return fmt.Errorf("could not combine signatures: %w", err)
	}
	if !haveFinal {
		return errors.New("could not combine signatures")
	}

	signature := session.FinalSig().Serialize()
	if isLiquid {
		tx := transaction.(*LiquidTransaction)
		tx.Transaction.Inputs[0].Witness = liquidtx.TxWitness{signature}
	} else {
		tx := transaction.(*BtcTransaction)
		tx.MsgTx().TxIn[0].Witness = wire.TxWitness{signature}
	}
	return nil
}
