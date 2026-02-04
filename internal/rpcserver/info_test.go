//go:build !unit

package rpcserver

import (
	"bytes"
	"testing"

	"github.com/BoltzExchange/boltz-client/v2/internal/onchain"
	"github.com/btcsuite/btcd/btcutil"

	"github.com/vulpemventures/go-elements/address"

	lnmock "github.com/BoltzExchange/boltz-client/v2/internal/mocks/lightning"
	"github.com/BoltzExchange/boltz-client/v2/pkg/boltzrpc/client"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/txscript"

	"github.com/BoltzExchange/boltz-client/v2/internal/lightning"
	"github.com/BoltzExchange/boltz-client/v2/pkg/boltz"
	"github.com/BoltzExchange/boltz-client/v2/pkg/boltzrpc"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
)

func TestGetInfo(t *testing.T) {
	nodes := []string{"CLN", "LND", "Standalone"}

	for _, node := range nodes {
		node := node
		t.Run(node, func(t *testing.T) {
			client, _, stop := setup(t, setupOptions{node: node})
			defer stop()

			info, err := client.GetInfo()

			require.NoError(t, err)
			require.Equal(t, "regtest", info.Network)
		})
	}

	t.Run("Syncing", func(t *testing.T) {
		node := lnmock.NewMockLightningNode(t)

		node.EXPECT().Connect().Return(nil)
		node.EXPECT().GetInfo().Return(&lightning.LightningInfo{Synced: false}, nil)

		client, _, stop := setup(t, setupOptions{lightning: node, dontSync: true})
		defer stop()

		_, err := client.GetInfo()
		require.Error(t, err)
		requireCode(t, err, codes.Unavailable)
		require.ErrorContains(t, err, "lightning node")
	})
}

func createTenant(t *testing.T, admin client.Boltz, name string) (*boltzrpc.Tenant, client.Connection, client.Connection) {
	tenantInfo, err := admin.CreateTenant(name)
	require.NoError(t, err)
	require.NotZero(t, tenantInfo.Id)

	write, err := admin.BakeMacaroon(&boltzrpc.BakeMacaroonRequest{
		TenantId:    &tenantInfo.Id,
		Permissions: client.FullPermissions,
	})
	require.NoError(t, err)

	read, err := admin.BakeMacaroon(&boltzrpc.BakeMacaroonRequest{
		TenantId:    &tenantInfo.Id,
		Permissions: client.ReadPermissions,
	})
	require.NoError(t, err)

	readConn := admin.Connection
	readConn.SetMacaroon(read.Macaroon)

	writeConn := admin.Connection
	writeConn.SetMacaroon(write.Macaroon)

	return tenantInfo, writeConn, readConn
}

func TestGetPairs(t *testing.T) {
	client, _, stop := setup(t, setupOptions{})
	defer stop()

	info, err := client.GetPairs()

	require.NoError(t, err)
	require.Len(t, info.Submarine, 2)
	require.Len(t, info.Reverse, 2)
	require.Len(t, info.Chain, 2)
}

func checkTxOutAddress(t *testing.T, chain *onchain.Onchain, currency boltz.Currency, txId string, outAddress string, cooperative bool) {
	transaction, err := chain.GetTransaction(currency, txId, nil, false)
	require.NoError(t, err)

	if tx, ok := transaction.(*boltz.BtcTransaction); ok {

		for _, input := range tx.MsgTx().TxIn {
			if cooperative {
				require.Len(t, input.Witness, 1)
			} else {
				require.Greater(t, len(input.Witness), 1)
			}
		}

		if outAddress != "" {
			decoded, err := btcutil.DecodeAddress(outAddress, &chaincfg.RegressionNetParams)
			require.NoError(t, err)
			script, err := txscript.PayToAddrScript(decoded)
			require.NoError(t, err)
			for _, output := range tx.MsgTx().TxOut {
				if bytes.Equal(output.PkScript, script) {
					return
				}
			}
			require.Fail(t, "could not find output address in transaction")
		}
	} else if tx, ok := transaction.(*boltz.LiquidTransaction); ok {
		for _, input := range tx.Inputs {
			if cooperative {
				require.Len(t, input.Witness, 1)
			} else {
				require.Greater(t, len(input.Witness), 1)
			}
		}
		if outAddress != "" {
			script, err := address.ToOutputScript(outAddress)
			require.NoError(t, err)
			for _, output := range tx.Outputs {
				if len(output.Script) == 0 {
					continue
				}
				if bytes.Equal(output.Script, script) {
					return
				}
			}
			require.Fail(t, "could not find output address in transaction")
		}
	}
}

func parseCurrency(grpcCurrency boltzrpc.Currency) boltz.Currency {
	if grpcCurrency == boltzrpc.Currency_BTC {
		return boltz.CurrencyBtc
	} else {
		return boltz.CurrencyLiquid
	}
}

var pairBtc = &boltzrpc.Pair{
	From: boltzrpc.Currency_BTC,
	To:   boltzrpc.Currency_BTC,
}
