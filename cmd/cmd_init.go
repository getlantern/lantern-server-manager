package main

import (
	"fmt"
	"github.com/charmbracelet/log"
	"github.com/sagernet/sing-box/option"
	"os"
	"os/exec"

	"github.com/getlantern/lantern-server-manager/common"
)

type InitCmd struct {
}

func InitializeConfigs() (*common.ServerConfig, *option.Options, error) {
	config, err := common.GenerateServerConfig(args.DataDir, args.APIPort)
	if err != nil {
		return nil, nil, err
	}
	singboxConfig, err := common.GenerateBasicSingBoxServerConfig(args.DataDir, args.VPNPort)
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

// attemptToOpenPorts uses firewall-cmd to open required ports.
// NO_FIREWALLD skips this. This is useful for local testing or within docker
var noFirewallD = os.Getenv("NO_FIREWALLD") != ""

func attemptToOpenPorts(config *common.ServerConfig, singBoxConfig *option.Options) {
	if noFirewallD {
		log.Infof("NO_FIREWALLD is set, not opening ports")
		return
	}
	// check if firewall-cmd exists
	if path, _ := exec.LookPath("firewall-cmd"); path == "" {
		log.Infof("firewall-cmd not found in $PATH. You may need to open the ports manually.")
		return
	}
	inboundOptions, err := common.GetShadowsocksInboundConfig(singBoxConfig)
	if err != nil {
		log.Fatal(err)
	}

	if err := exec.Command("firewall-cmd", "--add-port", fmt.Sprintf("%d/tcp", config.Port), "--permanent").Run(); err != nil {
		log.Errorf("failed to open port %d: %v", config.Port, err)
	} else {
		log.Infof("opened port %d", config.Port)
	}

	if err := exec.Command("firewall-cmd", "--add-port", fmt.Sprintf("%d/tcp", inboundOptions.ListenPort), "--permanent").Run(); err != nil {
		log.Errorf("failed to open port %d: %v", inboundOptions.ListenPort, err)
	} else {
		log.Infof("opened port %d", inboundOptions.ListenPort)
	}

	if err := exec.Command("firewall-cmd", "--reload").Run(); err != nil {
		log.Errorf("failed to reload firewall: %v", err)
	} else {
		log.Infof("reloaded firewall")
	}
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
