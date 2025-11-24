package bitcoin_wallet

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/BoltzExchange/boltz-client/v2/internal/logger"
	"github.com/BoltzExchange/boltz-client/v2/internal/onchain"
	"github.com/BoltzExchange/boltz-client/v2/internal/onchain/bitcoin-wallet/bdk"
	"github.com/BoltzExchange/boltz-client/v2/pkg/boltz"
)

type Backend struct {
	cfg             Config
	electrumServers []*onchain.ElectrumOptions
	chainClients    sync.Map
}

type Wallet struct {
	*bdk.Wallet
	backend  *Backend
	info     onchain.WalletInfo
	sendLock sync.Mutex
}

type Config struct {
	Network       *boltz.Network
	DataDir       string
	Electrum      *onchain.ElectrumOptions
	SyncInterval  time.Duration
	ChainProvider onchain.ChainProvider
}

func newChainClient(electrum *onchain.ElectrumOptions) (*bdk.ChainClient, error) {
	url := electrum.Url
	if electrum.SSL {
		url = "ssl://" + url
	} else {
		url = "tcp://" + url
	}
	logger.Debugf("Connecting to electrum server: %s", url)
	return bdk.NewChainClient(url)
}

func NewBackend(cfg Config) (*Backend, error) {
	electrum := cfg.Electrum
	if electrum == nil {
		switch cfg.Network {
		case boltz.MainNet:
			electrum = &onchain.ElectrumOptions{
				Url: "bitcoin-mainnet.blockstream.info:50002",
				SSL: true,
			}
		case boltz.Regtest:
			electrum = onchain.RegtestElectrumConfig.Btc
		default:
			return nil, errors.New("unknown network")
		}
		logger.Infof("BTC: wallet backend: %s", electrum)
	}
	electrumServers := []*onchain.ElectrumOptions{electrum}
	if cfg.Electrum == nil && cfg.Network == boltz.MainNet {
		boltzElectrum := &onchain.ElectrumOptions{
			Url: "esplora.bol.tz:50002",
			SSL: true,
		}
		logger.Infof("BTC: wallet backend fallback: %s", boltzElectrum)
		electrumServers = append(electrumServers, boltzElectrum)
	}
	return &Backend{
		chainClients:    sync.Map{},
		electrumServers: electrumServers,
		cfg:             cfg,
	}, nil
}

// we do lazy initialization of the electrum clients
// to avoid blocking startup if we experience connectivity issues
func (backend *Backend) getChainClient(electrum *onchain.ElectrumOptions) (*bdk.ChainClient, error) {
	client, ok := backend.chainClients.Load(electrum.Url)
	if ok {
		return client.(*bdk.ChainClient), nil
	}
	newClient, err := newChainClient(electrum)
	if err != nil {
		return nil, err
	}
	backend.chainClients.Store(electrum.Url, newClient)
	return newClient, nil
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
		creds,
		path.Join(backend.cfg.DataDir, dbName),
		convertNetwork(backend.cfg.Network),
	)
	if err != nil {
		return nil, err
	}

	return &Wallet{Wallet: wallet, backend: backend, info: info}, nil
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
	txId, err := w.backend.cfg.ChainProvider.BroadcastTransaction(txHex)
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

func (w *Wallet) FullScan() error {
	var err error
	for _, electrum := range w.backend.electrumServers {
		logger.Debugf("Full scanning wallet %d with electrum server: %s", w.info.Id, electrum)
		chainClient, err := w.backend.getChainClient(electrum)
		if err != nil {
			logger.Errorf("Client %s failed to get chain client: %v", electrum, err)
			continue
		}
		err = w.Wallet.FullScan(chainClient)
		if err != nil {
			logger.Errorf("Client %s failed to full scan wallet %d: %v", electrum, w.info.Id, err)
			continue
		}
		return nil
	}
	return fmt.Errorf("all clients failed to full scan, last error: %w", err)
}

func (w *Wallet) Sync() error {
	var err error
	for _, electrum := range w.backend.electrumServers {
		logger.Debugf("Syncing wallet %d with electrum server: %s", w.info.Id, electrum)
		chainClient, err := w.backend.getChainClient(electrum)
		if err != nil {
			logger.Errorf("Client %s failed to get chain client: %v", electrum, err)
			continue
		}
		start := time.Now()
		err = w.Wallet.Sync(chainClient)
		if err != nil {
			logger.Errorf("Client %s failed to sync wallet %d: %v", electrum, w.info.Id, err)
			continue
		}
		duration := time.Since(start)
		logger.Debugf("Sync for wallet %d with electrum server %s took %s", w.info.Id, electrum, duration)
		return nil
	}
	return fmt.Errorf("all clients failed to sync, last error: %w", err)
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
