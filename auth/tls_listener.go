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
	"io"
	"math/big"
	"net"
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
		// We pre-expire the certs to make them explicitly invalid; they're only
		// useful in contexts where they are not verified or validated.
		NotBefore:   time.Now(),
		NotAfter:    time.Now(),
		DNSNames:    []string{ip},
		KeyUsage:    x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}

	return x509.CreateCertificate(rand.Reader, &template, &template, &key.PublicKey, key)
}

func generateSignedCert(key *rsa.PrivateKey, ip string) ([]byte, error) {
	log.Debug("Requesting signed certificate from Lantern")
	subj := pkix.Name{
		CommonName: ip,
	}

	csrTemplate := x509.CertificateRequest{
		Subject:            subj,
		SignatureAlgorithm: x509.SHA256WithRSA,
	}

	csrTemplate.DNSNames = []string{ip}
	csrTemplate.IPAddresses = []net.IP{net.ParseIP(ip)}
	csrBytes, err := x509.CreateCertificateRequest(rand.Reader, &csrTemplate, key)
	if err != nil {
		return nil, fmt.Errorf("failed to create CSR: %w", err)
	}

	csrPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE REQUEST", Bytes: csrBytes})

	resp, err := http.Post("https://bo.reflog.me/v1/certificate/sign", "application/x-pem-file", bytes.NewReader(csrPEM))
	if err != nil {
		return nil, fmt.Errorf("failed to send CSR: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get signed certificate: %s", resp.Status)
	}
	certificatePEM, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read signed certificate: %w", err)
	}
	privateKeyPEM := pem.EncodeToMemory(
		&pem.Block{
			Type:  "RSA PRIVATE KEY",
			Bytes: x509.MarshalPKCS1PrivateKey(key),
		},
	)

	// Create a tls.Certificate object using our local private key and the signed cert from Vault.
	// certificatePEM from Vault should contain the signed cert + CA chain.
	tlsCert, err := tls.X509KeyPair(certificatePEM, privateKeyPEM)
	if err != nil {
		return nil, fmt.Errorf("failed to load generated X509 key pair: %w", err)
	}
	return tlsCert.Certificate[0], nil
}

// Certificate generates or loads a self-signed TLS certificate and private key.
// It first attempts to read "cert.pem" and "key.pem" from the specified dataDir.
// If the files don't exist, it generates a new RSA 2048-bit key pair and a
// self-signed certificate valid only for the given IP address. The certificate
// is intentionally created with an immediate expiration time, making it suitable
// only for contexts where validation is skipped or customized.
// The generated PEM-encoded certificate and key are returned.
// If signCert is true, the certificate will be signed by Lantern's API.
func Certificate(dataDir, ip string, signCert bool) (tls.Certificate, error) {
	certPEM, _ := os.ReadFile(path.Join(dataDir, "cert.pem"))
	keyPEM, _ := os.ReadFile(path.Join(dataDir, "key.pem"))
	if keyPEM == nil || certPEM == nil {
		log.Debug("Generating self-signed certificate")
		key, err := rsa.GenerateKey(rand.Reader, 2048)
		if err != nil {
			return tls.Certificate{}, err
		}

		var certificateBytes []byte
		if signCert {
			certificateBytes, err = generateSignedCert(key, ip)
		} else {
			certificateBytes, err = generateSelfSignedCert(key, ip)
		}
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
func CheckConnectivity(ip string, port int, caCert []byte) {
	time.Sleep(1 * time.Second) // Initial delay before the first check
	ticker := jitter.NewTicker(time.Minute, 0.2)
	defer ticker.Stop()

	certPool, _ := x509.SystemCertPool()

	// AppendCertsFromPEM() takes care of dealing with an empty byte array, so we don't have to.
	ok := certPool.AppendCertsFromPEM(caCert)
	if !ok {
		log.Error("Failed to append CA certificate to system cert pool. Connectivity checks may fail.")
	}

	client := http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				RootCAs: certPool,
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
func ListenAndServeTLS(dataDir string, signCert bool, caCert []byte, publicIP string, listenPort int, handler http.Handler) error {
	cert, err := Certificate(dataDir, publicIP, signCert)
	if err != nil {
		return fmt.Errorf("failed to generate certificate: %w", err)
	}

	conf := &tls.Config{
		Certificates: []tls.Certificate{cert},
	}

	go CheckConnectivity(publicIP, listenPort, caCert)
	addr := fmt.Sprintf(":%d", listenPort)
	server := &http.Server{Addr: addr, Handler: handler, TLSConfig: conf}
	return server.ListenAndServeTLS("", "")
}
