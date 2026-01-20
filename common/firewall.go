package common

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/charmbracelet/log"
)

// noFirewallD controls whether firewall rules are skipped.
// If the environment variable NO_FIREWALLD is set to any non-empty value,
// firewall modifications using firewall-cmd will be skipped.
// This is useful for local testing or running within containers like Docker.
var noFirewallD = os.Getenv("NO_FIREWALLD") != ""

// CloseFirewallPort attempts to close the specified port using firewall-cmd.
// If permanent is true, the rule will be removed permanently.
// If NO_FIREWALLD is set or firewall-cmd is not found, it logs the information and returns.
func CloseFirewallPort(port int) {
	if noFirewallD {
		log.Infof("NO_FIREWALLD is set, not opening ports")
		return
	}
	// check if firewall-cmd exists
	if path, _ := exec.LookPath("firewall-cmd"); path == "" {
		log.Infof("firewall-cmd not found in $PATH. You may need to open the ports manually.")
		return
	}

	if err := exec.Command("firewall-cmd", "--remove-port", fmt.Sprintf("%d/tcp", port), "--permanent").Run(); err != nil {
		log.Errorf("failed to close port %d: %v", port, err)
	} else {
		log.Infof("closed port %d", port)
	}
	if err := exec.Command("firewall-cmd", "--reload").Run(); err != nil {
		log.Errorf("failed to reload firewall: %v", err)
	} else {
		log.Infof("reloaded firewall")
	}
}

// OpenFirewallPort attempts to open the specified port using firewall-cmd.
// If permanent is true, the rule will be added permanently.
// If NO_FIREWALLD is set or firewall-cmd is not found, it logs the information and returns.
func OpenFirewallPort(port int) {
	if noFirewallD {
		log.Infof("NO_FIREWALLD is set, not opening ports")
		return
	}
	// check if firewall-cmd exists
	if path, _ := exec.LookPath("firewall-cmd"); path == "" {
		log.Infof("firewall-cmd not found in $PATH. You may need to open the ports manually.")
		return
	}
	if err := exec.Command("firewall-cmd", "--add-port", fmt.Sprintf("%d/tcp", port), "--permanent").Run(); err != nil {
		log.Errorf("failed to open port %d: %v", port, err)
	} else {
		log.Infof("opened port %d", port)
	}
	if err := exec.Command("firewall-cmd", "--reload").Run(); err != nil {
		log.Errorf("failed to reload firewall: %v", err)
	} else {
		log.Infof("reloaded firewall")
	}
}
