package liquid_wallet

import (
	"encoding/hex"
	"errors"
	"fmt"
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
	signer     *lwk.Signer
	descriptor *lwk.WolletDescriptor
	backend    *BlockchainBackend
	info       onchain.WalletInfo
	syncLock   sync.Mutex
	sendLock   sync.Mutex
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
	ConsolidationThreshold uint64
	ChainProvider          onchain.ChainProvider
	Persister              Persister
}

type clientConfig struct {
	electrum *onchain.ElectrumOptions
	esplora  *EsploraConfig
}

type BlockchainBackend struct {
	// cfg used for buliding this instance
	cfg Config
	// electrum also satisfies the EsploraClientInterface
	clientConfigs []clientConfig
	clients       sync.Map
	connectLock   sync.Mutex
}

func (b *BlockchainBackend) BroadcastTransaction(tx *lwk.Transaction) (string, error) {
	raw := tx.Bytes()
	return b.cfg.ChainProvider.BroadcastTransaction(hex.EncodeToString(raw))
}

func (b *BlockchainBackend) DeriveDefaultDescriptor(mnemonic string) (string, error) {
	return DeriveDefaultDescriptor(b.cfg.Network, mnemonic)
}

const ConnectTimeout = 10 * time.Second

func (b *BlockchainBackend) connectClient(config clientConfig) (lwk.EsploraClientInterface, error) {
	if config.electrum != nil {
		logger.Debugf("Connecting to Liquid electrum server: %s", config.electrum)
		validateDomain := config.electrum.SSL
		client, err := lwk.NewElectrumClient(config.electrum.Url, config.electrum.SSL, validateDomain)
		if err != nil {
			return nil, fmt.Errorf("new electrum client: %w", err)
		}
		return client, nil
	} else {
		logger.Debugf("Connecting to Liquid esplora server: %s", config.esplora.Url)
		concurrency := uint32(32)
		client, err := lwk.EsploraClientFromBuilder(lwk.EsploraClientBuilder{
			BaseUrl:     config.esplora.Url,
			Network:     convertNetwork(b.cfg.Network),
			Waterfalls:  config.esplora.Waterfall,
			Concurrency: &concurrency,
		})
		if err != nil {
			return nil, fmt.Errorf("esplora client: %w", err)
		}
		return client, nil
	}
}

func (b *BlockchainBackend) getClient(config clientConfig) (lwk.EsploraClientInterface, error) {
	var key string
	if config.electrum != nil {
		key = "electrum:" + config.electrum.Url
	} else {
		key = "esplora:" + config.esplora.Url
	}

	b.connectLock.Lock()
	defer b.connectLock.Unlock()

	if client, ok := b.clients.Load(key); ok {
		return client.(lwk.EsploraClientInterface), nil
	}

	res := make(chan error)
	var result lwk.EsploraClientInterface
	go func() {
		client, err := b.connectClient(config)
		if err == nil {
			b.clients.Store(key, client)
			result = client
		}
		res <- err
	}()

	select {
	case err := <-res:
		if err != nil {
			return nil, err
		}
		return result, nil
	case <-time.After(ConnectTimeout):
		return nil, errors.New("connection timeout")
	}
}

const MainnetElectrumBackup = "elements-mainnet.blockstream.info:50002"
const DefaultConsolidationThreshold = 200

func NewBackend(cfg Config) (*BlockchainBackend, error) {
	if cfg.Persister == nil {
		return nil, errors.New("persister is required")
	}
	if cfg.ChainProvider == nil {
		return nil, errors.New("chain provider is required")
	}
	if cfg.ConsolidationThreshold == 0 {
		cfg.ConsolidationThreshold = DefaultConsolidationThreshold
	}

	backend := &BlockchainBackend{
		cfg:     cfg,
		clients: sync.Map{},
	}

	if cfg.Electrum != nil {
		logger.Infof("Liquid: wallet backend: %s", cfg.Electrum)
		backend.clientConfigs = append(backend.clientConfigs, clientConfig{
			electrum: cfg.Electrum,
		})
	} else {
		esplora := cfg.Esplora
		if esplora == nil {
			switch cfg.Network {
			case boltz.Regtest:
				esplora = &EsploraConfig{
					Url:       "http://localhost:3003",
					Waterfall: false,
				}
			case boltz.MainNet:
				esplora = &EsploraConfig{
					Url:       "https://esplora.bol.tz/liquid",
					Waterfall: true,
				}
			default:
				return nil, errors.New("esplora is required")
			}
		}
		logger.Infof("Liquid: wallet backend: %s", esplora.Url)
		backend.clientConfigs = append(backend.clientConfigs, clientConfig{
			esplora: esplora,
		})

		// only add the default electrum client if no custom esplora is configured
		if cfg.Esplora == nil {
			if cfg.Network == boltz.MainNet {
				url := MainnetElectrumBackup
				logger.Infof("Liquid: wallet backend fallback: ssl://%s", url)
				backend.clientConfigs = append(backend.clientConfigs, clientConfig{
					electrum: &onchain.ElectrumOptions{
						Url: url,
						SSL: true,
					},
				})
			}
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

func DeriveDefaultDescriptor(network *boltz.Network, mnemonic string) (string, error) {
	signer, err := newSigner(network, mnemonic)
	if err != nil {
		return "", err
	}
	descriptor, err := signer.SinglesigDesc(lwk.SinglesigWpkh, lwk.DescriptorBlindingKeySlip77)
	if err != nil {
		return "", err
	}
	return descriptor.String(), nil
}

func (backend *BlockchainBackend) NewWallet(credentials *onchain.WalletCredentials) (onchain.Wallet, error) {
	if backend == nil {
		return nil, errors.New("backend instance is nil")
	}

	result := &Wallet{
		backend: backend,
		info:    credentials.WalletInfo,
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

	return result, nil
}

func (w *Wallet) Sync() error {
	if err := w.fullScan(false); err != nil {
		return err
	}
	if err := w.autoConsolidate(); err != nil {
		return fmt.Errorf("auto consolidation: %w", err)
	}
	return nil
}

func (w *Wallet) FullScan() error {
	if err := w.fullScan(true); err != nil {
		return err
	}
	if err := w.autoConsolidate(); err != nil {
		return fmt.Errorf("auto consolidation: %w", err)
	}
	return nil
}

func (w *Wallet) fullScan(all bool) error {
	logger.Debugf("Full scanning LWK wallet %d", w.info.Id)
	w.syncLock.Lock()
	defer w.syncLock.Unlock()

	index, err := w.loadLastIndex()
	if err != nil {
		return fmt.Errorf("load last index: %w", err)
	}
	if index == nil || all {
		idx := uint32(0)
		index = &idx
	}
	// Try each client until one succeeds
	var update **lwk.Update
	var lastErr error
	for i, config := range w.backend.clientConfigs {
		client, err := w.backend.getClient(config)
		if err != nil {
			logger.Debugf("Client %d failed to get client for wallet %d: %v", i, w.info.Id, err)
			lastErr = err
			continue
		}
		update, err = client.FullScanToIndex(w.Wollet, *index)
		if err != nil {
			logger.Debugf("Client %d failed to sync wallet %d: %v", i, w.info.Id, err)
			lastErr = err
			continue
		}
		break
	}
	if update == nil && lastErr != nil {
		return fmt.Errorf("all clients failed to sync, last error: %w", lastErr)
	}
	if update != nil {
		if err := w.ApplyUpdate(*update); err != nil {
			return fmt.Errorf("could not apply update: %w", err)
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
		feeRate, err := w.backend.cfg.ChainProvider.EstimateFee()
		if err != nil {
			return fmt.Errorf("estimate fee: %w", err)
		}
		feeRate = max(onchain.FeeFloor[boltz.CurrencyLiquid], feeRate)
		logger.Debugf("Using fee rate of %f sat/vbyte for consolidation", feeRate)
		txId, err := w.SendToAddress(onchain.WalletSendArgs{
			SendAll:     true,
			SatPerVbyte: feeRate,
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
		}
		if timeStamp := r.Timestamp(); timeStamp != nil {
			out.Timestamp = time.Unix(int64(*timeStamp), 0)
		}
		if height := r.Height(); height != nil {
			out.BlockHeight = *height
		}

		outputs := r.Outputs()
		for i, output := range r.Tx().Outputs() {
			maybeOutput := outputs[i]
			result := onchain.TransactionOutput{}
			if output.IsFee() {
				result.Amount = r.Fee()
			} else {
				if maybeOutput == nil {
					if address := output.UnconfidentialAddress(convertNetwork(w.backend.cfg.Network)); address != nil {
						result.Address = (*address).String()
					}
					if amount := output.Value(); amount != nil {
						result.Amount = *amount
					}
				} else {
					result.Address = (*maybeOutput).Address().String()
					result.Amount = (*maybeOutput).Unblinded().Value()
					result.IsOurAddress = true
				}
			}
			if asset := output.Asset(); asset != nil && *asset != w.assetId() {
				result.Amount = 0
			}
			out.Outputs = append(out.Outputs, result)
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
	w.sendLock.Lock()
	defer w.sendLock.Unlock()

	tx, err := w.createTransaction(args)
	if err != nil {
		return "", err
	}

	txId, err := w.backend.BroadcastTransaction(tx)
	if err != nil {
		return "", err
	}

	if err := w.applyTransaction(tx); err != nil {
		return "", err
	}

	return txId, nil
}

func (w *Wallet) applyTransaction(tx *lwk.Transaction) error {
	w.syncLock.Lock()
	defer w.syncLock.Unlock()

	if err := w.Wollet.ApplyTransaction(tx); err != nil {
		return fmt.Errorf("failed to apply transaction: %w", err)
	}
	return nil
}

func (w *Wallet) ApplyTransaction(txHex string) error {
	tx, err := lwk.NewTransaction(txHex)
	if err != nil {
		return fmt.Errorf("failed to parse transaction: %w", err)
	}
	return w.applyTransaction(tx)
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

	// we calculate the amount sent by first summing up all our own inputs and subtracting all our own outputs
	for _, input := range tx.Inputs() {
		for _, txo := range txos {
			if txo.Outpoint().String() == input.Outpoint().String() {
				send += txo.Unblinded().Value()
				break
			}
		}
	}

	for _, output := range tx.Outputs() {
		if key := w.descriptor.DeriveBlindingKey(output.ScriptPubkey()); key != nil {
			secrets, err := output.Unblind(*key)
			if err == nil {
				send -= secrets.Value()
			}
		}
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
				if unblinded.Asset() == w.assetId() {
					output.Value = unblinded.Value()
				}
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
