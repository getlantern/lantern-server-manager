package common

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand/v2"
	"os"
	"os/exec"
	"path"
	"time"

	"github.com/charmbracelet/log"
	"github.com/sethvargo/go-password/password"

	"github.com/getlantern/lantern-server-manager/auth"
)

type ServerConfig struct {
	ExternalIP  string `json:"external_ip"`
	Port        int    `json:"port"`
	AccessToken string `json:"access_token"`
	HMACSecret  []byte `json:"hmac_secret"`
}

func (c *ServerConfig) GetNewServerURL() string {
	return fmt.Sprintf("lantern://new-private-server?ip=%s&port=%d&token=%s", c.ExternalIP, c.Port, c.AccessToken)
}

func (c *ServerConfig) GetQR() string {
	qrCodeOptions := []string{"ANSI", "ANSI256", "ASCII", "ASCIIi", "UTF8", "UTF8i", "ANSIUTF8", "ANSIUTF8i", "ANSI256UTF8"}
	text := "https://google.com/" // TODO: c.GetNewServerURL()
	qrCode := bytes.NewBufferString("")
	for _, qrCodeOption := range qrCodeOptions {
		cmd := exec.Command("qrencode", "-t", qrCodeOption, text+qrCodeOption)
		// collect output
		cmd.Stdout = qrCode
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			log.Errorf("Error generating QR code: %v", err)
			continue
		}
	}
	//qrterminal.GenerateHalfBlock(c.GetNewServerURL(), qrterminal.L, qrCode)

	return qrCode.String()
}

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

var AdminExpirationTime = time.Date(2900, 1, 1, 0, 0, 0, 0, time.UTC)

func GenerateServerConfig(dataDir string) (*ServerConfig, error) {
	publicIP, err := GetPublicIP()
	if err != nil {
		log.Error("Cannot detect your public ExternalIP, please get it from your host provider")
		publicIP = "0.0.0.0"
	}

	// generate a number that is a valid non-privileged port
	port := rand.N(65535-1024) + 1024
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
