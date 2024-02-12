package utils

import (
	"errors"
	"fmt"
	"strings"

	"github.com/BoltzExchange/boltz-client/boltz"
	"github.com/BoltzExchange/boltz-client/lightning"
	"github.com/BoltzExchange/boltz-client/logger"
)

func connect(node lightning.LightningNode, peer boltz.NodeInfo) error {
	var uri string
	for _, current := range peer.Uris {
		uri = current
		// prefer clearnet
		if !strings.Contains(current, "onion") {
			break
		}
	}
	if uri == "" {
		return fmt.Errorf("no uri for peer: %s", peer.PublicKey)
	}
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
	nodesForSymbol, hasNodesForSymbol := nodes[symbol]

	if !hasNodesForSymbol {
		return errors.New("could not find boltz nodes for symbol: " + symbol)
	}

	for name, node := range nodesForSymbol {
		if err := connect(lightning, node); err != nil {
			logger.Errorf("Could not connect to node %s: %s", name, err)
		}
	}
	return err
}
