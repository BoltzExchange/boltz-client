package bitcoin_wallet

import (
	"errors"
	"fmt"
	"time"

	"github.com/BoltzExchange/boltz-client/v2/internal/onchain"
	"github.com/BoltzExchange/boltz-client/v2/internal/onchain/bitcoin-wallet/bdk"
	"github.com/BoltzExchange/boltz-client/v2/pkg/boltz"
)

type Backend struct {
	*bdk.Backend
	cfg         Config
	TxProvider  onchain.TxProvider
	FeeProvider onchain.FeeProvider
}

type Wallet struct {
	*bdk.Wallet
	info       onchain.WalletInfo
	txProvider onchain.TxProvider
}

type Config struct {
	Network      *boltz.Network
	DataDir      string
	Electrum     *onchain.ElectrumOptions
	SyncInterval time.Duration
	TxProvider   onchain.TxProvider
	FeeProvider  onchain.FeeProvider
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
		cfg.DataDir+"/bdk.sqlite",
	)
	if err != nil {
		return nil, err
	}
	return &Backend{backend, cfg, cfg.TxProvider, cfg.FeeProvider}, nil
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
	var creds bdk.WalletCredentials
	if credentials.CoreDescriptor != "" {
		creds.CoreDescriptor = credentials.CoreDescriptor
	}
	if credentials.Mnemonic != "" {
		creds.Mnemonic = &credentials.Mnemonic
		info.Readonly = false
	}
	wallet, err := bdk.NewWallet(backend.Backend, creds)
	if err != nil {
		return nil, err
	}

	return &Wallet{Wallet: wallet, txProvider: backend.TxProvider, info: info}, nil
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

func (w *Wallet) BumpTransactionFee(txId string, satPerVbyte float64) (string, error) {
	txHex, err := w.Wallet.BumpTransactionFee(txId, satPerVbyte)
	if err != nil {
		return "", err
	}
	newTxId, err := w.txProvider.BroadcastTransaction(txHex)
	if err != nil {
		return "", err
	}
	return newTxId, nil
}

func (w *Wallet) SendToAddress(args onchain.WalletSendArgs) (string, error) {
	result, err := w.Wallet.SendToAddress(
		args.Address,
		args.Amount,
		args.SatPerVbyte,
		args.SendAll,
	)
	if err != nil {
		return "", err
	}
	txId, err := w.txProvider.BroadcastTransaction(result.TxHex)
	if err != nil {
		return "", err
	}
	if err := w.Wallet.ApplyTransaction(result.TxHex); err != nil {
		return "", err
	}
	return txId, nil
}

func (w *Wallet) Sync() error {
	return w.Wallet.Sync()
}

func (w *Wallet) GetSendFee(args onchain.WalletSendArgs) (send uint64, fee uint64, err error) {
	result, err := w.Wallet.SendToAddress(
		args.Address,
		args.Amount,
		args.SatPerVbyte,
		args.SendAll,
	)
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
