package rpcserver

import (
	"crypto/tls"
	"github.com/BoltzExchange/boltz-client/logger"
	"github.com/BoltzExchange/boltz-client/utils"
	"github.com/lightningnetwork/lnd/cert"
	"os"
	"time"
)

func loadCertificate(
	tlsCertPath string,
	tlsKeyPath string,
	tlsDisableAutofill bool,
) (*tls.Config, error) {
	if !utils.FileExists(tlsCertPath) || !utils.FileExists(tlsKeyPath) {
		logger.Warn("Could not find TLS certificate or key")
		logger.Info("Generating new TLS certificate and key")

		defaultAutogenValidity := 14 * 30 * 24 * time.Hour

		certBytes, keyBytes, err := cert.GenCertPair(
			"boltz",
			[]string{},
			[]string{},
			tlsDisableAutofill,
			defaultAutogenValidity,
		)

		if err != nil {
			return nil, err
		}

		err = cert.WriteCertPair(tlsCertPath, tlsKeyPath, certBytes, keyBytes)
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
		logger.Warn("TLS certificate is outdated. Removing files and generating new one")

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
