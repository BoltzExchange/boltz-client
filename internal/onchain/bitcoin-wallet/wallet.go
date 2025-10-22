package bitcoin_wallet

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/BoltzExchange/boltz-client/v2/internal/onchain"
	"github.com/BoltzExchange/boltz-client/v2/internal/onchain/bitcoin-wallet/bdk"
	"github.com/BoltzExchange/boltz-client/v2/pkg/boltz"
)

type Backend struct {
	*bdk.Backend
	cfg           Config
	ChainProvider onchain.ChainProvider
}

type Wallet struct {
	*bdk.Wallet
	info          onchain.WalletInfo
	chainProvider onchain.ChainProvider
	sendLock      sync.Mutex
}

type Config struct {
	Network       *boltz.Network
	DataDir       string
	Electrum      *onchain.ElectrumOptions
	SyncInterval  time.Duration
	ChainProvider onchain.ChainProvider
}

func NewBackend(cfg Config) (*Backend, error) {
	var url string
	if cfg.Electrum == nil {
		url = "tcp://localhost:19001"
	} else {
		url = cfg.Electrum.Url
		if cfg.Electrum.SSL {
			url = "ssl://" + url
		} else {
			url = "tcp://" + url
		}
	}
	backend, err := bdk.NewBackend(
		convertNetwork(cfg.Network),
		url,
	)
	if err != nil {
		return nil, err
	}
	return &Backend{backend, cfg, cfg.ChainProvider}, nil
}

func convertNetwork(network *boltz.Network) bdk.Network {
	switch network {
	case boltz.MainNet:
		return bdk.NetworkBitcoin
	case boltz.TestNet:
		return bdk.NetworkTestnet
	case boltz.Regtest:
		return bdk.NetworkRegtest
	default:
		panic(fmt.Sprintf("unexpected network %v", *network))
	}
}

func (backend *Backend) NewWallet(credentials *onchain.WalletCredentials) (onchain.Wallet, error) {
	info := credentials.WalletInfo
	info.Currency = boltz.CurrencyBtc
	creds := bdk.WalletCredentials{
		CoreDescriptor: credentials.CoreDescriptor,
	}
	if credentials.Mnemonic != "" {
		creds.Mnemonic = &credentials.Mnemonic
		info.Readonly = false
	}
	descriptorHash := sha256.Sum256([]byte(creds.CoreDescriptor))
	// each wallet requires its own db
	dbName := fmt.Sprintf("bdk-%d-%x.sqlite", info.Id, descriptorHash[:8])
	wallet, err := bdk.NewWallet(
		backend.Backend,
		creds,
		path.Join(backend.cfg.DataDir, dbName),
	)
	if err != nil {
		return nil, err
	}

	return &Wallet{Wallet: wallet, chainProvider: backend.ChainProvider, info: info}, nil
}

func (backend *Backend) DeriveDefaultDescriptor(mnemonic string) (string, error) {
	return bdk.DeriveDefaultXpub(convertNetwork(backend.cfg.Network), mnemonic)
}

func (w *Wallet) NewAddress() (string, error) {
	return w.Wallet.NewAddress()
}

func (w *Wallet) GetBalance() (*onchain.Balance, error) {
	balance, err := w.Balance()
	if err != nil {
		return nil, err
	}
	return &onchain.Balance{
		Total:       balance.Confirmed + balance.Unconfirmed,
		Confirmed:   balance.Confirmed,
		Unconfirmed: balance.Unconfirmed,
	}, nil
}

func (w *Wallet) ApplyTransaction(txHex string) error {
	return w.Wallet.ApplyTransaction(txHex)
}

func (w *Wallet) broadcastTransaction(txHex string) (string, error) {
	txId, err := w.chainProvider.BroadcastTransaction(txHex)
	if err != nil {
		return "", err
	}
	if err := w.Wallet.ApplyTransaction(txHex); err != nil {
		return "", err
	}
	return txId, nil
}

func (w *Wallet) BumpTransactionFee(txId string, satPerVbyte float64) (string, error) {
	txHex, err := w.Wallet.BumpTransactionFee(txId, satPerVbyte)
	if err != nil {
		return "", err
	}
	return w.broadcastTransaction(txHex)
}

func (w *Wallet) sendToAddress(args onchain.WalletSendArgs) (*bdk.WalletSendResult, error) {
	result, err := w.Wallet.SendToAddress(
		args.Address,
		args.Amount,
		args.SatPerVbyte,
		args.SendAll,
	)
	if err != nil {
		if strings.Contains(err.Error(), "Insufficient funds") {
			return nil, w.info.InsufficientBalanceError(args.Amount)
		}
		return nil, err
	}
	return &result, nil
}

func (w *Wallet) SendToAddress(args onchain.WalletSendArgs) (string, error) {
	w.sendLock.Lock()
	defer w.sendLock.Unlock()

	result, err := w.sendToAddress(args)
	if err != nil {
		return "", err
	}
	return w.broadcastTransaction(result.TxHex)
}

func (w *Wallet) Sync() error {
	return w.Wallet.Sync()
}

func (w *Wallet) GetSendFee(args onchain.WalletSendArgs) (send uint64, fee uint64, err error) {
	result, err := w.sendToAddress(args)
	if err != nil {
		return 0, 0, err
	}
	return result.SendAmount, result.Fee, nil
}

func (w *Wallet) GetOutputs(address string) ([]*onchain.Output, error) {
	return nil, errors.ErrUnsupported
}

func (w *Wallet) Disconnect() error {
	return nil
}

func (w *Wallet) GetTransactions(limit, offset uint64) ([]*onchain.WalletTransaction, error) {
	if limit == 0 {
		limit = onchain.DefaultTransactionsLimit
	}
	transactions, err := w.Wallet.GetTransactions(limit, offset)
	if err != nil {
		return nil, err
	}
	result := make([]*onchain.WalletTransaction, len(transactions))
	for i, tx := range transactions {
		result[i] = &onchain.WalletTransaction{
			Id:              tx.Id,
			Timestamp:       time.Unix(int64(tx.Timestamp), 0),
			BlockHeight:     tx.BlockHeight,
			BalanceChange:   tx.BalanceChange,
			IsConsolidation: tx.IsConsolidation,
		}
		for _, output := range tx.Outputs {
			result[i].Outputs = append(result[i].Outputs, onchain.TransactionOutput{
				Address:      output.Address,
				Amount:       output.Amount,
				IsOurAddress: output.IsOurAddress,
			})
		}
	}
	return result, nil
}

func (w *Wallet) GetWalletInfo() onchain.WalletInfo {
	return w.info
}

func (w *Wallet) Ready() bool {
	return true
}
