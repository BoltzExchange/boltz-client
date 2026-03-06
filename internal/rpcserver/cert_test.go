package rpcserver

import (
	"os"
	"testing"

	"github.com/BoltzExchange/boltz-client/v2/internal/test"

	"github.com/stretchr/testify/assert"
)

const (
	tlsKeyPath  = "./tls.key"
	tlsCertPath = "./tls.cert"
)

func TestLoadCertificate(t *testing.T) {
	test.InitLogger()

	cleanup(t)

	// Should create new certificate
	newCert, err := loadCertificate(tlsCertPath, tlsKeyPath, true)
	assert.Equal(t, nil, err)

	assert.Equal(t, true, fileExists(tlsCertPath))
	assert.Equal(t, true, fileExists(tlsKeyPath))

	// Should load the generated certificate
	loadedCert, err := loadCertificate(tlsCertPath, tlsKeyPath, true)
	assert.Equal(t, nil, err)

	assert.Equal(t, newCert, loadedCert, "generated and loaded certificate do not match")

	// Should renew expired certificates
	renewedCert, err := loadCertificate(tlsCertPath, tlsKeyPath, false)
	assert.Equal(t, nil, err)

	assert.Equal(t, true, fileExists(tlsCertPath))
	assert.Equal(t, true, fileExists(tlsKeyPath))

	assert.NotEqual(t, loadedCert, renewedCert, "expired certificates are not renewed")

	cleanup(t)
}

func cleanup(t *testing.T) {
	removeFileIfExists := func(path string) {
		if fileExists(path) {
			err := os.Remove(path)
			assert.Equal(t, nil, err)
		}
	}

	removeFileIfExists(tlsKeyPath)
	removeFileIfExists(tlsCertPath)
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return !os.IsNotExist(err)
}
