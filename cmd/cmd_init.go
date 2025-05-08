package main

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/charmbracelet/log"
	"github.com/sagernet/sing-box/option"

	"github.com/getlantern/lantern-server-manager/common"
)

// InitCmd defines the structure for the 'init' subcommand.
// It currently doesn't hold any command-specific arguments but serves as the handler target.
type InitCmd struct {
}

// InitializeConfigs generates the initial server and sing-box configurations.
// It uses the global 'args' variable to access the data directory and port settings.
// It returns the generated ServerConfig, sing-box Options, and any error encountered.
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

// Run executes the 'init' subcommand logic.
// It calls InitializeConfigs to generate the necessary configuration files
// and then prints the root access token information using printRootToken.
func (c InitCmd) Run() error {
	if config, singboxConfig, err := InitializeConfigs(); err != nil {
		return err
	} else {
		printRootToken(config, singboxConfig)
	}

	return nil
}

// noFirewallD controls whether firewall rules are skipped.
// If the environment variable NO_FIREWALLD is set to any non-empty value,
// firewall modifications using firewall-cmd will be skipped.
// This is useful for local testing or running within containers like Docker.
var noFirewallD = os.Getenv("NO_FIREWALLD") != ""

// attemptToOpenPorts tries to open the necessary ports using firewall-cmd.
// It opens the API port defined in the ServerConfig and the VPN port
// defined in the sing-box configuration's Shadowsocks inbound options.
// It skips execution if noFirewallD is true or if firewall-cmd is not found.
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

// printRootToken logs information about the server setup, including required open ports,
// the Lantern VPN connection URL, and a QR code representation of the URL.
func printRootToken(config *common.ServerConfig, singBoxConfig *option.Options) {
	inboundOptions, err := common.GetShadowsocksInboundConfig(singBoxConfig)
	if err != nil {
		log.Fatal(err)
	}

	log.Infof("Make sure that the following ports are open: %d, %d", config.Port, inboundOptions.ListenPort)
	log.Infof("Paste this link into Lantern VPN app:\n%s", config.GetNewServerURL())
	log.Printf("Or scan this QR code in Lantern VPN app:\n%s", config.GetQR())
}
