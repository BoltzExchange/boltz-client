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
	"github.com/lightningnetwork/lnd/input"
	"github.com/vulpemventures/go-elements/payment"
	"github.com/vulpemventures/go-elements/taproot"
)

const leafVersionLiquid = 196

type TapLeaf = txscript.TapLeaf

type SwapTree struct {
	ClaimLeaf  TapLeaf
	RefundLeaf TapLeaf

	isLiquid bool
	isClaim  bool
	ourKey   *btcec.PrivateKey
	boltzKey *btcec.PublicKey

	aggregateKey  *musig2.AggregateKey
	rootNode      txscript.TapNode
	indexed       *txscript.IndexedTapScriptTree
	liquidIndexed *taproot.IndexedElementsTapScriptTree
	taprootTweak  musig2.KeyTweakDesc
}

func (tree *SwapTree) Serialize() *SerializedTree {
	if tree == nil {
		return nil
	}
	return &SerializedTree{
		ClaimLeaf: SerializedLeaf{
			Version: tree.ClaimLeaf.LeafVersion,
			Output:  tree.ClaimLeaf.Script,
		},
		RefundLeaf: SerializedLeaf{
			Version: tree.RefundLeaf.LeafVersion,
			Output:  tree.RefundLeaf.Script,
		},
	}
}
func (tree *SwapTree) Init(
	isLiquid bool,
	isClaim bool,
	ourKey *btcec.PrivateKey,
	boltzKey *btcec.PublicKey,
) error {
	tree.isLiquid = isLiquid
	tree.isClaim = isClaim
	tree.ourKey = ourKey
	tree.boltzKey = boltzKey

	if isLiquid {
		tree.liquidIndexed = taproot.AssembleTaprootScriptTree(
			taproot.TapElementsLeaf{TapLeaf: tree.ClaimLeaf},
			taproot.TapElementsLeaf{TapLeaf: tree.RefundLeaf},
		)
		tree.rootNode = tree.liquidIndexed.RootNode
	} else {
		tree.indexed = txscript.AssembleTaprootScriptTree(tree.ClaimLeaf, tree.RefundLeaf)
		tree.rootNode = tree.indexed.RootNode
	}

	scriptRoot := tree.rootNode.TapHash()

	// boltz key always comes first
	keys := []*btcec.PublicKey{boltzKey, ourKey.PubKey()}

	// only aggreagte the internal key initially
	internalKey, _, _, err := musig2.AggregateKeys(keys, false)
	if err != nil {
		return err
	}

	// after we got the internal key, we can compute the taproot tweak
	tag := chainhash.TagTapTweak
	if isLiquid {
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
	return err
}

func (tree *SwapTree) PubKey() *btcec.PublicKey {
	return tree.aggregateKey.FinalKey
}

func (tree *SwapTree) Address(network *Network, blindingPubKey *btcec.PublicKey) (string, error) {
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

func (tree *SwapTree) CheckAddress(expected string, network *Network, blindingPubKey *btcec.PublicKey) error {
	encoded, err := tree.Address(network, blindingPubKey)
	if err != nil {
		return err
	}
	if encoded != expected {
		return fmt.Errorf("expected address %v, got %v", expected, encoded)
	}
	return nil
}

func (tree *SwapTree) GetControlBlock(isRefund bool) ([]byte, error) {
	internalKey := tree.aggregateKey.PreTweakedKey
	leafHash := tree.GetLeafHash(isRefund)
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

func (tree *SwapTree) GetLeafHash(isRefund bool) chainhash.Hash {
	leaf := tree.GetLeaf(isRefund)
	if tree.isLiquid {
		return liquidLeaf(leaf).TapHash()
	}
	return leaf.TapHash()
}

func (tree *SwapTree) GetLeaf(isRefund bool) TapLeaf {
	if isRefund {
		return tree.RefundLeaf
	} else {
		return tree.ClaimLeaf
	}
}

func (tree *SwapTree) checkLeafVersions() error {
	leafVersion := leafVersion(tree.isLiquid)
	if tree.RefundLeaf.LeafVersion != leafVersion || tree.ClaimLeaf.LeafVersion != leafVersion {
		return errors.New("invalid leaf version")
	}
	return nil
}

func (tree *SwapTree) Check(
	swapType SwapType,
	timeoutBlockHeight uint32,
	preimageHash []byte,
) error {
	if err := tree.checkLeafVersions(); err != nil {
		return err
	}

	claimPubKey := toXOnly(tree.claimPubKey())
	claim := txscript.NewScriptBuilder()
	if swapType == ReverseSwap || swapType == ChainSwap {
		claim.AddOp(txscript.OP_SIZE)
		claim.AddInt64(32)
		claim.AddOp(txscript.OP_EQUALVERIFY)
		claim.AddOp(txscript.OP_HASH160)
		claim.AddData(input.Ripemd160H(preimageHash))
		claim.AddOp(txscript.OP_EQUALVERIFY)
		claim.AddData(claimPubKey)
		claim.AddOp(txscript.OP_CHECKSIG)
	} else if swapType == NormalSwap {
		claim.AddOp(txscript.OP_HASH160)
		claim.AddData(input.Ripemd160H(preimageHash))
		claim.AddOp(txscript.OP_EQUALVERIFY)
		claim.AddData(claimPubKey)
		claim.AddOp(txscript.OP_CHECKSIG)
	}

	if err := checkScript(tree.ClaimLeaf.Script, claim); err != nil {
		return err
	}

	refund := txscript.NewScriptBuilder()
	refund.AddData(toXOnly(tree.refundPubKey()))
	refund.AddOp(txscript.OP_CHECKSIGVERIFY)
	refund.AddInt64(int64(timeoutBlockHeight))
	refund.AddOp(txscript.OP_CHECKLOCKTIMEVERIFY)

	if err := checkScript(tree.RefundLeaf.Script, refund); err != nil {
		return err
	}

	return nil
}

func (tree *SwapTree) claimPubKey() *btcec.PublicKey {
	if tree.isClaim {
		return tree.ourKey.PubKey()
	}
	return tree.boltzKey
}

func (tree *SwapTree) refundPubKey() *btcec.PublicKey {
	if tree.isClaim {
		return tree.boltzKey
	}
	return tree.ourKey.PubKey()
}

func toXOnly(publicKey *btcec.PublicKey) []byte {
	serialized := publicKey.SerializeCompressed()
	return serialized[1:33]
}

func checkScript(actual []byte, expected *txscript.ScriptBuilder) error {
	expectedScript, err := expected.Script()
	if err != nil {
		return errors.New("expected script is invalid")
	}

	if !bytes.Equal(actual, expectedScript) {
		return errors.New("invalid script")
	}
	return nil
}

func liquidLeaf(leaf txscript.TapLeaf) taproot.TapElementsLeaf {
	return taproot.TapElementsLeaf{TapLeaf: leaf}
}

func leafVersion(isLiuqid bool) txscript.TapscriptLeafVersion {
	if isLiuqid {
		return leafVersionLiquid
	}
	return txscript.BaseLeafVersion
}

type SerializedLeaf struct {
	Version txscript.TapscriptLeafVersion `json:"version"`
	Output  HexString                     `json:"output"`
}

type SerializedTree struct {
	ClaimLeaf  SerializedLeaf `json:"claimLeaf"`
	RefundLeaf SerializedLeaf `json:"refundLeaf"`
}

func (leaf *SerializedLeaf) Deserialize() TapLeaf {
	return TapLeaf{
		LeafVersion: leaf.Version,
		Script:      leaf.Output,
	}
}

func (tree *SerializedTree) Deserialize() *SwapTree {
	return &SwapTree{
		ClaimLeaf:  tree.ClaimLeaf.Deserialize(),
		RefundLeaf: tree.RefundLeaf.Deserialize(),
	}
}
