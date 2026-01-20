package auth

import (
	"bytes"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"net/http"
	"os"
	"path"
	"sync/atomic"
	"time"

	"github.com/charmbracelet/log"
	"github.com/getlantern/lantern-server-manager/common"
	"github.com/go-acme/lego/v4/certcrypto"
	"github.com/go-acme/lego/v4/certificate"
	"github.com/go-acme/lego/v4/challenge/http01"
	"github.com/go-acme/lego/v4/lego"
	"github.com/go-acme/lego/v4/registration"
	"github.com/mroth/jitter"
)

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

var cert atomic.Value // stores *tls.Certificate

// legoUser implements the registration.User interface for ACME registration
type legoUser struct {
	Email        string                 `json:"email"`
	Registration *registration.Resource `json:"registration"`
	key          crypto.PrivateKey
}

func (u *legoUser) GetEmail() string                        { return u.Email }
func (u *legoUser) GetRegistration() *registration.Resource { return u.Registration }
func (u *legoUser) GetPrivateKey() crypto.PrivateKey        { return u.key }

func loadCert(dataDir, certPEMFile, keyPEMFile string, publicIP string) (*tls.Certificate, error) {
	// If custom cert/key files are provided, use those directly
	if certPEMFile != "" && keyPEMFile != "" {
		log.Debug("Loading custom TLS certificate. Skipping ACME", "cert", certPEMFile, "key", keyPEMFile)
		certPEM, err := os.ReadFile(certPEMFile)
		if err != nil {
			return nil, err
		}
		keyPEM, err := os.ReadFile(keyPEMFile)
		if err != nil {
			return nil, err
		}
		c, err := tls.X509KeyPair(certPEM, keyPEM)
		return &c, err
	}

	// Try to load existing ACME certificate
	acmeCertPath := path.Join(dataDir, "acme_cert.pem")
	acmeKeyPath := path.Join(dataDir, "acme_key.pem")
	acmeAccountPath := path.Join(dataDir, "acme_account.json")
	accountKeyPath := path.Join(dataDir, "acme_account_key.pem")

	// Check if we have a valid existing certificate
	if certPEM, err := os.ReadFile(acmeCertPath); err == nil {
		if keyPEM, err := os.ReadFile(acmeKeyPath); err == nil {
			c, err := tls.X509KeyPair(certPEM, keyPEM)
			if err == nil {
				// Check if certificate is still valid (not expired)
				if c.Leaf == nil {
					c.Leaf, _ = x509.ParseCertificate(c.Certificate[0])
				}
				if c.Leaf != nil && time.Now().Before(c.Leaf.NotAfter.Add(-24*time.Hour)) {
					log.Debug("Using existing ACME certificate")
					return &c, nil
				}
				log.Debug("ACME certificate expired or expiring soon, renewing...")
			}
		}
	}

	// 1. Create/load account private key
	var accountKey crypto.PrivateKey
	if keyPEM, err := os.ReadFile(accountKeyPath); err == nil {
		accountKey, err = certcrypto.ParsePEMPrivateKey(keyPEM)
		if err != nil {
			return nil, fmt.Errorf("failed to parse account key: %w", err)
		}
		log.Debug("Loaded existing ACME account key")
	} else {
		log.Debug("Generating new ACME account key")
		key, err := rsa.GenerateKey(rand.Reader, 2048)
		if err != nil {
			return nil, fmt.Errorf("failed to generate account key: %w", err)
		}
		accountKey = key

		keyPEMBuf := new(bytes.Buffer)
		err = pem.Encode(keyPEMBuf, &pem.Block{
			Type:  "RSA PRIVATE KEY",
			Bytes: x509.MarshalPKCS1PrivateKey(key),
		})
		if err != nil {
			return nil, err
		}
		if err := os.WriteFile(accountKeyPath, keyPEMBuf.Bytes(), 0600); err != nil {
			return nil, fmt.Errorf("failed to save account key: %w", err)
		}
	}

	// 2. Create/load user registration
	user := &legoUser{
		Email: "admin@thisbox.org",
		key:   accountKey,
	}

	if accountData, err := os.ReadFile(acmeAccountPath); err == nil {
		if err := json.Unmarshal(accountData, user); err != nil {
			log.Warn("Failed to parse account file, will re-register", "error", err)
		}
	}

	// 3. Setup lego client
	config := lego.NewConfig(user)
	config.CADirURL = lego.LEDirectoryProduction
	config.Certificate.KeyType = certcrypto.RSA2048
	config.Certificate.DisableCommonName = true

	client, err := lego.NewClient(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create ACME client: %w", err)
	}

	common.OpenFirewallPort(80)
	defer common.CloseFirewallPort(80)
	// Use HTTP-01 challenge on port 80
	err = client.Challenge.SetHTTP01Provider(http01.NewProviderServer("", "80"))
	if err != nil {
		return nil, fmt.Errorf("failed to set HTTP challenge provider: %w", err)
	}

	// 4. Register if needed
	if user.Registration == nil {
		log.Debug("Registering new ACME account")
		reg, err := client.Registration.Register(registration.RegisterOptions{
			TermsOfServiceAgreed: true,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to register ACME account: %w", err)
		}
		user.Registration = reg

		accountData, err := json.MarshalIndent(user, "", "  ")
		if err != nil {
			return nil, err
		}
		if err := os.WriteFile(acmeAccountPath, accountData, 0600); err != nil {
			return nil, fmt.Errorf("failed to save account: %w", err)
		}
	}

	// 5. Obtain certificate
	log.Debug("Obtaining ACME certificate", "domain", publicIP)
	request := certificate.ObtainRequest{
		Domains: []string{publicIP},
		Bundle:  true,
		Profile: "shortlived",
	}

	certificates, err := client.Certificate.Obtain(request)
	if err != nil {
		return nil, fmt.Errorf("failed to obtain certificate: %w", err)
	}

	// 6. Save certificate and key
	if err := os.WriteFile(acmeCertPath, certificates.Certificate, 0644); err != nil {
		return nil, fmt.Errorf("failed to save certificate: %w", err)
	}
	if err := os.WriteFile(acmeKeyPath, certificates.PrivateKey, 0600); err != nil {
		return nil, fmt.Errorf("failed to save private key: %w", err)
	}

	log.Info("ACME certificate obtained successfully", "domain", publicIP)

	c, err := tls.X509KeyPair(certificates.Certificate, certificates.PrivateKey)
	if err != nil {
		return nil, err
	}
	return &c, nil
}

func ListenAndServeTLS(dataDir, certPEM, keyPEM string, publicIP string, listenPort int, handler http.Handler) error {
	c, err := loadCert(dataDir, certPEM, keyPEM, publicIP)
	if err != nil {
		log.Fatal(err)
	}
	cert.Store(c)

	conf := &tls.Config{
		GetCertificate: func(chi *tls.ClientHelloInfo) (*tls.Certificate, error) {
			return cert.Load().(*tls.Certificate), nil
		},
		MinVersion: tls.VersionTLS12,
	}

	go CheckConnectivity(publicIP, listenPort)
	go keepCertificateFresh(dataDir, certPEM, keyPEM, publicIP)
	addr := fmt.Sprintf(":%d", listenPort)
	server := &http.Server{Addr: addr, Handler: handler, TLSConfig: conf}
	return server.ListenAndServeTLS("", "")
}

func keepCertificateFresh(dataDir, certPEM, keyPEM string, publicIP string) {
	ticker := time.NewTicker(24 * time.Hour)
	defer ticker.Stop()
	for {
		<-ticker.C
		c, err := loadCert(dataDir, certPEM, keyPEM, publicIP)
		if err != nil {
			log.Error("Failed to renew certificate", "error", err)
			continue
		}
		cert.Store(c)
		log.Info("ACME certificate renewed successfully", "domain", publicIP)
	}
}
