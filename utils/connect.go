package utils

import (
	"errors"
	"fmt"
	"net"
	"strings"

	"github.com/BoltzExchange/boltz-client/boltz"
	"github.com/BoltzExchange/boltz-client/lightning"
	"github.com/BoltzExchange/boltz-client/logger"
)

func searchUri(network string, uris []string) string {
	for _, uri := range uris {
		split := strings.Split(uri, "@")
		if len(split) < 2 {
			continue
		}
		split = strings.Split(split[1], ":")
		if len(split) < 2 {
			continue
		}
		// remove the port
		split = split[:len(split)-1]
		if network != "tor" {
			_, err := net.ResolveIPAddr(network, strings.Join(split, ":"))
			if err == nil {
				return uri
			}
		} else if strings.Contains(uri, ".onion") {
			return uri
		}
	}
	return ""
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
		fullName := fmt.Sprintf("%s (%s)", name, node.PublicKey)
		if len(node.Uris) == 0 {
			logger.Warnf("no uris for node %s", fullName)
			continue
		}
		for _, network := range []string{"ip4", "ip6", "tor"} {
			uri := searchUri(network, node.Uris)
			if uri == "" {
				logger.Infof("no %s uri for node %s", network, fullName)
				continue
			}
			err := lightning.ConnectPeer(uri)
			if err == nil || strings.Contains(err.Error(), "already connected to peer") {
				err = nil
				break
			}
		}
		if err == nil {
			logger.Infof("Connected to Boltz node %s", fullName)
		} else {
			logger.Errorf("could not connect to boltz node %s: %v", fullName, err)
		}
	}
	return err
}
