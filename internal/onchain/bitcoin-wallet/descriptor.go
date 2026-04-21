package bitcoin_wallet

import (
	"github.com/BoltzExchange/boltz-client/v2/internal/onchain/bitcoin-wallet/bdk"
	"github.com/BoltzExchange/boltz-client/v2/pkg/boltz"
)

func DeriveDefaultDescriptor(network *boltz.Network, mnemonic string) (string, error) {
	return bdk.DeriveDefaultXpub(convertNetwork(network), mnemonic)
}
