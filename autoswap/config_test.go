package autoswap

import (
	"github.com/BoltzExchange/boltz-client/database"
	"github.com/BoltzExchange/boltz-client/onchain"
	"testing"

	"github.com/BoltzExchange/boltz-client/boltz"
	"github.com/BoltzExchange/boltz-client/boltzrpc"
	"github.com/stretchr/testify/require"
)

func getShared(t *testing.T) shared {
	return shared{database: getTestDb(t), onchain: getOnchain(), rpc: NewMockRpcProvider(t)}
}

func mockRpc(shared shared) *MockRpcProvider {
	return shared.rpc.(*MockRpcProvider)
}

func TestGetPair(t *testing.T) {
	shared := getShared(t)
	cfg := NewLightningConfig(DefaultLightningConfig(), shared)

	pair := cfg.GetPair(boltzrpc.SwapType_SUBMARINE)
	require.Equal(t, boltzrpc.Currency_LBTC, pair.From)
	require.Equal(t, boltzrpc.Currency_BTC, pair.To)

	pair = cfg.GetPair(boltzrpc.SwapType_REVERSE)
	require.Equal(t, boltzrpc.Currency_LBTC, pair.To)
	require.Equal(t, boltzrpc.Currency_BTC, pair.From)

	cfg.Currency = boltzrpc.Currency_BTC

	pair = cfg.GetPair(boltzrpc.SwapType_REVERSE)
	require.Equal(t, boltzrpc.Currency_BTC, pair.To)
	require.Equal(t, boltzrpc.Currency_BTC, pair.From)
}

func TestLightningConfig(t *testing.T) {
	tt := []struct {
		name    string
		cfg     *SerializedLnConfig
		err     bool
		wallets []onchain.WalletInfo
	}{
		{name: "Default", cfg: DefaultLightningConfig(), err: false},
		{
			name: "MissingInbound",
			cfg: &SerializedLnConfig{
				OutboundBalancePercent: 25,
			},
			err: true,
		},
		{
			name: "ValidReverse",
			cfg: &SerializedLnConfig{
				InboundBalancePercent: 25,
				SwapType:              "reverse",
			},
			err: false,
		},
		{
			name: "TooMuchBalance/Percent",
			cfg: &SerializedLnConfig{
				OutboundBalancePercent: 75,
				InboundBalancePercent:  75,
			},
			err: true,
		},
		{
			name: "PerChannel/SubmarineForbidden",
			cfg: &SerializedLnConfig{
				OutboundBalance: 10000,
				PerChannel:      true,
				SwapType:        "submarine",
			},
			err: true,
		},
		{
			name: "StaticAddress/Invalid",
			cfg: &SerializedLnConfig{
				Currency:      boltzrpc.Currency_BTC,
				StaticAddress: "invalid",
				SwapType:      "reverse",
				Enabled:       true,
			},
			err: true,
		},
		{
			name: "StaticAddress/Valid",
			cfg: &SerializedLnConfig{
				Currency:      boltzrpc.Currency_BTC,
				StaticAddress: "bcrt1q3287hr2zmlqgj7pdnj7vt2sx3glpnfruq7uc2s",
				SwapType:      "reverse",
				Enabled:       true,
			},
			err: false,
		},
		{
			name: "Wallet/Valid",
			cfg: &SerializedLnConfig{
				Wallet:   "test",
				Currency: boltzrpc.Currency_BTC,
				Enabled:  true,
			},
			wallets: []onchain.WalletInfo{{Id: 1, Name: "test", Currency: boltz.CurrencyBtc}},
			err:     false,
		},
		{
			name: "Wallet/Invalid/Disabled",
			cfg: &SerializedLnConfig{
				Wallet:   "test",
				Currency: boltzrpc.Currency_BTC,
			},
			err: false,
		},
		{
			name: "Wallet/Invalid/Currency",
			cfg: &SerializedLnConfig{
				Wallet:   "test",
				Currency: boltzrpc.Currency_BTC,
				Enabled:  true,
			},
			wallets: []onchain.WalletInfo{{Id: 1, Name: "test", Currency: boltz.CurrencyLiquid}},
			err:     true,
		},
		{
			name: "Wallet/Invalid/Name",
			cfg: &SerializedLnConfig{
				Wallet:   "test",
				Currency: boltzrpc.Currency_BTC,
				Enabled:  true,
			},
			err: true,
		},
		{
			name: "Wallet/Readonly",
			cfg: &SerializedLnConfig{
				Wallet:   "test",
				Currency: boltzrpc.Currency_LBTC,
				SwapType: "reverse",
				Enabled:  true,
			},
			wallets: []onchain.WalletInfo{{Id: 1, Name: "test", Currency: boltz.CurrencyLiquid, Readonly: true}},
			err:     false,
		},
		{
			name: "Wallet/NoReadonly",
			cfg: &SerializedLnConfig{
				Wallet:   "test",
				Currency: boltzrpc.Currency_LBTC,
				Enabled:  true,
			},
			wallets: []onchain.WalletInfo{{Id: 1, Name: "test", Currency: boltz.CurrencyLiquid, Readonly: true}},
			err:     true,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			shared := getShared(t)
			for _, wallet := range tc.wallets {
				shared.onchain.AddWallet(mockedWallet{info: wallet}.Create(t))
			}
			cfg := NewLightningConfig(withThresholds(tc.cfg), shared)
			err := cfg.Init()
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

	tenantName := "test"

	tt := []struct {
		name      string
		config    *SerializedChainConfig
		wallets   []onchain.WalletInfo
		err       bool
		errFunc   require.ErrorAssertionFunc
		setTenant bool
	}{
		{
			name:   "Empty",
			config: &SerializedChainConfig{},
			err:    true,
		},
		{
			name: "NoWallets",
			config: &SerializedChainConfig{
				MaxBalance: 100000,
			},
			err: true,
		},
		{
			name: "InvalidWallet",
			config: &SerializedChainConfig{
				MaxBalance: 100000,
				FromWallet: "i dont",
				ToWallet:   "exist",
			},
			err: true,
		},
		{
			name: "Tenant/Invalid",
			config: &SerializedChainConfig{
				MaxBalance: 100000,
				Tenant:     &tenantName,
			},
			err: true,
		},
		{
			name: "Tenant/NoWallets",
			config: &SerializedChainConfig{
				MaxBalance: 100000,
				FromWallet: btcWallet.Name,
				ToWallet:   liquidWallet.Name,
			},
			setTenant: true,
			wallets:   []onchain.WalletInfo{btcWallet, liquidWallet},
			err:       true,
		},
		{
			name: "Tenant/Valid",
			config: &SerializedChainConfig{
				MaxBalance: 100000,
				Tenant:     &tenantName,
				FromWallet: btcWallet.Name,
				ToWallet:   liquidWallet.Name,
			},
			wallets:   []onchain.WalletInfo{btcWallet, liquidWallet},
			setTenant: true,
			err:       false,
		},
		{
			name: "ToWallet",
			config: &SerializedChainConfig{
				MaxBalance: 100000,
				FromWallet: btcWallet.Name,
				ToWallet:   liquidWallet.Name,
			},
			wallets: []onchain.WalletInfo{btcWallet, liquidWallet},
			err:     false,
		},
		{
			name: "ToWallet/SameCurrency",
			config: &SerializedChainConfig{
				MaxBalance: 100000,
				FromWallet: liquidWallet.Name,
				ToWallet:   liquidWallet.Name,
			},
			wallets: []onchain.WalletInfo{liquidWallet},
			err:     true,
		},
		{
			name: "ToAddress/Valid",

			config: &SerializedChainConfig{
				MaxBalance: 100000,
				FromWallet: liquidWallet.Name,
				ToAddress:  "bcrt1q2q5f9te4va7xet4c93awrurux04h0pfwcuzzcu",
			},
			wallets: []onchain.WalletInfo{liquidWallet},
			err:     false,
		},
		{
			name: "ToAddress/SameCurrency",
			config: &SerializedChainConfig{
				MaxBalance: 100000,
				FromWallet: btcWallet.Name,
				ToAddress:  "bcrt1q2q5f9te4va7xet4c93awrurux04h0pfwcuzzcu",
			},
			wallets: []onchain.WalletInfo{btcWallet},
			err:     true,
		},
		{
			name: "ToAddress/Invalid",
			config: &SerializedChainConfig{
				MaxBalance: 100000,
				FromWallet: liquidWallet.Name,
				ToAddress:  "ahdslöfkjasöldfkj",
			},
			wallets: []onchain.WalletInfo{liquidWallet},
			err:     true,
		},
		{
			name: "ReserveGreaterThanMax",
			config: &SerializedChainConfig{
				ReserveBalance: 200,
				MaxBalance:     100,
			},
			errFunc: func(t require.TestingT, err error, i ...interface{}) {
				require.ErrorContains(t, err, "reserve balance", i...)
			},
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			shared := getShared(t)
			tenant := &database.Tenant{Name: tenantName}
			err := shared.database.CreateTenant(tenant)
			require.NoError(t, err)

			for _, info := range tc.wallets {
				if tc.setTenant {
					info.TenantId = tenant.Id
				}
				shared.onchain.AddWallet(mockedWallet{info: info}.Create(t))
			}

			chainConfig := NewChainConfig(tc.config, shared)
			require.NotNil(t, chainConfig)
			err = chainConfig.Init()
			if tc.errFunc != nil {
				tc.errFunc(t, err)
			} else if tc.err {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.NotEqual(t, chainConfig.pair.From, chainConfig.pair.To)
				require.NotEmpty(t, chainConfig.description)
				require.NotZero(t, chainConfig.maxFeePercent)
				require.NotNil(t, chainConfig.tenant)
			}
		})
	}
}
