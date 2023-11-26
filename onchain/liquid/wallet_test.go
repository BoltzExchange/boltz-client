package liquid

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/BoltzExchange/boltz-client/boltz"
	"github.com/BoltzExchange/boltz-client/logger"
	"github.com/BoltzExchange/boltz-client/onchain"
	"github.com/BoltzExchange/boltz-client/test"
	"github.com/stretchr/testify/require"
)

var wallet *Wallet

func InitTestWallet() (*Wallet, error) {
	var err error
	test.InitLogger()
	wallet, err := InitWallet("./test-data", boltz.Regtest, true)
	if err != nil {
		return nil, err
	}
	if wallet.Exists() {
		if err := wallet.Login(); err != nil {
			return nil, err
		}
		time.Sleep(200 * time.Millisecond)
	} else {
		if _, err := wallet.Register(); err != nil {
			return nil, err
		}
	}

	balance, err := wallet.GetBalance()
	if err != nil {
		return nil, err
	}
	if balance.Confirmed == 0 {
		addr, err := wallet.NewAddress()
		if err != nil {
			return nil, err
		}
		test.LiquidCli("sendtoaddress " + addr + " 1")
		test.MineBlock()
		ticker := time.NewTicker(1 * time.Second)
		timeout := time.After(15 * time.Second)
		for {
			select {
			case <-ticker.C:
				balance, err := wallet.GetBalance()
				if err != nil {
					return nil, err
				}
				if balance.Confirmed > 0 {
					return wallet, nil
				}
			case <-timeout:
				return nil, fmt.Errorf("timeout")
			}
		}
	}
	return wallet, nil
}

func TestMain(m *testing.M) {
	var err error
	wallet, err = InitTestWallet()
	if err != nil {
		logger.Fatal(err.Error())
	}
	os.Exit(m.Run())
}

func TestBalance(t *testing.T) {
	balance, err := wallet.GetBalance()
	require.NoError(t, err)
	require.NotZero(t, balance.Total)
}

func TestSend(t *testing.T) {
	txid, err := wallet.SendToAddress(test.LiquidCli("getnewaddress"), 10000, 1)
	fmt.Println(txid)
	require.NoError(t, err)
	test.MineBlock()
}

func TestFee(t *testing.T) {
	fee, err := wallet.EstimateFee(1)
	require.NoError(t, err)
	require.NotZero(t, fee)
}

func TestBlockStream(t *testing.T) {
	blocks := make(chan *onchain.BlockEpoch)
	go func() {
		err := wallet.RegisterBlockListener(blocks)
		require.NoError(t, err)
	}()
	test.MineBlock()
	block := <-blocks

	require.NotEqual(t, 0, block.Height)

	height, err := wallet.GetBlockHeight()
	require.NoError(t, err)
	require.Equal(t, block.Height, height)
}

func TestReal(t *testing.T) {
	subaccounts, err := wallet.GetSubaccounts(true)
	require.NoError(t, err)
	require.NotZero(t, subaccounts)

	balance, err := wallet.GetBalance()
	require.NoError(t, err)
	require.NotZero(t, balance.Total)

	require.NoError(t, wallet.SetSubaccount(nil))
	balance, err = wallet.GetBalance()
	require.NoError(t, err)
	require.Zero(t, balance)
}
