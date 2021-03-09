package database

import (
	"database/sql"
	"errors"
	"github.com/BoltzExchange/boltz-lnd/boltz"
)

type ChannelCreation struct {
	SwapId                 string
	Status                 boltz.ChannelState
	InboundLiquidity       uint32
	Private                bool
	FundingTransactionId   string
	FundingTransactionVout uint32
}

type ChannelCreationSerialized struct {
	SwapId                 string
	Status                 string
	InboundLiquidity       uint32
	Private                bool
	FundingTransactionId   string
	FundingTransactionVout uint32
}

func (channelCreation *ChannelCreation) Serialize() ChannelCreationSerialized {
	return ChannelCreationSerialized{
		SwapId:                 channelCreation.SwapId,
		Status:                 channelCreation.Status.String(),
		InboundLiquidity:       channelCreation.InboundLiquidity,
		Private:                channelCreation.Private,
		FundingTransactionId:   channelCreation.FundingTransactionId,
		FundingTransactionVout: channelCreation.FundingTransactionVout,
	}
}

func parseChannelCreation(rows *sql.Rows) (*ChannelCreation, error) {
	var channelCreation ChannelCreation

	var status string

	err := scanRow(
		rows,
		map[string]interface{}{
			"swapId":                 &channelCreation.SwapId,
			"status":                 &status,
			"inboundLiquidity":       &channelCreation.InboundLiquidity,
			"private":                &channelCreation.Private,
			"fundingTransactionId":   &channelCreation.FundingTransactionId,
			"fundingTransactionVout": &channelCreation.FundingTransactionVout,
		},
	)

	if err != nil {
		return nil, err
	}

	channelCreation.Status = boltz.ParseChannelState(status)

	return &channelCreation, err
}

func (database *Database) QueryChannelCreation(id string) (channelCreation *ChannelCreation, err error) {
	rows, err := database.db.Query("SELECT * FROM channelCreations WHERE swapId = '" + id + "'")

	if err != nil {
		return channelCreation, err
	}

	defer rows.Close()

	if rows.Next() {
		channelCreation, err = parseChannelCreation(rows)
	} else {
		return channelCreation, errors.New("could not find Channel Creation " + id)
	}

	return channelCreation, err
}

func (database *Database) CreateChannelCreation(channelCreation ChannelCreation) error {
	insertStatement := "INSERT INTO channelCreations (swapId, status, inboundLiquidity, private, fundingTransactionId, fundingTransactionVout) VALUES (?, ?, ?, ?, ?, ?)"
	statement, err := database.db.Prepare(insertStatement)

	if err != nil {
		return err
	}

	_, err = statement.Exec(
		channelCreation.SwapId,
		channelCreation.Status.String(),
		channelCreation.InboundLiquidity,
		channelCreation.Private,
		channelCreation.FundingTransactionId,
		channelCreation.FundingTransactionVout,
	)

	if err != nil {
		return err
	}

	return statement.Close()
}

func (database *Database) SetChannelFunding(channelCreation *ChannelCreation, fundingTransactionId string, fundingTransactionVout uint32) error {
	channelCreation.Status = boltz.ChannelAccepted
	channelCreation.FundingTransactionId = fundingTransactionId
	channelCreation.FundingTransactionVout = fundingTransactionVout

	_, err := database.db.Exec(
		"UPDATE channelCreations SET status = ?, fundingTransactionId = ?, fundingTransactionVout = ? WHERE swapId = ?",
		boltz.ChannelAccepted.String(),
		fundingTransactionId,
		fundingTransactionVout,
		channelCreation.SwapId,
	)

	return err
}

func (database *Database) UpdateChannelCreationStatus(channelCreation *ChannelCreation, status boltz.ChannelState) error {
	channelCreation.Status = status

	_, err := database.db.Exec("UPDATE channelCreations SET status = '" + status.String() + "' WHERE swapId = '" + channelCreation.SwapId + "'")
	return err
}
