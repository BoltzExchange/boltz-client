package autoswap

import (
	"github.com/BoltzExchange/boltz-client/database"
	"github.com/BoltzExchange/boltz-client/onchain"
	"testing"

	"github.com/BoltzExchange/boltz-client/boltz"
	"github.com/BoltzExchange/boltz-client/boltzrpc"
	"github.com/stretchr/testify/require"
)

func TestGetPair(t *testing.T) {
	cfg := NewConfig(DefaultLightningConfig())

	pair := cfg.GetPair(boltz.NormalSwap)
	require.Equal(t, boltzrpc.Currency_LBTC, pair.From)
	require.Equal(t, boltzrpc.Currency_BTC, pair.To)

	pair = cfg.GetPair(boltz.ReverseSwap)
	require.Equal(t, boltzrpc.Currency_LBTC, pair.To)
	require.Equal(t, boltzrpc.Currency_BTC, pair.From)

	cfg.Currency = boltzrpc.Currency_BTC

	pair = cfg.GetPair(boltz.ReverseSwap)
	require.Equal(t, boltzrpc.Currency_BTC, pair.To)
	require.Equal(t, boltzrpc.Currency_BTC, pair.From)
}

func TestLightningConfig(t *testing.T) {
	enabled := func(cfg *SerializedLnConfig) *SerializedLnConfig {
		cfg.Enabled = true
		cfg.MaxBalancePercent = 75
		cfg.MinBalancePercent = 25
		return cfg
	}
	tt := []struct {
		name    string
		cfg     *SerializedLnConfig
		err     bool
		wallets []onchain.WalletInfo
	}{
		{name: "Default", cfg: DefaultLightningConfig(), err: false},
		{
			name: "MissingMax",
			cfg: &SerializedLnConfig{
				MinBalancePercent: 25,
			},
			err: true,
		},
		{
			name: "ValidReverse",
			cfg: &SerializedLnConfig{
				MaxBalancePercent: 75,
				SwapType:          "reverse",
			},
			err: false,
		},
		{
			name: "MinGreaterMax/Percent",
			cfg: &SerializedLnConfig{
				MinBalancePercent: 75,
				MaxBalancePercent: 25,
			},
			err: true,
		},

		{
			name: "MinGreaterMax/Abs",
			cfg: &SerializedLnConfig{
				MinBalance: 10000,
				MaxBalance: 5000,
			},
			err: true,
		},
		{
			name: "PerChannel/SubmarineForbidden",
			cfg: &SerializedLnConfig{
				MinBalance: 10000,
				PerChannel: true,
				SwapType:   "submarine",
			},
			err: true,
		},
		{
			name: "StaticAddress/Invalid",
			cfg: enabled(&SerializedLnConfig{
				Currency:      boltzrpc.Currency_BTC,
				StaticAddress: "invalid",
				SwapType:      "reverse",
			}),
			err: true,
		},
		{
			name: "StaticAddress/Valid",
			cfg: enabled(&SerializedLnConfig{
				Currency:      boltzrpc.Currency_BTC,
				StaticAddress: "bcrt1q3287hr2zmlqgj7pdnj7vt2sx3glpnfruq7uc2s",
				SwapType:      "reverse",
			}),
			err: false,
		},
		{
			name: "Wallet/Valid",
			cfg: enabled(&SerializedLnConfig{
				Wallet:   "test",
				Currency: boltzrpc.Currency_BTC,
			}),
			wallets: []onchain.WalletInfo{{Id: 1, Name: "test", Currency: boltz.CurrencyBtc}},
			err:     false,
		},
		{
			name: "Wallet/Invalid/Currency",
			cfg: enabled(&SerializedLnConfig{
				Wallet:   "test",
				Currency: boltzrpc.Currency_BTC,
			}),
			wallets: []onchain.WalletInfo{{Id: 1, Name: "test", Currency: boltz.CurrencyLiquid}},
			err:     true,
		},
		{
			name: "Wallet/Invalid/Name",
			cfg: enabled(&SerializedLnConfig{
				Wallet:   "test",
				Currency: boltzrpc.Currency_BTC,
			}),
			err: true,
		},
		{
			name: "Wallet/Readonly",
			cfg: enabled(&SerializedLnConfig{
				Wallet:   "test",
				Currency: boltzrpc.Currency_LBTC,
				SwapType: "reverse",
			}),
			wallets: []onchain.WalletInfo{{Id: 1, Name: "test", Currency: boltz.CurrencyLiquid, Readonly: true}},
			err:     false,
		},
		{
			name: "Wallet/NoReadonly",
			cfg: enabled(&SerializedLnConfig{
				Wallet:   "test",
				Currency: boltzrpc.Currency_LBTC,
			}),
			wallets: []onchain.WalletInfo{{Id: 1, Name: "test", Currency: boltz.CurrencyLiquid, Readonly: true}},
			err:     true,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			chain := getOnchain()
			for _, wallet := range tc.wallets {
				chain.AddWallet(mockedWallet(t, wallet))
			}
			cfg := NewConfig(tc.cfg)
			err := cfg.Init(chain)
			if tc.err {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestChainConfig(t *testing.T) {
	btcWallet := onchain.WalletInfo{Id: 1, Name: "btc", Currency: boltz.CurrencyBtc}
	liquidWallet := onchain.WalletInfo{Id: 2, Name: "liquid", Currency: boltz.CurrencyLiquid}

	entityName := "test"

	tt := []struct {
		name      string
		config    *SerializedChainConfig
		wallets   []onchain.WalletInfo
		err       bool
		setEntity bool
	}{
		{
			name:   "Empty",
			config: &SerializedChainConfig{},
			err:    true,
		},
		{
			name: "NoWallets",
			config: &SerializedChainConfig{
				MaxBalance: 100,
			},
			err: true,
		},
		{
			name: "InvalidWallet",
			config: &SerializedChainConfig{
				MaxBalance: 100,
				FromWallet: "i dont",
				ToWallet:   "exist",
			},
			err: true,
		},
		{
			name: "Entity/Invalid",
			config: &SerializedChainConfig{
				MaxBalance: 100,
				Entity:     &entityName,
			},
			err: true,
		},
		{
			name: "Entity/NoWallets",
			config: &SerializedChainConfig{
				MaxBalance: 100,
				FromWallet: btcWallet.Name,
				ToWallet:   liquidWallet.Name,
			},
			setEntity: true,
			wallets:   []onchain.WalletInfo{btcWallet, liquidWallet},
			err:       true,
		},
		{
			name: "Entity/Valid",
			config: &SerializedChainConfig{
				MaxBalance: 100,
				Entity:     &entityName,
				FromWallet: btcWallet.Name,
				ToWallet:   liquidWallet.Name,
			},
			wallets:   []onchain.WalletInfo{btcWallet, liquidWallet},
			setEntity: true,
			err:       false,
		},
		{
			name: "ToWallet",
			config: &SerializedChainConfig{
				MaxBalance: 100,
				FromWallet: btcWallet.Name,
				ToWallet:   liquidWallet.Name,
			},
			wallets: []onchain.WalletInfo{btcWallet, liquidWallet},
			err:     false,
		},
		{
			name: "ToWallet/SameCurrency",
			config: &SerializedChainConfig{
				MaxBalance: 100,
				FromWallet: liquidWallet.Name,
				ToWallet:   liquidWallet.Name,
			},
			wallets: []onchain.WalletInfo{liquidWallet},
			err:     true,
		},
		{
			name: "ToAddress/Valid",

			config: &SerializedChainConfig{
				MaxBalance: 100,
				FromWallet: liquidWallet.Name,
				ToAddress:  "bcrt1q2q5f9te4va7xet4c93awrurux04h0pfwcuzzcu",
			},
			wallets: []onchain.WalletInfo{liquidWallet},
			err:     false,
		},
		{
			name: "ToAddress/SameCurrency",
			config: &SerializedChainConfig{
				MaxBalance: 100,
				FromWallet: btcWallet.Name,
				ToAddress:  "bcrt1q2q5f9te4va7xet4c93awrurux04h0pfwcuzzcu",
			},
			wallets: []onchain.WalletInfo{btcWallet},
			err:     true,
		},
		{
			name: "ToAddress/Invalid",
			config: &SerializedChainConfig{
				MaxBalance: 100,
				FromWallet: liquidWallet.Name,
				ToAddress:  "ahdslöfkjasöldfkj",
			},
			wallets: []onchain.WalletInfo{liquidWallet},
			err:     true,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			db := getTestDb(t)
			chain := getOnchain()
			entity := &database.Entity{Name: entityName}
			err := db.CreateEntity(entity)
			require.NoError(t, err)

			for _, info := range tc.wallets {
				if tc.setEntity {
					info.EntityId = entity.Id
				}
				chain.AddWallet(mockedWallet(t, info))
			}

			chainConfig := NewChainConfig(tc.config, db, chain)
			require.NotNil(t, chainConfig)
			err = chainConfig.Init()
			if tc.err {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.NotEqual(t, chainConfig.pair.From, chainConfig.pair.To)
				require.NotEmpty(t, chainConfig.description)
				require.NotZero(t, chainConfig.maxFeePercent)
				require.NotNil(t, chainConfig.entity)
			}
		})
	}
}
