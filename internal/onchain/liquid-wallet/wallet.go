package liquid_wallet

import (
	"context"
	"errors"
	"fmt"
	"math/rand/v2"
	"sync"
	"time"

	"github.com/BoltzExchange/boltz-client/v2/internal/logger"
	"github.com/BoltzExchange/boltz-client/v2/internal/onchain"
	"github.com/BoltzExchange/boltz-client/v2/internal/onchain/liquid-wallet/lwk"
	"github.com/BoltzExchange/boltz-client/v2/pkg/boltz"
)

// Persister defines the interface for persisting and loading the last index
type Persister interface {
	// LoadLastIndex loads the last index for a given wallet ID
	LoadLastIndex(walletId uint64) (*uint32, error)
	// PersistLastIndex persists the last index for a given wallet ID
	PersistLastIndex(walletId uint64, index uint32) error
}

// InMemoryPersister implements the Persister interface using in-memory storage
type InMemoryPersister struct {
	mu      sync.RWMutex
	indices map[uint64]uint32
}

// NewInMemoryPersister creates a new in-memory persister
func NewInMemoryPersister() *InMemoryPersister {
	return &InMemoryPersister{
		indices: make(map[uint64]uint32),
	}
}

func (p *InMemoryPersister) LoadLastIndex(walletId uint64) (*uint32, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if index, exists := p.indices[walletId]; exists {
		return &index, nil
	}
	return nil, nil
}

func (p *InMemoryPersister) PersistLastIndex(walletId uint64, index uint32) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.indices[walletId] = index
	return nil
}

type Wallet struct {
	*lwk.Wollet
	signer     *lwk.Signer
	backend    *BlockchainBackend
	info       onchain.WalletInfo
	lastIndex  *uint32
	syncCancel context.CancelFunc
	syncWait   sync.WaitGroup
	persister  Persister
}

type EsploraConfig struct {
	Url       string
	Waterfall bool
}

type Config struct {
	Network                *boltz.Network
	DataDir                string
	Esplora                *EsploraConfig
	SyncInterval           time.Duration
	ConsolidationThreshold uint64
	Persister              Persister
}

type BlockchainBackend struct {
	// cfg used for buliding this instance
	cfg     Config
	esplora *lwk.EsploraClient
}

const DefaultSyncInterval = 30 * time.Second
const DefaultConsolidationThreshold = 200

func NewBlockchainBackend(cfg Config) (*BlockchainBackend, error) {
	var err error

	if cfg.SyncInterval == 0 {
		cfg.SyncInterval = DefaultSyncInterval
	}
	if cfg.ConsolidationThreshold == 0 {
		cfg.ConsolidationThreshold = DefaultConsolidationThreshold
	}
	if cfg.Persister == nil {
		cfg.Persister = NewInMemoryPersister()
	}

	backend := &BlockchainBackend{cfg: cfg}
	if cfg.Esplora != nil {
		esplora := cfg.Esplora
		if esplora.Waterfall {
			backend.esplora, err = lwk.EsploraClientNewWaterfalls(esplora.Url, convertNetwork(cfg.Network))
		} else {
			backend.esplora, err = lwk.NewEsploraClient(esplora.Url, convertNetwork(cfg.Network))
		}
		if err != nil {
			return nil, err
		}
	} else {
		return nil, errors.New("esplora is required")
	}

	return backend, nil
}

var ErrNotInitialized = errors.New("lwk not initialized")

var Regtest = lwk.NetworkRegtestDefault()
var Testnet = lwk.NetworkTestnet()
var Mainnet = lwk.NetworkMainnet()

func convertNetwork(network *boltz.Network) *lwk.Network {
	if network == nil {
		return nil
	}
	switch network {
	case boltz.Regtest:
		return Regtest
	case boltz.TestNet:
		return Testnet
	case boltz.MainNet:
		return Mainnet
	default:
		panic(fmt.Sprintf("unsupported network %v", *network))
	}
}

func NewWallet(backend *BlockchainBackend, credentials *onchain.WalletCredentials) (*Wallet, error) {
	if backend == nil {
		return nil, errors.New("backend instance is nil")
	}

	result := &Wallet{
		backend:   backend,
		info:      credentials.WalletInfo,
		persister: backend.cfg.Persister,
	}

	var descriptor *lwk.WolletDescriptor
	var err error
	if credentials.Mnemonic != "" {
		mnemonic, err := lwk.NewMnemonic(credentials.Mnemonic)
		if err != nil {
			return nil, err
		}
		result.signer, err = lwk.NewSigner(mnemonic, convertNetwork(backend.cfg.Network))
		if err != nil {
			return nil, err
		}
		descriptor, err = result.signer.WpkhSlip77Descriptor()
		if err != nil {
			return nil, err
		}
	} else {
		result.info.Readonly = true
		if credentials.CoreDescriptor == "" {
			return nil, errors.New("invalid credentials")
		}
		descriptor, err = lwk.NewWolletDescriptor(credentials.CoreDescriptor)
		if err != nil {
			return nil, err
		}
	}

	result.Wollet, err = lwk.NewWollet(
		convertNetwork(backend.cfg.Network),
		descriptor,
		&backend.cfg.DataDir,
	)
	if err != nil {
		return nil, err
	}

	if err := result.loadLastIndex(); err != nil {
		return nil, fmt.Errorf("failed to load last index: %w", err)
	}

	if err := result.FullScan(); err != nil {
		return nil, err
	}

	ctx, cancel := context.WithCancel(context.Background())
	result.syncCancel = cancel
	result.syncWait.Add(1)
	go result.syncLoop(ctx)

	return result, nil
}

func (w *Wallet) syncLoop(ctx context.Context) {
	for {
		// avoid traffic spikes if a lot of wallets are using the same backend
		sleep := time.Duration(float64(w.backend.cfg.SyncInterval) * (0.75 + rand.Float64()*0.5))
		select {
		case <-ctx.Done():
			w.syncWait.Done()
			return
		case <-time.After(sleep):
			if err := w.FullScan(); err != nil {
				logger.Errorf("LWK full scan for wallet %d failed: %v", w.info.Id, err)
			}
		}
	}
}

func (w *Wallet) FullScan() error {
	logger.Debugf("Full scanning LWK wallet %d", w.info.Id)
	update, err := w.backend.esplora.FullScan(w.Wollet)
	if err != nil {
		return err
	}
	if update != nil {
		if err := w.ApplyUpdate(*update); err != nil {
			return fmt.Errorf("could not apply update: %w", err)
		}
		if err := w.autoConsolidate(); err != nil {
			return fmt.Errorf("auto consolidate: %w", err)
		}
	}
	return nil
}

func (w *Wallet) autoConsolidate() error {
	utxos, err := w.Utxos()
	if err != nil {
		return fmt.Errorf("get utxos: %w", err)
	}
	if len(utxos) >= int(w.backend.cfg.ConsolidationThreshold) {
		logger.Debugf("Auto consolidating wallet %s with %d utxos", w.info, len(utxos))
		address, err := w.NewAddress()
		if err != nil {
			return fmt.Errorf("new address: %w", err)
		}
		txId, err := w.SendToAddress(onchain.WalletSendArgs{
			SendAll: true,
			// TODO: proper fee estimation
			SatPerVbyte: 0.1,
			Address:     address,
		})
		if err != nil {
			return fmt.Errorf("send: %w", err)
		}
		logger.Infof("Auto consolidated wallet %s: %s", w.info, txId)
		return nil
	}
	return nil
}

func (w *Wallet) Ready() bool {
	// can return true here since we wait for the fully sync to when initializing the wallet
	return true
}

func (w *Wallet) Disconnect() error {
	w.syncCancel()
	w.syncWait.Wait()
	return nil
}

func (w *Wallet) BumpTransactionFee(txId string, satPerVbyte float64) (string, error) {
	return "", errors.New("not implemented")
}

func (w *Wallet) GetWalletInfo() onchain.WalletInfo {
	return w.info
}

func (w *Wallet) GetBalance() (*onchain.Balance, error) {
	var result onchain.Balance
	utxos, err := w.Utxos()
	if err != nil {
		return nil, err
	}
	assetId := w.assetId()
	for _, utxo := range utxos {
		if utxo.Unblinded().Asset() == assetId {
			value := utxo.Unblinded().Value()
			if utxo.Height() != nil {
				result.Confirmed += value
			} else {
				result.Unconfirmed += value
			}
			result.Total += value
		}
	}
	return &result, nil
}

func (w *Wallet) NewAddress() (string, error) {
	result, err := w.Address(w.lastIndex)
	if err != nil {
		return "", err
	}
	idx := result.Index() + 1
	w.lastIndex = &idx
	if err := w.persistLastIndex(); err != nil {
		return "", fmt.Errorf("failed to persist last index: %w", err)
	}
	return result.Address().String(), nil
}

func (w *Wallet) assetId() string {
	return w.backend.cfg.Network.Liquid.AssetID
}

func (w *Wallet) GetTransactions(limit, offset uint64) ([]*onchain.WalletTransaction, error) {
	// TODO: implement pagination in lwk
	transactions, err := w.Transactions()
	if err != nil {
		return nil, err
	}

	var result []*onchain.WalletTransaction
	for _, r := range transactions {
		out := &onchain.WalletTransaction{
			Id:              r.Tx().Txid().String(),
			BalanceChange:   r.Balance()[w.assetId()],
			IsConsolidation: r.Type() == "redeposit",
			Outputs: []onchain.TransactionOutput{
				{
					Amount: r.Fee(),
				},
			},
		}
		if timeStamp := r.Timestamp(); timeStamp != nil {
			out.Timestamp = time.Unix(int64(*timeStamp), 0)
		}
		if height := r.Height(); height != nil {
			out.BlockHeight = *height
		}

		for _, maybeOutput := range r.Outputs() {
			if maybeOutput != nil {
				output := *maybeOutput
				result := onchain.TransactionOutput{
					Address: output.Address().String(),
					Amount:  output.Unblinded().Value(),
				}
				out.Outputs = append(out.Outputs, result)
			}
		}

		result = append(result, out)
	}
	return result, nil
}

func (w *Wallet) createTransaction(args onchain.WalletSendArgs) (*lwk.Transaction, error) {
	if w.signer == nil {
		return nil, errors.New("wallet is readonly")
	}

	builder := lwk.NewTxBuilder(convertNetwork(w.backend.cfg.Network))
	addr, err := lwk.NewAddress(args.Address)
	if err != nil {
		return nil, err
	}
	if args.SendAll {
		if err := builder.DrainLbtcTo(addr); err != nil {
			return nil, fmt.Errorf("drain lbtc: %w", err)
		}
	} else {
		if err := builder.AddLbtcRecipient(addr, args.Amount); err != nil {
			return nil, fmt.Errorf("add lbtc recipient: %w", err)
		}
	}
	rate := float32(args.SatPerVbyte * 1000)
	if err := builder.FeeRate(&rate); err != nil {
		return nil, fmt.Errorf("set fee rate: %w", err)
	}

	pset, err := builder.Finish(w.Wollet)
	if err != nil {
		return nil, fmt.Errorf("finish: %w", err)
	}

	pset, err = w.signer.Sign(pset)
	if err != nil {
		return nil, fmt.Errorf("sign: %w", err)
	}

	pset, err = w.Finalize(pset)
	if err != nil {
		return nil, fmt.Errorf("finalize: %w", err)
	}

	tx, err := pset.ExtractTx()
	if err != nil {
		return nil, fmt.Errorf("extract: %w", err)
	}

	return tx, nil
}

func (w *Wallet) SendToAddress(args onchain.WalletSendArgs) (string, error) {
	tx, err := w.createTransaction(args)
	if err != nil {
		return "", err
	}

	// TODO: external broadcast provider
	txId, err := w.backend.esplora.Broadcast(tx)
	if err != nil {
		return "", fmt.Errorf("broadcast: %w", err)
	}

	return txId.String(), nil
}

func (w *Wallet) GetSendFee(args onchain.WalletSendArgs) (send uint64, fee uint64, err error) {
	tx, err := w.createTransaction(args)
	if err != nil {
		return 0, 0, err
	}

	txos, err := w.Txos()
	if err != nil {
		return 0, 0, err
	}

	for _, input := range tx.Inputs() {
		for _, txo := range txos {
			if txo.Outpoint().String() == input.Outpoint().String() {
				send += txo.Unblinded().Value()
				break
			}
		}
	}

	for _, output := range tx.Outputs() {
		if output.IsFee() {
			fee = *output.Value()
			break
		}
	}
	return send - fee, fee, nil
}

func GenerateMnemonic(network *boltz.Network) (string, error) {
	signer, err := lwk.SignerRandom(convertNetwork(network))
	if err != nil {
		return "", err
	}
	mnemonic, err := signer.Mnemonic()
	if err != nil {
		return "", err
	}
	return mnemonic.String(), nil
}

func (w *Wallet) persistLastIndex() error {
	if w.lastIndex == nil {
		return nil
	}
	return w.persister.PersistLastIndex(w.info.Id, *w.lastIndex)
}

func (w *Wallet) loadLastIndex() error {
	index, err := w.persister.LoadLastIndex(w.info.Id)
	if err != nil {
		return err
	}
	w.lastIndex = index
	return nil
}
