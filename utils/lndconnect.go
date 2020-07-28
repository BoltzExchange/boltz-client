package utils

import (
	"errors"
	"github.com/BoltzExchange/boltz-lnd/boltz"
	"github.com/BoltzExchange/boltz-lnd/lnd"
	"github.com/BoltzExchange/boltz-lnd/logger"
	"strings"
)

func ConnectBoltzLnd(lnd *lnd.LND, boltz *boltz.Boltz, symbol string) (string, error) {
	nodes, err := boltz.GetNodes()

	if err != nil {
		return "", err
	}

	node, hasNode := nodes.Nodes[symbol]

	if !hasNode {
		return "", errors.New("could not find Boltz LND node for symbol: " + symbol)
	}

	if len(node.URIs) == 0 {
		return node.NodeKey, errors.New("could not find URIs for Boltz LND node for symbol: " + symbol)
	}

	uriParts := strings.Split(node.URIs[0], "@")

	if len(uriParts) != 2 {
		return node.NodeKey, errors.New("could not parse URI of Boltz LND")
	}

	_, err = lnd.ConnectPeer(uriParts[0], uriParts[1])

	if err == nil {
		logger.Info("Connected to Boltz LND node: " + node.URIs[0])
	} else if strings.HasPrefix(err.Error(), "rpc error: code = Unknown desc = already connected to peer") {
		logger.Info("Already connected to Boltz LND node: " + node.URIs[0])
		err = nil
	}

	return node.NodeKey, err
}
