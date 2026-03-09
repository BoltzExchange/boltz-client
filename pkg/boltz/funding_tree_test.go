package boltz

import (
	"math"
	"testing"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/txscript"
	"github.com/btcsuite/btcd/wire"
	"github.com/stretchr/testify/require"
	"github.com/vulpemventures/go-elements/address"
	"github.com/vulpemventures/go-elements/elementsutil"
	liquidtx "github.com/vulpemventures/go-elements/transaction"
)

func TestValidatePresignedTransaction_BTC(t *testing.T) {
	ourKey, _ := btcec.NewPrivateKey()
	boltzKey, _ := btcec.NewPrivateKey()

	tree, err := NewFundingTree(CurrencyBtc, ourKey, boltzKey.PubKey(), 100)
	require.NoError(t, err)

	expectedAddress, err := tree.Address(Regtest, nil)
	require.NoError(t, err)

	lockupTx := createTestBtcLockupTx(t, expectedAddress, 100000, Regtest)

	t.Run("invalid transaction hex", func(t *testing.T) {
		details := &FundingAddressSigningDetails{
			TransactionHex: "invalid-hex",
		}
		err := validatePresignedTransaction(Regtest, lockupTx, expectedAddress, details)
		require.Error(t, err)
	})

	t.Run("wrong number of outputs", func(t *testing.T) {
		txWithTwoOutputs := createTestBtcTxWithMultipleOutputs(t, lockupTx, expectedAddress, 2, Regtest)
		txHex, err := txWithTwoOutputs.Serialize()
		require.NoError(t, err)

		details := &FundingAddressSigningDetails{TransactionHex: txHex}
		err = validatePresignedTransaction(Regtest, lockupTx, expectedAddress, details)
		require.Error(t, err)
		require.Contains(t, err.Error(), "expected exactly one output")
	})

	t.Run("value mismatch", func(t *testing.T) {
		spendingTx := createTestBtcSpendingTx(t, lockupTx, expectedAddress, 50000, Regtest)
		txHex, err := spendingTx.Serialize()
		require.NoError(t, err)

		details := &FundingAddressSigningDetails{TransactionHex: txHex}
		err = validatePresignedTransaction(Regtest, lockupTx, expectedAddress, details)
		require.Error(t, err)
		require.Contains(t, err.Error(), "expected value")
	})

	t.Run("script mismatch", func(t *testing.T) {
		differentKey, _ := btcec.NewPrivateKey()
		differentAddr, _ := btcutil.NewAddressTaproot(toXOnly(differentKey.PubKey()), Regtest.Btc)

		spendingTx := createTestBtcSpendingTx(t, lockupTx, differentAddr.EncodeAddress(), 100000, Regtest)
		txHex, err := spendingTx.Serialize()
		require.NoError(t, err)

		details := &FundingAddressSigningDetails{TransactionHex: txHex}
		err = validatePresignedTransaction(Regtest, lockupTx, expectedAddress, details)
		require.Error(t, err)
		require.Contains(t, err.Error(), "expected script")
	})

	t.Run("hash mismatch", func(t *testing.T) {
		spendingTx := createTestBtcSpendingTx(t, lockupTx, expectedAddress, 100000, Regtest)
		txHex, err := spendingTx.Serialize()
		require.NoError(t, err)

		details := &FundingAddressSigningDetails{
			TransactionHex:  txHex,
			TransactionHash: HexString(make([]byte, 32)),
		}
		err = validatePresignedTransaction(Regtest, lockupTx, expectedAddress, details)
		require.Error(t, err)
		require.Contains(t, err.Error(), "expected transaction hash")
	})

	t.Run("valid transaction", func(t *testing.T) {
		spendingTx := createTestBtcSpendingTx(t, lockupTx, expectedAddress, 100000, Regtest)
		txHex, err := spendingTx.Serialize()
		require.NoError(t, err)

		outputDetails := []OutputDetails{{LockupTransaction: lockupTx}}
		expectedHash, err := btcTaprootHash(spendingTx, outputDetails, 0)
		require.NoError(t, err)

		details := &FundingAddressSigningDetails{
			TransactionHex:  txHex,
			TransactionHash: HexString(expectedHash),
		}
		err = validatePresignedTransaction(Regtest, lockupTx, expectedAddress, details)
		require.NoError(t, err)
	})
}

func TestValidatePresignedTransaction_Liquid(t *testing.T) {
	ourKey, _ := btcec.NewPrivateKey()
	boltzKey, _ := btcec.NewPrivateKey()
	blindingKey, _ := btcec.NewPrivateKey()

	tree, err := NewFundingTree(CurrencyLiquid, ourKey, boltzKey.PubKey(), 100)
	require.NoError(t, err)

	expectedAddress, err := tree.Address(Regtest, blindingKey.PubKey())
	require.NoError(t, err)

	lockupTx := createTestLiquidLockupTx(t, expectedAddress, 100000, Regtest)

	t.Run("invalid transaction hex", func(t *testing.T) {
		details := &FundingAddressSigningDetails{TransactionHex: "invalid-hex"}
		err := validatePresignedTransaction(Regtest, lockupTx, expectedAddress, details)
		require.Error(t, err)
	})

	t.Run("wrong number of outputs", func(t *testing.T) {
		txWithOneOutput := createTestLiquidTxWithOutputCount(t, lockupTx, expectedAddress, 1, Regtest)
		txHex, err := txWithOneOutput.Serialize()
		require.NoError(t, err)

		details := &FundingAddressSigningDetails{TransactionHex: txHex}
		err = validatePresignedTransaction(Regtest, lockupTx, expectedAddress, details)
		require.Error(t, err)
		require.Contains(t, err.Error(), "expected exactly two outputs")
	})

	t.Run("script mismatch", func(t *testing.T) {
		differentKey, _ := btcec.NewPrivateKey()
		differentBlindingKey, _ := btcec.NewPrivateKey()

		differentTree, err := NewFundingTree(CurrencyLiquid, differentKey, boltzKey.PubKey(), 100)
		require.NoError(t, err)
		differentAddr, err := differentTree.Address(Regtest, differentBlindingKey.PubKey())
		require.NoError(t, err)

		spendingTx := createTestLiquidSpendingTx(t, lockupTx, differentAddr, 100000, Regtest)
		txHex, err := spendingTx.Serialize()
		require.NoError(t, err)

		details := &FundingAddressSigningDetails{TransactionHex: txHex}
		err = validatePresignedTransaction(Regtest, lockupTx, expectedAddress, details)
		require.Error(t, err)
		require.Contains(t, err.Error(), "expected script")
	})

	t.Run("hash mismatch", func(t *testing.T) {
		spendingTx := createTestLiquidSpendingTx(t, lockupTx, expectedAddress, 100000, Regtest)
		txHex, err := spendingTx.Serialize()
		require.NoError(t, err)

		details := &FundingAddressSigningDetails{
			TransactionHex:  txHex,
			TransactionHash: HexString(make([]byte, 32)),
		}
		err = validatePresignedTransaction(Regtest, lockupTx, expectedAddress, details)
		require.Error(t, err)
		require.Contains(t, err.Error(), "expected transaction hash")
	})

	t.Run("valid transaction", func(t *testing.T) {
		spendingTx := createTestLiquidSpendingTx(t, lockupTx, expectedAddress, 100000, Regtest)
		txHex, err := spendingTx.Serialize()
		require.NoError(t, err)

		outputDetails := []OutputDetails{{LockupTransaction: lockupTx}}
		expectedHash := liquidTaprootHash(&spendingTx.Transaction, Regtest, outputDetails, 0, true)

		details := &FundingAddressSigningDetails{
			TransactionHex:  txHex,
			TransactionHash: HexString(expectedHash),
		}
		err = validatePresignedTransaction(Regtest, lockupTx, expectedAddress, details)
		require.NoError(t, err)
	})
}

func mustDecodeAddress(t *testing.T, addr string, network *Network) btcutil.Address {
	decoded, err := btcutil.DecodeAddress(addr, network.Btc)
	require.NoError(t, err)
	return decoded
}

func createTestBtcLockupTx(t *testing.T, toAddress string, value int64, network *Network) *BtcTransaction {
	msgTx := wire.NewMsgTx(wire.TxVersion)
	prevHash, _ := chainhash.NewHashFromStr("0000000000000000000000000000000000000000000000000000000000000001")
	msgTx.AddTxIn(wire.NewTxIn(wire.NewOutPoint(prevHash, 0), nil, nil))

	outputAddress := mustDecodeAddress(t, toAddress, network)
	outputScript, err := txscript.PayToAddrScript(outputAddress)
	require.NoError(t, err)
	msgTx.AddTxOut(wire.NewTxOut(value, outputScript))

	return &BtcTransaction{Tx: *btcutil.NewTx(msgTx)}
}

func createTestBtcSpendingTx(t *testing.T, lockupTx *BtcTransaction, toAddress string, value int64, network *Network) *BtcTransaction {
	msgTx := wire.NewMsgTx(wire.TxVersion)
	lockupMsgTx := lockupTx.MsgTx()
	txHash := lockupMsgTx.TxHash()
	msgTx.AddTxIn(wire.NewTxIn(wire.NewOutPoint(&txHash, 0), nil, nil))

	outputAddress := mustDecodeAddress(t, toAddress, network)
	outputScript, err := txscript.PayToAddrScript(outputAddress)
	require.NoError(t, err)
	msgTx.AddTxOut(wire.NewTxOut(value, outputScript))

	return &BtcTransaction{Tx: *btcutil.NewTx(msgTx)}
}

func createTestBtcTxWithMultipleOutputs(t *testing.T, lockupTx *BtcTransaction, toAddress string, numOutputs int, network *Network) *BtcTransaction {
	msgTx := wire.NewMsgTx(wire.TxVersion)
	lockupMsgTx := lockupTx.MsgTx()
	txHash := lockupMsgTx.TxHash()
	msgTx.AddTxIn(wire.NewTxIn(wire.NewOutPoint(&txHash, 0), nil, nil))

	outputAddress := mustDecodeAddress(t, toAddress, network)
	outputScript, err := txscript.PayToAddrScript(outputAddress)
	require.NoError(t, err)

	for i := 0; i < numOutputs; i++ {
		msgTx.AddTxOut(wire.NewTxOut(50000, outputScript))
	}

	return &BtcTransaction{Tx: *btcutil.NewTx(msgTx)}
}

func createTestLiquidLockupTx(t *testing.T, toAddress string, value uint64, network *Network) *LiquidTransaction {
	tx := liquidtx.NewTx(2)
	prevHash := make([]byte, 32)
	prevHash[0] = 0x01
	tx.Inputs = append(tx.Inputs, liquidtx.NewTxInput(prevHash, 0))

	script, err := address.ToOutputScript(toAddress)
	require.NoError(t, err)

	assetBytes, _ := elementsutil.AssetHashToBytes(network.Liquid.AssetID)
	valueBytes, _ := elementsutil.ValueToBytes(value)
	tx.Outputs = append(tx.Outputs, liquidtx.NewTxOutput(assetBytes, valueBytes, script))

	return &LiquidTransaction{Transaction: *tx}
}

func createTestLiquidSpendingTx(t *testing.T, lockupTx *LiquidTransaction, toAddress string, value uint64, network *Network) *LiquidTransaction {
	tx := liquidtx.NewTx(2)
	lockupHash := lockupTx.TxHash()
	tx.Inputs = append(tx.Inputs, liquidtx.NewTxInput(lockupHash[:], 0))

	script, err := address.ToOutputScript(toAddress)
	require.NoError(t, err)

	assetBytes, _ := elementsutil.AssetHashToBytes(network.Liquid.AssetID)
	valueBytes, _ := elementsutil.ValueToBytes(value)
	tx.Outputs = append(tx.Outputs, liquidtx.NewTxOutput(assetBytes, valueBytes, script))

	placeholderFeeBytes, _ := elementsutil.ValueToBytes(1)
	tx.Outputs = append(tx.Outputs, liquidtx.NewTxOutput(assetBytes, placeholderFeeBytes, nil))

	feeValue := uint64(math.Ceil(float64(tx.DiscountVirtualSize()) * 0.1))
	feeValueBytes, _ := elementsutil.ValueToBytes(feeValue)
	tx.Outputs[1] = liquidtx.NewTxOutput(assetBytes, feeValueBytes, nil)

	return &LiquidTransaction{Transaction: *tx}
}

func createTestLiquidTxWithOutputCount(t *testing.T, lockupTx *LiquidTransaction, toAddress string, numOutputs int, network *Network) *LiquidTransaction {
	tx := liquidtx.NewTx(2)
	lockupHash := lockupTx.TxHash()
	tx.Inputs = append(tx.Inputs, liquidtx.NewTxInput(lockupHash[:], 0))

	script, err := address.ToOutputScript(toAddress)
	require.NoError(t, err)

	assetBytes, _ := elementsutil.AssetHashToBytes(network.Liquid.AssetID)
	valueBytes, _ := elementsutil.ValueToBytes(50000)

	for i := 0; i < numOutputs; i++ {
		tx.Outputs = append(tx.Outputs, liquidtx.NewTxOutput(assetBytes, valueBytes, script))
	}

	return &LiquidTransaction{Transaction: *tx}
}
