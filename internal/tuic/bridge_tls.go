package tuic

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"math/big"
	"strings"
	"sync"
	"time"
)

const bridgeServerName = "tuic-bridge.local"

type bridgeIdentity struct {
	certificate tls.Certificate
	certPEM     []byte
	keyPEM      []byte
	client      *tls.Config
}

var sharedBridgeIdentity = sync.OnceValues(newBridgeIdentity)

func newBridgeIdentity() (*bridgeIdentity, error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, err
	}
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return nil, err
	}
	now := time.Now()
	template := &x509.Certificate{
		SerialNumber: serial,
		DNSNames:     []string{bridgeServerName},
		NotBefore:    now.Add(-5 * time.Minute), NotAfter: now.AddDate(10, 0, 0),
		KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		return nil, err
	}
	keyDER, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		return nil, err
	}
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER})
	certificate, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		return nil, err
	}
	roots := x509.NewCertPool()
	roots.AppendCertsFromPEM(certPEM)
	return &bridgeIdentity{
		certificate: certificate, certPEM: certPEM, keyPEM: keyPEM,
		client: &tls.Config{RootCAs: roots, ServerName: bridgeServerName, MinVersion: tls.VersionTLS13},
	}, nil
}

// BridgeStreamSettings gives only the process-owned Xray bridge its server
// identity. The relay verifies this identity before sending SOCKS credentials.
func BridgeStreamSettings() ([]byte, error) {
	identity, err := sharedBridgeIdentity()
	if err != nil {
		return nil, err
	}
	return json.Marshal(map[string]any{
		"network": "tcp", "security": "tls",
		"tlsSettings": map[string]any{
			"minVersion": "1.3",
			"certificates": []map[string]any{{
				"certificate": strings.Split(strings.TrimSpace(string(identity.certPEM)), "\n"),
				"key":         strings.Split(strings.TrimSpace(string(identity.keyPEM)), "\n"),
			}},
		},
	})
}
