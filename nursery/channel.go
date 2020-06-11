package nursery

import (
	"crypto/sha256"
	"math"
	"strconv"

	"github.com/BoltzExchange/boltz-lnd/boltz"
	"github.com/BoltzExchange/boltz-lnd/database"
	"github.com/btcsuite/btcutil"
	"github.com/google/logger"
	"github.com/lightningnetwork/lnd/lnrpc"
	"github.com/lightningnetwork/lnd/zpay32"
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
					if len(invoice.Htlcs) == 0 {
						logger.Warning("There is no pending HTLC for Channel Creation " + swap.Id)
						return
					}

					chanId := invoice.Htlcs[0].ChanId

					for _, htlc := range invoice.Htlcs {
						if htlc.ChanId != chanId {
							logger.Warning("Not all HTLCs of Channel Creation " + swap.Id + " were sent through the same channel")
							return
						}
					}

					channels, err := nursery.lnd.ListChannels()

					if err != nil {
						logger.Warning("Could not query channels")
						return
					}

					channelVerificationError := "could not find channel"

					for _, channel := range channels.Channels {
						if channel.ChanId != chanId {
							continue
						}

						if channel.RemotePubkey != nursery.boltzPubKey {
							channelVerificationError = "channel was not opened by Boltz"
							break
						}

						decodedInvoice, err := zpay32.Decode(swap.Invoice, nursery.chainParams)

						if err != nil {
							channelVerificationError = "could not decode invoice"
							break
						}

						invoiceAmount := decodedInvoice.MilliSat.ToSatoshis().ToUnit(btcutil.AmountSatoshi)
						expectedCapacity := calculateChannelCreationCapacity(invoiceAmount, channelCreation.InboundLiquidity)

						if channel.Capacity < expectedCapacity {
							channelVerificationError = "channel capacity less than than expected " + strconv.FormatInt(expectedCapacity, 10) + " satoshis"
							break
						}

						if channel.Private && !channelCreation.Private {
							channelVerificationError = "channel is not private"
							break
						} else if !channel.Private && channelCreation.Private {
							channelVerificationError = "channel is not public"
							break
						}

						channelVerificationError = ""
						break
					}

					if channelVerificationError != "" {
						logger.Warning("Could not verify channel of Channel Creation " + swap.Id + ": " + channelVerificationError)
						return
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
