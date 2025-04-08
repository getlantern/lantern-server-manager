package common

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math/rand/v2"
	"os"
	"time"

	"github.com/charmbracelet/log"
	"github.com/mdp/qrterminal/v3"
	"github.com/sethvargo/go-password/password"

	"github.com/getlantern/lantern-server-manager/auth"
)

type ServerConfig struct {
	Port        int    `json:"port"`
	AccessToken string `json:"access_token"`
	HMACSecret  []byte `json:"hmac_secret"`
}

func (c *ServerConfig) GetNewServerURL(publicIP string) string {
	return fmt.Sprintf("lantern://new-private-server?ip=%s&port=%d&token=%s", publicIP, c.Port, c.AccessToken)
}
func (c *ServerConfig) GetQR(publicIP string) string {
	qrCode := bytes.NewBufferString("")
	qrterminal.Generate(c.GetNewServerURL(publicIP), qrterminal.L, qrCode)

	return qrCode.String()
}
func ReadServerConfig() (*ServerConfig, error) {
	data, err := os.ReadFile("server.json")
	if err != nil {
		return nil, err
	}
	var config ServerConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, err
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

func GenerateServerConfig() (*ServerConfig, error) {
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
	return conf, os.WriteFile("server.json", data, 0600)
}
