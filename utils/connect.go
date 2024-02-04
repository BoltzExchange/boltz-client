package utils

import (
	"errors"
	"fmt"
	"strings"

	"github.com/BoltzExchange/boltz-client/boltz"
	"github.com/BoltzExchange/boltz-client/lightning"
	"github.com/BoltzExchange/boltz-client/logger"
)

func ConnectBoltz(lightning lightning.LightningNode, boltz *boltz.Boltz) (string, error) {
	nodes, err := boltz.GetNodes()

	if err != nil {
		return "", err
	}

	symbol := "BTC"
	node, hasNode := nodes.Nodes[symbol]

	if !hasNode {
		return "", errors.New("could not find Boltz node for symbol: " + symbol)
	}

	if len(node.URIs) == 0 {
		return node.NodeKey, errors.New("could not find URIs for Boltz LND node for symbol: " + symbol)
	}
	uri := node.URIs[0]
	err = lightning.ConnectPeer(uri)

	if err == nil {
		logger.Info("Connected to Boltz node: " + uri)
	} else if strings.Contains(err.Error(), "already connected to peer") {
		logger.Info("Already connected to Boltz node: " + uri)
		err = nil
	} else {
		err = fmt.Errorf("could not connect to boltz node %s: %w", uri, err)
	}

	return node.NodeKey, err
}
