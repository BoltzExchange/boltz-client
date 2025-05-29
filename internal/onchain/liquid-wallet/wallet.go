package liquid_wallet

import (
	"errors"
	"fmt"
	"time"

	"github.com/BoltzExchange/boltz-client/v2/internal/onchain"
	"github.com/BoltzExchange/boltz-client/v2/internal/onchain/liquid-wallet/lwk"
	"github.com/BoltzExchange/boltz-client/v2/internal/onchain/wallet"
	"github.com/BoltzExchange/boltz-client/v2/pkg/boltz"
)

type Wallet struct {
	*lwk.Wollet
	signer    *lwk.Signer
	shared    *shared
	info      onchain.WalletInfo
	lastIndex *uint32
}

type EsploraConfig struct {
	Url       string
	Waterfall bool
}

type Config struct {
	Network *boltz.Network
	DataDir string
	Esplora *EsploraConfig
}

type shared struct {
	cfg     *Config
	esplora *lwk.EsploraClient
}

var clients *shared

func Init(cfg *Config) error {
	var err error

	clients = &shared{
		cfg: cfg,
	}
	if clients.cfg.Esplora != nil {
		esplora := clients.cfg.Esplora
		if esplora.Waterfall {
			clients.esplora, err = lwk.EsploraClientNewWaterfalls(esplora.Url, convertNetwork(cfg.Network))
		} else {
			clients.esplora, err = lwk.NewEsploraClient(esplora.Url, convertNetwork(cfg.Network))
		}
		if err != nil {
			return err
		}
	} else {
		return errors.New("esplora is required")
	}

	return nil
}

var ErrNotInitialized = errors.New("lwk not initialized")

var Regtest = lwk.NetworkRegtestDefault()
var Testnet = lwk.NetworkTestnet()
var Mainnet = lwk.NetworkMainnet()

func convertNetwork(network *boltz.Network) *lwk.Network {
	switch network {
	case boltz.Regtest:
		return Regtest
	case boltz.TestNet:
		return Testnet
	case boltz.MainNet:
		return Mainnet
	default:
		return nil
	}
}

func NewWallet(credentials *wallet.Credentials) (*Wallet, error) {
	if clients == nil {
		return nil, ErrNotInitialized
	}

	result := &Wallet{
		shared: clients,
		info:   credentials.WalletInfo,
	}

	if credentials.Mnemonic != "" {
		mnemonic, err := lwk.NewMnemonic(credentials.Mnemonic)
		if err != nil {
			return nil, err
		}
		result.signer, err = lwk.NewSigner(mnemonic, convertNetwork(clients.cfg.Network))
		if err != nil {
			return nil, err
		}
		descriptor, err := result.signer.WpkhSlip77Descriptor()
		if err != nil {
			return nil, err
		}
		credentials.CoreDescriptor = descriptor.String()
	} else {
		result.info.Readonly = true
	}

	descriptor, err := lwk.NewWolletDescriptor(credentials.CoreDescriptor)
	if err != nil {
		return nil, err
	}
	result.Wollet, err = lwk.NewWollet(
		convertNetwork(clients.cfg.Network),
		descriptor,
		&clients.cfg.DataDir,
	)
	if err != nil {
		return nil, err
	}

	if err := result.FullScan(); err != nil {
		return nil, err
	}

	return result, nil
}

func (w *Wallet) FullScan() error {
	update, err := w.shared.esplora.FullScan(w.Wollet)
	if err != nil {
		return fmt.Errorf("full scan: %w", err)
	}
	if update != nil {
		return w.ApplyUpdate(*update)
	}
	return nil
}

func (w *Wallet) Ready() bool {
	// TODO
	return true
}

func (w *Wallet) Disconnect() error {
	return nil
}

func (w *Wallet) BumpTransactionFee(txId string, satPerVbyte float64) (string, error) {
	return "", nil
}

func (w *Wallet) GetWalletInfo() onchain.WalletInfo {
	return w.info
}

func (w *Wallet) GetBalance() (*onchain.Balance, error) {
	balance, err := w.Balance()
	if err != nil {
		return nil, err
	}
	b, ok := balance[w.assetId()]
	if !ok {
		return nil, fmt.Errorf("asset %s not found", w.assetId())
	}
	// TODO: unconfirmed?
	return &onchain.Balance{
		Total:     b,
		Confirmed: b,
	}, nil
}

func (w *Wallet) NewAddress() (string, error) {
	result, err := w.Address(w.lastIndex)
	if err != nil {
		return "", err
	}
	idx := result.Index() + 1
	w.lastIndex = &idx
	return result.Address().String(), nil
}

func (w *Wallet) assetId() string {
	return w.shared.cfg.Network.Liquid.AssetID
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

	builder := lwk.NewTxBuilder(convertNetwork(clients.cfg.Network))
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
	txId, err := w.shared.esplora.Broadcast(tx)
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
