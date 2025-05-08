package common

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/mdp/qrterminal/v3"
	"math/rand/v2"
	"os"
	"path"
	"time"

	"github.com/charmbracelet/log"
	"github.com/sethvargo/go-password/password"

	"github.com/getlantern/lantern-server-manager/auth"
)

// ServerConfig holds the core configuration for the Lantern Server Manager API server.
type ServerConfig struct {
	// ExternalIP is the publicly accessible IP address of the server.
	ExternalIP string `json:"external_ip"`
	// Port is the port number the API server listens on.
	Port int `json:"port"`
	// AccessToken is the initial administrative access token.
	AccessToken string `json:"access_token"`
	// HMACSecret is the secret key used for signing and verifying JWT tokens.
	HMACSecret []byte `json:"hmac_secret"`
}

// GetNewServerURL generates the URL used by the Lantern VPN app to configure a new private server.
// It includes the server's external IP, API port, and the admin access token.
func (c *ServerConfig) GetNewServerURL() string {
	return fmt.Sprintf("lantern://new-private-server?ip=%s&port=%d&token=%s", c.ExternalIP, c.Port, c.AccessToken)
}

// GetQR generates a string representation of a QR code for the server URL.
// This QR code can be scanned by the Lantern VPN app.
func (c *ServerConfig) GetQR() string {
	qrCode := bytes.NewBufferString("")
	qrterminal.GenerateHalfBlock(c.GetNewServerURL(), qrterminal.L, qrCode)

	return qrCode.String()
}

// ReadServerConfig reads the server configuration from the "server.json" file
// located in the specified data directory. It unmarshalls the JSON data into a
// ServerConfig struct and performs basic validation.
func ReadServerConfig(dataDir string) (*ServerConfig, error) {
	data, err := os.ReadFile(path.Join(dataDir, "server.json"))
	if err != nil {
		return nil, err
	}
	var config ServerConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, err
	}
	if config.ExternalIP == "" {
		return nil, errors.New("server ip is required")
	}
	if config.Port == 0 {
		return nil, fmt.Errorf("port not set")
	}
	if config.AccessToken == "" {
		return nil, fmt.Errorf("access token not set")
	}
	if len(config.HMACSecret) == 0 {
		return nil, fmt.Errorf("hmac secret not set")
	}
	return &config, nil
}

// AdminExpirationTime defines a far-future expiration date for the initial admin token.
var AdminExpirationTime = time.Date(2900, 1, 1, 0, 0, 0, 0, time.UTC)

// GenerateServerConfig creates a new initial server configuration.
// It attempts to detect the public IP, generates a random API port if not provided,
// creates a strong HMAC secret, generates an initial admin access token with a
// very long expiration time, and writes the configuration to "server.json"
// in the specified data directory.
func GenerateServerConfig(dataDir string, listenPort int) (*ServerConfig, error) {
	publicIP, err := GetPublicIP()
	if err != nil {
		log.Error("Cannot detect your public ExternalIP, please get it from your host provider")
		publicIP = "0.0.0.0"
	}

	port := listenPort
	if port == 0 {
		// generate a number that is a valid non-privileged port
		port = rand.N(65535-1024) + 1024
	}
	// generate hmac secret
	hmacSecret := password.MustGenerate(32, 10, 10, false, false)
	// generate an access token
	accessToken, err := auth.GenerateAccessToken([]byte(hmacSecret), "admin", AdminExpirationTime)
	if err != nil {
		return nil, err
	}
	conf := &ServerConfig{
		ExternalIP:  publicIP,
		Port:        port,
		AccessToken: accessToken,
		HMACSecret:  []byte(hmacSecret),
	}
	// write the config to a file
	data, err := json.Marshal(conf)
	if err != nil {
		return nil, err
	}
	log.Infof("Writing intial config to server.json")
	return conf, os.WriteFile(path.Join(dataDir, "server.json"), data, 0600)
}
