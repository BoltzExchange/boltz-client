package boltz

import (
	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/lightningnetwork/lnd/zpay32"
)

const magicRoutingHintConstant = 596385002596073472

func FindMagicRoutingHint(invoice *zpay32.Invoice) *btcec.PublicKey {
	for _, h := range invoice.RouteHints {
		for _, hint := range h {
			if hint.ChannelID == magicRoutingHintConstant {
				return hint.NodeID
			}
		}
	}
	return nil
}
