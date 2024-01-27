package boltz

import (
	"bytes"
	"encoding/hex"
	"errors"
	"fmt"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcec/v2/schnorr/musig2"
	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/txscript"
	"github.com/lightningnetwork/lnd/input"
	"github.com/vulpemventures/go-elements/taproot"
)

const leafVersionLiquid = 196

type SwapTree struct {
	ClaimLeaf  txscript.TapLeaf
	RefundLeaf txscript.TapLeaf
	IsLiquid   bool

	aggregateKey  *musig2.AggregateKey
	rootNode      txscript.TapNode
	indexed       *txscript.IndexedTapScriptTree
	liquidIndexed *taproot.IndexedElementsTapScriptTree
}

func NewSwapTree(
	isLiquid bool,
	isReverse bool,
	claimPubKey *btcec.PublicKey,
	refundPubKey *btcec.PublicKey,
	preimageHash []byte,
	timeoutBlockHeight uint32,
) (*SwapTree, error) {
	tree := &SwapTree{
		IsLiquid: isLiquid,
	}

	var err error
	tree.RefundLeaf, err = tree.createRefundLeaf(timeoutBlockHeight, refundPubKey)
	if err != nil {
		return nil, err
	}
	tree.ClaimLeaf, err = tree.createClaimLeaf(isReverse, preimageHash, claimPubKey)
	if err != nil {
		return nil, err
	}

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

	// boltz key always comes first
	keys := []*btcec.PublicKey{claimPubKey, refundPubKey}
	if isReverse {
		keys = []*btcec.PublicKey{refundPubKey, claimPubKey}
	}
	scriptRoot := tree.rootNode.TapHash()
	tree.aggregateKey, _, _, err = musig2.AggregateKeys(keys, false, musig2.WithTaprootKeyTweak(scriptRoot[:]))
	if err != nil {
		return nil, err
	}
	return tree, nil
}

func (tree *SwapTree) PubKey() *btcec.PublicKey {
	return tree.aggregateKey.FinalKey
}

func (tree *SwapTree) Address(network *Network) (string, error) {
	if tree.IsLiquid {
		return "", nil

	} else {
		address, err := btcutil.NewAddressTaproot(toXOnly(tree.aggregateKey.FinalKey), network.Btc)
		if err != nil {
			return "", err
		}
		return address.EncodeAddress(), nil
	}
}

func (tree *SwapTree) GetControlBlock(leaf txscript.TapNode) ([]byte, error) {
	internalKey := tree.aggregateKey.PreTweakedKey
	if tree.IsLiquid {
		idx := tree.liquidIndexed.LeafProofIndex[leaf.TapHash()]
		controlBlock := tree.liquidIndexed.LeafMerkleProofs[idx].ToControlBlock(internalKey)
		return controlBlock.ToBytes()
	} else {
		idx := tree.indexed.LeafProofIndex[leaf.TapHash()]
		controlBlock := tree.indexed.LeafMerkleProofs[idx].ToControlBlock(internalKey)
		return controlBlock.ToBytes()
	}
}

func (tree *SwapTree) GetLeaf(isRefund bool) txscript.TapLeaf {
	if isRefund {
		return tree.RefundLeaf
	} else {
		return tree.ClaimLeaf
	}
}

func (tree *SwapTree) createLeaf(builder *txscript.ScriptBuilder) (txscript.TapLeaf, error) {
	script, err := builder.Script()
	leaf := txscript.TapLeaf{Script: script}
	if tree.IsLiquid {
		leaf.LeafVersion = leafVersionLiquid
	} else {
		leaf.LeafVersion = txscript.BaseLeafVersion
	}
	return leaf, err
}

func (tree *SwapTree) createRefundLeaf(
	timeoutBlockHeight uint32,
	refundPubKey *btcec.PublicKey,
) (txscript.TapLeaf, error) {
	refund := txscript.NewScriptBuilder()

	refund.AddData(toXOnly(refundPubKey))
	refund.AddOp(txscript.OP_CHECKSIGVERIFY)
	refund.AddInt64(int64(timeoutBlockHeight))
	refund.AddOp(txscript.OP_CHECKLOCKTIMEVERIFY)

	return tree.createLeaf(refund)
}

func (tree *SwapTree) createClaimLeaf(
	isReverse bool,
	preimageHash []byte,
	claimPubKey *btcec.PublicKey,
) (txscript.TapLeaf, error) {
	claim := txscript.NewScriptBuilder()

	if isReverse {
		claim.AddOp(txscript.OP_SIZE)
		claim.AddInt64(32)
		claim.AddOp(txscript.OP_EQUALVERIFY)
		claim.AddOp(txscript.OP_HASH160)
		claim.AddData(input.Ripemd160H(preimageHash))
		claim.AddOp(txscript.OP_EQUALVERIFY)
		claim.AddData(toXOnly(claimPubKey))
		claim.AddOp(txscript.OP_CHECKSIG)

	} else {
		claim.AddOp(txscript.OP_HASH160)
		claim.AddData(input.Ripemd160H(preimageHash))
		claim.AddOp(txscript.OP_EQUALVERIFY)
		claim.AddData(toXOnly(claimPubKey))
		claim.AddOp(txscript.OP_CHECKSIG)
	}

	return tree.createLeaf(claim)
}

func (tree *SwapTree) checkRefundLeaf(
	refundPublicKey *btcec.PublicKey,
	timeoutBlockHeight uint32,
) error {
	refund := txscript.NewScriptBuilder()
	refund.AddData(toXOnly(refundPublicKey))
	refund.AddOp(txscript.OP_CHECKSIGVERIFY)
	refund.AddInt64(int64(timeoutBlockHeight))
	refund.AddOp(txscript.OP_CHECKLOCKTIMEVERIFY)

	if err := checkScript(tree.RefundLeaf.Script, refund); err != nil {
		fmt.Println("refund...")
		return err
	}

	return nil
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
		fmt.Println("expected...")
		fmt.Println(txscript.DisasmString(expectedScript))
		fmt.Println("actual...")
		fmt.Println(txscript.DisasmString(actual))
		return errors.New("invalid script")
	}
	return nil
}

func ParseLeaf(version txscript.TapscriptLeafVersion, script string) (txscript.TapLeaf, error) {
	decoded, err := hex.DecodeString(script)
	leaf := txscript.TapLeaf{
		LeafVersion: version,
		Script:      decoded,
	}
	return leaf, err
}
