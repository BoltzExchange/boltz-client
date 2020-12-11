package rpcserver

import (
	"crypto/tls"
	"github.com/BoltzExchange/boltz-lnd/logger"
	"github.com/BoltzExchange/boltz-lnd/utils"
	"github.com/lightningnetwork/lnd/cert"
	"os"
)

func loadCertificate(
	tlsCertPath string,
	tlsKeyPath string,
	tlsDisableAutofill bool,
) (*tls.Config, error) {
	if !utils.FileExists(tlsCertPath) || !utils.FileExists(tlsKeyPath) {
		logger.Warning("Could not find TLS certificate or key")
		logger.Info("Generating new TLS certificate and key")

		err := cert.GenCertPair(
			"boltz",
			tlsCertPath,
			tlsKeyPath,
			[]string{},
			[]string{},
			tlsDisableAutofill,
			cert.DefaultAutogenValidity,
		)

		if err != nil {
			return nil, err
		}
	}

	certData, x590cert, err := cert.LoadCert(tlsCertPath, tlsKeyPath)

	logger.Info("Loaded TLS certificate and key")

	if err != nil {
		return nil, err
	}

	if isOutdated, _ := cert.IsOutdated(x590cert, []string{}, []string{}, tlsDisableAutofill); isOutdated {
		logger.Warning("TLS certificate is outdated. Removing files and generating new one")

		err := os.Remove(tlsCertPath)

		if err != nil {
			return nil, err
		}

		err = os.Remove(tlsKeyPath)

		if err != nil {
			return nil, err
		}

		return loadCertificate(tlsCertPath, tlsKeyPath, tlsDisableAutofill)
	}

	return cert.TLSConfFromCert(certData), nil
}
