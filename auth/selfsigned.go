// support HTTP/2.
package auth

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"github.com/charmbracelet/log"
	"math/big"
	"net/http"
	"os"
	"path"
	"time"
)

// Certificate returns a self-signed x509 certificate and private key.
func Certificate(dataDir, ip string) (tls.Certificate, error) {
	certPEM, _ := os.ReadFile(path.Join(dataDir, "cert.pem"))
	keyPEM, _ := os.ReadFile(path.Join(dataDir, "key.pem"))
	if keyPEM == nil || certPEM == nil {
		log.Debug("Generating self-signed certificate")
		key, err := rsa.GenerateKey(rand.Reader, 2048)
		if err != nil {
			return tls.Certificate{}, err
		}

		template := x509.Certificate{
			SerialNumber: big.NewInt(1234),
			Issuer: pkix.Name{
				CommonName: ip,
			},
			Subject: pkix.Name{
				CommonName: ip,
			},
			// We pre-expire the certs to make them explicitly invalid; they're only
			// useful in contexts where they are not verified or validated.
			NotBefore:   time.Now(),
			NotAfter:    time.Now(),
			DNSNames:    []string{ip},
			KeyUsage:    x509.KeyUsageDigitalSignature,
			ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		}

		certificateBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, &key.PublicKey, key)
		if err != nil {
			return tls.Certificate{}, err
		}

		certPEMBuf := new(bytes.Buffer)
		err = pem.Encode(certPEMBuf, &pem.Block{
			Type:  "CERTIFICATE",
			Bytes: certificateBytes,
		})
		if err != nil {
			return tls.Certificate{}, err
		}

		keyPEMBuf := new(bytes.Buffer)
		err = pem.Encode(keyPEMBuf, &pem.Block{
			Type:  "RSA PRIVATE KEY",
			Bytes: x509.MarshalPKCS1PrivateKey(key),
		})
		if err != nil {
			return tls.Certificate{}, err
		}
		certPEM = certPEMBuf.Bytes()
		keyPEM = keyPEMBuf.Bytes()
	} else {
		log.Debug("Using existing self-signed certificate")
	}
	return tls.X509KeyPair(certPEM, keyPEM)
}

// SelfSignedListenAndServeTLS listens on the TCP network address addr and then calls
// Serve with handler and a self-signed certificate to handle requests on
// incoming TLS connections.
func SelfSignedListenAndServeTLS(dataDir, publicIP, addr string, handler http.Handler) error {
	cert, err := Certificate(dataDir, publicIP)
	if err != nil {
		return fmt.Errorf("failed to generate certificate: %w", err)
	}

	conf := &tls.Config{
		Certificates: []tls.Certificate{cert},
	}
	server := &http.Server{Addr: addr, Handler: handler, TLSConfig: conf}
	return server.ListenAndServeTLS("", "")
}
