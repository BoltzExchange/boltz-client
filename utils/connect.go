package utils

import (
	"errors"
	"fmt"
	"strings"

	"github.com/BoltzExchange/boltz-client/boltz"
	"github.com/BoltzExchange/boltz-client/lightning"
	"github.com/BoltzExchange/boltz-client/logger"
)

func connect(node lightning.LightningNode, peer *boltz.NodeInfo) error {
	if len(peer.Uris) == 0 {
		return fmt.Errorf("no uris found for peer: %s", peer.PublicKey)
	}
	uri := peer.Uris[0]
	err := node.ConnectPeer(uri)
	if err == nil {
		logger.Infof("Connected to Boltz node: %s", uri)
	} else if strings.Contains(err.Error(), "already connected to peer") {
		logger.Infof("Already connected to Boltz node: %s", uri)
		err = nil
	} else {
		err = fmt.Errorf("could not connect to boltz node %s: %w", uri, err)
	}

	return err
}

func ConnectBoltz(lightning lightning.LightningNode, boltz *boltz.Boltz) error {
	nodes, err := boltz.GetNodes()
	if err != nil {
		return err
	}

	symbol := "BTC"
	btcNodes, hasNode := nodes[symbol]

	if !hasNode {
		return errors.New("could not find Boltz node for symbol: " + symbol)
	}

	if err := connect(lightning, btcNodes.LND); err != nil {
		return err
	}

	if err := connect(lightning, btcNodes.CLN); err != nil {
		return err
	}
	return err
}
