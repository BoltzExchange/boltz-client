package boltz

import (
	"bytes"
	"errors"
	"fmt"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcec/v2/schnorr"
	"github.com/btcsuite/btcd/btcec/v2/schnorr/musig2"
	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/txscript"
	"github.com/vulpemventures/go-elements/payment"
	"github.com/vulpemventures/go-elements/taproot"
)

type FundingTree struct {
	refundLeaf TapLeaf

	isLiquid      bool
	ourKey        *btcec.PrivateKey
	boltzKey      *btcec.PublicKey
	aggregateKey  *musig2.AggregateKey
	indexed       *txscript.IndexedTapScriptTree
	liquidIndexed *taproot.IndexedElementsTapScriptTree
	taprootTweak  musig2.KeyTweakDesc
}

func NewFundingTree(
	currency Currency,
	ourKey *btcec.PrivateKey,
	boltzKey *btcec.PublicKey,
	timeoutBlockHeight uint32,
) (*FundingTree, error) {
	if currency != CurrencyBtc && currency != CurrencyLiquid {
		return nil, errors.New("invalid currency")
	}

	isLiquid := currency == CurrencyLiquid
	leafVer := leafVersion(isLiquid)

	refundScript := txscript.NewScriptBuilder()
	refundScript.AddData(toXOnly(ourKey.PubKey()))
	refundScript.AddOp(txscript.OP_CHECKSIGVERIFY)
	refundScript.AddInt64(int64(timeoutBlockHeight))
	refundScript.AddOp(txscript.OP_CHECKLOCKTIMEVERIFY)

	refundBytes, err := refundScript.Script()
	if err != nil {
		return nil, fmt.Errorf("failed to build refund script: %w", err)
	}

	refundLeaf := TapLeaf{
		LeafVersion: leafVer,
		Script:      refundBytes,
	}

	tree := &FundingTree{
		refundLeaf: refundLeaf,
		isLiquid:   isLiquid,
		ourKey:     ourKey,
		boltzKey:   boltzKey,
	}

	var rootNode txscript.TapNode
	if isLiquid {
		tree.liquidIndexed = taproot.AssembleTaprootScriptTree(
			taproot.TapElementsLeaf{TapLeaf: refundLeaf},
		)
		rootNode = tree.liquidIndexed.RootNode
	} else {
		tree.indexed = txscript.AssembleTaprootScriptTree(refundLeaf)
		rootNode = tree.indexed.RootNode
	}

	scriptRoot := rootNode.TapHash()
	// boltz key always comes first
	keys := []*btcec.PublicKey{boltzKey, ourKey.PubKey()}

	// only aggreagte the internal key initially
	internalKey, _, _, err := musig2.AggregateKeys(keys, false)
	if err != nil {
		return nil, fmt.Errorf("failed to aggregate keys: %w", err)
	}

	// after we got the internal key, we can compute the taproot tweak
	tag := chainhash.TagTapTweak
	if tree.isLiquid {
		tag = taproot.TagTapTweakElements
	}
	tapTweakHash := chainhash.TaggedHash(
		tag, schnorr.SerializePubKey(internalKey.FinalKey), scriptRoot[:],
	)
	tree.taprootTweak = musig2.KeyTweakDesc{
		Tweak:   *tapTweakHash,
		IsXOnly: true,
	}
	tree.aggregateKey, _, _, err = musig2.AggregateKeys(keys, false, musig2.WithKeyTweaks(tree.taprootTweak))

	return tree, nil
}

func (tree *FundingTree) Address(network *Network, blindingPubKey *btcec.PublicKey) (string, error) {
	key := tree.aggregateKey.FinalKey
	if tree.isLiquid {
		p2tr, err := payment.FromTweakedKey(key, network.Liquid, blindingPubKey)
		if err != nil {
			return "", err
		}

		return p2tr.ConfidentialTaprootAddress()
	} else {
		address, err := btcutil.NewAddressTaproot(toXOnly(key), network.Btc)
		if err != nil {
			return "", err
		}
		return address.EncodeAddress(), nil
	}
}

func (tree *FundingTree) CheckAddress(expected string, network *Network, blindingPubKey *btcec.PublicKey) error {
	encoded, err := tree.Address(network, blindingPubKey)
	if err != nil {
		return err
	}
	if encoded != expected {
		return fmt.Errorf("expected address %v, got %v", expected, encoded)
	}
	return nil
}

func (tree *FundingTree) GetControlBlock() ([]byte, error) {
	leafHash := tree.GetLeafHash()
	internalKey := tree.aggregateKey.PreTweakedKey
	if tree.isLiquid {
		idx := tree.liquidIndexed.LeafProofIndex[leafHash]
		controlBlock := tree.liquidIndexed.LeafMerkleProofs[idx].ToControlBlock(internalKey)
		return controlBlock.ToBytes()
	} else {
		idx := tree.indexed.LeafProofIndex[leafHash]
		controlBlock := tree.indexed.LeafMerkleProofs[idx].ToControlBlock(internalKey)
		return controlBlock.ToBytes()
	}
}

func (tree *FundingTree) GetLeafHash() chainhash.Hash {
	if tree.isLiquid {
		return liquidLeaf(tree.refundLeaf).TapHash()
	}
	return tree.refundLeaf.TapHash()
}

func (tree *FundingTree) GetLeaf() TapLeaf {
	return tree.refundLeaf
}

// FundingMusigSession wraps a musig2 session for signing funding tree transactions
type FundingMusigSession struct {
	*musig2.Session
	tree *FundingTree
}

// NewFundingSigningSession creates a new musig2 signing session for the funding tree
func NewFundingSigningSession(tree *FundingTree) (*FundingMusigSession, error) {
	ctx, err := musig2.NewContext(
		tree.ourKey,
		false,
		musig2.WithTweakedContext(tree.taprootTweak),
		musig2.WithKnownSigners([]*btcec.PublicKey{tree.boltzKey, tree.ourKey.PubKey()}),
	)
	if err != nil {
		return nil, err
	}

	session, err := ctx.NewSession()
	if err != nil {
		return nil, err
	}

	return &FundingMusigSession{session, tree}, nil
}

// decodePartialSignature decodes a partial signature from hex bytes
func decodePartialSignature(sig HexString) (*musig2.PartialSignature, error) {
	partial := &musig2.PartialSignature{}
	if err := partial.Decode(bytes.NewReader(sig)); err != nil {
		return nil, err
	}
	return partial, nil
}

// Sign creates a partial signature for the given transaction hash using the boltz nonce
func (session *FundingMusigSession) Sign(signingDetails *FundingAddressSigningDetails) (*PartialSignature, error) {
	hash := signingDetails.TransactionHash
	boltzNonce := signingDetails.PubNonce

	if len(hash) != 32 {
		return nil, fmt.Errorf("invalid hash length %d", len(hash))
	}

	if len(boltzNonce) != 66 {
		return nil, fmt.Errorf("invalid nonce length %d", len(boltzNonce))
	}

	all, err := session.RegisterPubNonce([66]byte(boltzNonce))
	if err != nil {
		return nil, err
	}
	if !all {
		return nil, fmt.Errorf("could not combine nonces")
	}

	ourNonce := session.PublicNonce()

	partial, err := session.Session.Sign([32]byte(hash))
	if err != nil {
		return nil, err
	}

	b := bytes.NewBuffer(nil)
	if err := partial.Encode(b); err != nil {
		return nil, err
	}

	return &PartialSignature{
		PubNonce:         HexString(ourNonce[:]),
		PartialSignature: HexString(b.Bytes()),
	}, nil
}
