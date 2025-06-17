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
	"math/big"
	"net/http"
	"os"
	"path"
	"time"

	"github.com/charmbracelet/log"
	"github.com/mroth/jitter"
)

func generateSelfSignedCert(key *rsa.PrivateKey, ip string) ([]byte, error) {
	sn, _ := rand.Int(rand.Reader, big.NewInt(1000000000))
	template := x509.Certificate{
		SerialNumber: sn,
		Issuer: pkix.Name{
			CommonName: ip,
		},
		Subject: pkix.Name{
			CommonName: ip,
		},
		NotBefore:   time.Now(),
		DNSNames:    []string{ip},
		KeyUsage:    x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}

	return x509.CreateCertificate(rand.Reader, &template, &template, &key.PublicKey, key)
}

// Certificate generates or loads a self-signed TLS certificate and private key.
// It first attempts to read "cert.pem" and "key.pem" from the specified dataDir.
// If the files don't exist, it generates a new RSA 2048-bit key pair and a
// self-signed certificate valid only for the given IP address. The certificate
// is intentionally created with an immediate expiration time, making it suitable
// only for contexts where validation is skipped or customized.
// The generated PEM-encoded certificate and key are returned.
// If certPEMFile and keyPEMFile are provided, it uses those files instead of generating a new one.
func Certificate(dataDir, ip, certPEMFile, keyPEMFile string) (tls.Certificate, error) {
	if certPEMFile != "" && keyPEMFile != "" {
		certPEM, err := os.ReadFile(certPEMFile)
		if err != nil {
			return tls.Certificate{}, err
		}
		keyPEM, err := os.ReadFile(keyPEMFile)
		if err != nil {
			return tls.Certificate{}, err
		}
		return tls.X509KeyPair(certPEM, keyPEM)
	}
	certPEM, _ := os.ReadFile(path.Join(dataDir, "cert.pem"))
	keyPEM, _ := os.ReadFile(path.Join(dataDir, "key.pem"))
	if keyPEM == nil || certPEM == nil {
		log.Debug("Generating self-signed certificate")
		key, err := rsa.GenerateKey(rand.Reader, 2048)
		if err != nil {
			return tls.Certificate{}, err
		}

		certificateBytes, err := generateSelfSignedCert(key, ip)
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
		_ = os.WriteFile(path.Join(dataDir, "cert.pem"), certPEMBuf.Bytes(), 0644)
		_ = os.WriteFile(path.Join(dataDir, "key.pem"), keyPEMBuf.Bytes(), 0644)

	} else {
		log.Debug("Using existing self-signed certificate")
	}
	return tls.X509KeyPair(certPEM, keyPEM)
}

// CheckConnectivity periodically checks the health endpoint of the server using its public IP and port.
// It runs in a loop, making requests every minute with jitter.
// It uses an HTTP client configured to skip TLS verification, suitable for self-signed certificates.
// Errors during the check are logged.
func CheckConnectivity(ip string, port int) {
	time.Sleep(1 * time.Second) // Initial delay before the first check
	ticker := jitter.NewTicker(time.Minute, 0.2)
	defer ticker.Stop()

	client := http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		},
	}
	for {
		req, _ := http.NewRequest("GET", fmt.Sprintf("https://%s:%d/api/v1/health", ip, port), nil)

		_, err := client.Do(req)
		if err != nil {
			log.Errorf("Connectivity check failed. Please check the configuration, make sure that port %d is open. Error: %v", port, err)
		}
		<-ticker.C
	}
}

// ListenAndServeTLS listens on the TCP network address addr and then calls
// Serve with handler and a self-signed certificate (optionally signed by Lantern) to handle requests on
// incoming TLS connections.
func ListenAndServeTLS(dataDir, certPEM, keyPEM string, publicIP string, listenPort int, handler http.Handler) error {
	cert, err := Certificate(dataDir, publicIP, certPEM, keyPEM)
	if err != nil {
		return fmt.Errorf("failed to generate certificate: %w", err)
	}

	conf := &tls.Config{
		Certificates: []tls.Certificate{cert},
	}

	go CheckConnectivity(publicIP, listenPort)
	addr := fmt.Sprintf(":%d", listenPort)
	server := &http.Server{Addr: addr, Handler: handler, TLSConfig: conf}
	return server.ListenAndServeTLS("", "")
}
