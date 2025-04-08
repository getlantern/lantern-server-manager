package main

import (
	"github.com/getlantern/lantern-server-manager/common"
	"io"
	"net/http"
	"strings"

	"github.com/charmbracelet/log"
)

type InitCmd struct {
}

func getPublicIP() (string, error) {
	resp, err := http.Get("https://ifconfig.io")
	if err != nil {
		return "", err
	}
	defer func(Body io.ReadCloser) {
		_ = Body.Close()
	}(resp.Body)
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(body)), nil
}

func InitializeConfigs() (*common.ServerConfig, error) {
	config, err := common.GenerateServerConfig()
	if err != nil {
		return nil, err
	}
	if err = common.GenerateBasicSingboxServerConfig(); err != nil {
		return nil, err
	}
	return config, nil
}

func (c InitCmd) Run() error {
	if config, err := InitializeConfigs(); err != nil {
		return err
	} else {
		printRootToken(config)
	}

	return nil
}

func printRootToken(config *common.ServerConfig) {
	publicIP, err := getPublicIP()
	if err != nil {
		log.Error("Cannot detect your public IP, please get if from your host provider")
		publicIP = "0.0.0.0"
	}
	log.Infof("Paste this link into Lantern VPN app:\n%s", config.GetNewServerURL(publicIP))
	log.Printf("Or scan this QR code in Lantern VPN app:\n%s", config.GetQR(publicIP))
}
