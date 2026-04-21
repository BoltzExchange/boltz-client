package liquid_wallet

import (
	"github.com/BoltzExchange/boltz-client/v2/internal/onchain/liquid-wallet/lwk"
	"github.com/BoltzExchange/boltz-client/v2/pkg/boltz"
)

func DeriveDefaultDescriptor(network *boltz.Network, mnemonic string) (string, error) {
	return DeriveSinglesigDescriptor(network, mnemonic, lwk.SinglesigWpkh)
}

func DeriveSinglesigDescriptor(network *boltz.Network, mnemonic string, script lwk.Singlesig) (string, error) {
	signer, err := newSigner(network, mnemonic)
	if err != nil {
		return "", err
	}
	descriptor, err := signer.SinglesigDesc(script, lwk.DescriptorBlindingKeySlip77)
	if err != nil {
		return "", err
	}
	return descriptor.String(), nil
}
