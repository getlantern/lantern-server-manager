package main

import (
	"github.com/charmbracelet/log"
	"github.com/sagernet/sing-box/option"

	"github.com/getlantern/lantern-server-manager/common"
)

type InitCmd struct {
}

func InitializeConfigs() (*common.ServerConfig, *option.Options, error) {
	config, err := common.GenerateServerConfig(args.DataDir)
	if err != nil {
		return nil, nil, err
	}
	singboxConfig, err := common.GenerateBasicSingBoxServerConfig(args.DataDir)
	if err != nil {
		return nil, nil, err
	}
	return config, singboxConfig, nil
}

func (c InitCmd) Run() error {
	if config, singboxConfig, err := InitializeConfigs(); err != nil {
		return err
	} else {
		printRootToken(config, singboxConfig)
	}

	return nil
}

func printRootToken(config *common.ServerConfig, singBoxConfig *option.Options) {
	inboundOptions, err := common.GetShadowsocksInboundConfig(singBoxConfig)
	if err != nil {
		log.Fatal(err)
	}

	log.Infof("Make sure that the following ports are open: %d, %d", config.Port, inboundOptions.ListenPort)
	log.Infof("Paste this link into Lantern VPN app:\n%s", config.GetNewServerURL())
	log.Printf("Or scan this QR code in Lantern VPN app:\n%s", config.GetQR())
}
