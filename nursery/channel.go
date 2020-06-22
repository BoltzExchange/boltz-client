package nursery

import (
	"crypto/sha256"
	"github.com/BoltzExchange/boltz-lnd/boltz"
	"github.com/BoltzExchange/boltz-lnd/database"
	"github.com/BoltzExchange/boltz-lnd/logger"
	"github.com/lightningnetwork/lnd/lnrpc"
	"math"
)

func (nursery *Nursery) subscribeChannelCreationInvoice(swap database.Swap, channelCreation *database.ChannelCreation) (chan bool, error) {
	stopListening := make(chan bool)

	preimageHash := sha256.Sum256(swap.Preimage)
	invoiceSubscription, err := nursery.lnd.SubscribeSingleInvoice(preimageHash[:])

	if err != nil {
		return nil, err
	}

	logger.Info("Subscribed to invoice events of Channel Creation " + swap.Id)

	go func() {
		for {
			select {
			case invoice := <-invoiceSubscription:
				switch invoice.State {
				case lnrpc.Invoice_ACCEPTED:
					var expectedChannelId uint64

					channels, err := nursery.lnd.ListChannels()

					if err != nil {
						logger.Warning("Could not query channels")
						return
					}

					for _, channel := range channels.Channels {
						id, vout, err := parseChannelPoint(channel.ChannelPoint)

						if err != nil {
							logger.Error("Could not parse funding channel point: " + err.Error())
							return
						}

						if id == channelCreation.FundingTransactionId && vout == channelCreation.FundingTransactionVout {
							expectedChannelId = channel.ChanId
							break
						}
					}

					if expectedChannelId == 0 {
						logger.Error("Could not find Channel of Channel Creation " + swap.Id)
						return
					}

					for _, htlc := range invoice.Htlcs {
						if htlc.ChanId != expectedChannelId {
							logger.Error("Not all HTLCs of Channel Creation " + swap.Id + " were sent through the correct channel")
							return
						}
					}

					_, err = nursery.lnd.SettleInvoice(swap.Preimage)

					if err != nil {
						logger.Error("Could not settle invoice of Channel Creation " + swap.Id + ": " + err.Error())
					}

				case lnrpc.Invoice_SETTLED:
					nursery.updateChannelCreationStatus(channelCreation, boltz.ChannelSettled)
					return
				}

				break

			case <-stopListening:
				// TODO: cleanup invoice subscription
				return
			}
		}
	}()

	return stopListening, err
}

func (nursery *Nursery) updateChannelCreationStatus(channelCreation *database.ChannelCreation, state boltz.ChannelState) {
	err := nursery.database.UpdateChannelCreationStatus(channelCreation, state)

	if err != nil {
		logger.Error("Could not update state of Channel Creation " + channelCreation.SwapId + ": " + err.Error())
	}
}

func calculateChannelCreationCapacity(invoiceAmount float64, inboundLiquidity int) int64 {
	capacity := invoiceAmount / (1 - (float64(inboundLiquidity) / 100))
	return int64(math.Floor(capacity))
}
