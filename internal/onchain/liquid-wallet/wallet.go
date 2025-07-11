package liquid_wallet

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"math/rand/v2"
	"strings"
	"sync"
	"time"

	"github.com/BoltzExchange/boltz-client/v2/internal/logger"
	"github.com/BoltzExchange/boltz-client/v2/internal/onchain"
	"github.com/BoltzExchange/boltz-client/v2/internal/onchain/liquid-wallet/lwk"
	"github.com/BoltzExchange/boltz-client/v2/pkg/boltz"
	"github.com/btcsuite/btcd/btcec/v2"
)

// Persister defines methods for saving and retrieving the last used address index for a wallet.
// The index is marked as used as soon as its corresponding address is generated, ensuring address uniqueness.
type Persister interface {
	LoadLastIndex(walletId uint64) (*uint32, error)
	PersistLastIndex(walletId uint64, index uint32) error
}

type Wallet struct {
	*lwk.Wollet
	signer           *lwk.Signer
	descriptor       *lwk.WolletDescriptor
	backend          *BlockchainBackend
	info             onchain.WalletInfo
	syncCancel       context.CancelFunc
	syncWait         sync.WaitGroup
	spentOutputs     map[string]bool
	spentOutputsLock sync.RWMutex
}

type EsploraConfig struct {
	Url       string
	Waterfall bool
}

type Config struct {
	Network                *boltz.Network
	DataDir                string
	Esplora                *EsploraConfig
	Electrum               *onchain.ElectrumOptions
	SyncInterval           time.Duration
	ConsolidationThreshold uint64
	TxProvider             onchain.TxProvider
	Persister              Persister
}

type BlockchainBackend struct {
	// cfg used for buliding this instance
	cfg Config
	// electrum also satisfies the EsploraClientInterface
	client lwk.EsploraClientInterface
}

func (b *BlockchainBackend) BroadcastTransaction(tx *lwk.Transaction) (string, error) {
	if b.cfg.TxProvider != nil {
		raw := tx.Bytes()
		return b.cfg.TxProvider.BroadcastTransaction(hex.EncodeToString(raw))
	}
	txId, err := b.client.Broadcast(tx)
	if err != nil {
		return "", err
	}
	return txId.String(), nil
}

const DefaultSyncInterval = 30 * time.Second
const DefaultConsolidationThreshold = 200

func NewBlockchainBackend(cfg Config) (*BlockchainBackend, error) {
	var err error
	if cfg.Persister == nil {
		return nil, errors.New("persister is required")
	}
	if cfg.SyncInterval == 0 {
		if cfg.Network == boltz.Regtest {
			cfg.SyncInterval = 1 * time.Second
		} else {
			cfg.SyncInterval = DefaultSyncInterval
		}
	}
	if cfg.ConsolidationThreshold == 0 {
		cfg.ConsolidationThreshold = DefaultConsolidationThreshold
	}

	backend := &BlockchainBackend{cfg: cfg}
	if cfg.Electrum != nil {
		logger.Infof("Using electrum client as liquid wallet backend: %s", cfg.Electrum.Url)
		backend.client, err = lwk.NewElectrumClient(cfg.Electrum.Url, cfg.Electrum.SSL, false)
		if err != nil {
			return nil, fmt.Errorf("new electrum client: %w", err)
		}
	} else {
		if cfg.Esplora == nil {
			switch cfg.Network {
			case boltz.Regtest:
				cfg.Esplora = &EsploraConfig{
					Url:       "http://localhost:3003",
					Waterfall: false,
				}
			case boltz.MainNet:
				cfg.Esplora = &EsploraConfig{
					Url:       "https://esplora.bol.tz/liquid",
					Waterfall: true,
				}
			default:
				return nil, errors.New("esplora is required")
			}
		}
		logger.Infof("Using esplora client as liquid wallet backend: %s", cfg.Esplora.Url)
		concurrency := uint32(32)
		backend.client, err = lwk.EsploraClientFromBuilder(lwk.EsploraClientBuilder{
			BaseUrl:     cfg.Esplora.Url,
			Network:     convertNetwork(cfg.Network),
			Waterfalls:  cfg.Esplora.Waterfall,
			Concurrency: &concurrency,
		})
		if err != nil {
			return nil, fmt.Errorf("esplora client: %w", err)
		}
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

func newSigner(network *boltz.Network, mnemonic string) (*lwk.Signer, error) {
	parsed, err := lwk.NewMnemonic(mnemonic)
	if err != nil {
		return nil, err
	}
	return lwk.NewSigner(parsed, convertNetwork(network))
}

func DeriveDefaultDescriptor(network *boltz.Network, credentials *onchain.WalletCredentials) error {
	if credentials.CoreDescriptor == "" {
		if credentials.Mnemonic == "" {
			return errors.New("core descriptor or mnemonic is required")
		}
		signer, err := newSigner(network, credentials.Mnemonic)
		if err != nil {
			return err
		}
		descriptor, err := signer.SinglesigDesc(lwk.SinglesigWpkh, lwk.DescriptorBlindingKeySlip77)
		if err != nil {
			return err
		}
		credentials.CoreDescriptor = descriptor.String()
	}
	return nil
}

func NewWallet(backend *BlockchainBackend, credentials *onchain.WalletCredentials) (*Wallet, error) {
	if backend == nil {
		return nil, errors.New("backend instance is nil")
	}

	result := &Wallet{
		backend:      backend,
		info:         credentials.WalletInfo,
		spentOutputs: make(map[string]bool),
	}

	if credentials.CoreDescriptor == "" {
		return nil, errors.New("core descriptor is required")
	}
	var err error
	result.descriptor, err = lwk.NewWolletDescriptor(credentials.CoreDescriptor)
	if err != nil {
		return nil, err
	}
	if credentials.Mnemonic != "" {
		result.signer, err = newSigner(backend.cfg.Network, credentials.Mnemonic)
		if err != nil {
			return nil, err
		}
	} else {
		result.info.Readonly = true
	}

	result.Wollet, err = lwk.NewWollet(
		convertNetwork(backend.cfg.Network),
		result.descriptor,
		&backend.cfg.DataDir,
	)
	if err != nil {
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
			if err := w.Sync(); err != nil {
				logger.Errorf("LWK full scan for wallet %d failed: %v", w.info.Id, err)
			}
		}
	}
}

func (w *Wallet) Sync() error {
	logger.Debugf("Full scanning LWK wallet %d", w.info.Id)
	index, err := w.loadLastIndex()
	if err != nil {
		return fmt.Errorf("load last index: %w", err)
	}
	if index == nil {
		all := uint32(0)
		index = &all
	}
	update, err := w.backend.client.FullScanToIndex(w.Wollet, *index)
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
		w.spentOutputsLock.Lock()
		w.spentOutputs = make(map[string]bool)
		w.spentOutputsLock.Unlock()
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
	index, err := w.loadLastIndex()
	if err != nil {
		return "", fmt.Errorf("load last index: %w", err)
	}
	result, err := w.Address(index)
	if err != nil {
		return "", err
	}
	idx := result.Index() + 1
	if err := w.persistLastIndex(idx); err != nil {
		return "", fmt.Errorf("failed to persist last index: %w", err)
	}
	return result.Address().String(), nil
}

func (w *Wallet) assetId() string {
	return w.backend.cfg.Network.Liquid.AssetID
}

func (w *Wallet) GetTransactions(limit, offset uint64) ([]*onchain.WalletTransaction, error) {
	if limit == 0 {
		limit = onchain.DefaultTransactionsLimit
	}
	transactions, err := w.TransactionsPaginated(uint32(offset), uint32(limit))
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

func (w *Wallet) DeriveBlindingKey(address string) (*btcec.PrivateKey, error) {
	addr, err := lwk.NewAddress(address)
	if err != nil {
		return nil, err
	}
	key := w.descriptor.DeriveBlindingKey(addr.ScriptPubkey())
	if key == nil {
		return nil, errors.New("could not derive blinding key")
	}
	privKey, _ := btcec.PrivKeyFromBytes((*key).Bytes())
	return privKey, nil
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

	utxos, err := w.Utxos()
	if err != nil {
		return nil, err
	}

	var outpoints []*lwk.OutPoint
	for _, output := range utxos {
		outpoint := output.Outpoint()
		if !w.spentOutputs[outpoint.String()] {
			outpoints = append(outpoints, outpoint)
		} else {
			logger.Debugf("Ignoring outpoint %s since it is marked as spent", outpoint.String())
		}
	}

	if err := builder.SetWalletUtxos(outpoints); err != nil {
		return nil, fmt.Errorf("set wallet utxos: %w", err)
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
		if strings.Contains(err.Error(), "InsufficientFunds") {
			return nil, w.info.InsufficientBalanceError(args.Amount)
		}
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
	w.spentOutputsLock.Lock()
	defer w.spentOutputsLock.Unlock()
	tx, err := w.createTransaction(args)
	if err != nil {
		return "", err
	}

	txId, err := w.backend.BroadcastTransaction(tx)
	if err != nil {
		return "", err
	}

	for _, input := range tx.Inputs() {
		w.spentOutputs[input.Outpoint().String()] = true
	}

	return txId, nil
}

func (w *Wallet) GetSendFee(args onchain.WalletSendArgs) (send uint64, fee uint64, err error) {
	w.spentOutputsLock.RLock()
	defer w.spentOutputsLock.RUnlock()
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

func (w *Wallet) GetOutputs(address string) ([]*onchain.Output, error) {
	utxos, err := w.Utxos()
	if err != nil {
		return nil, err
	}

	var outputs []*onchain.Output
	for _, utxo := range utxos {
		if utxo.Address().String() == address {
			output := &onchain.Output{TxId: utxo.Outpoint().Txid().String()}
			if unblinded := utxo.Unblinded(); unblinded != nil {
				output.Value = unblinded.Value()
			}
			outputs = append(outputs, output)
		}
	}
	return outputs, nil
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

func (w *Wallet) persistLastIndex(index uint32) error {
	return w.backend.cfg.Persister.PersistLastIndex(w.info.Id, index)
}

func (w *Wallet) loadLastIndex() (*uint32, error) {
	return w.backend.cfg.Persister.LoadLastIndex(w.info.Id)
}
