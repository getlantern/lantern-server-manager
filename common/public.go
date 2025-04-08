package common

import (
	"errors"
	"fmt"
	"github.com/charmbracelet/log"
	"io"
	"net/http"
	"strings"
	"time"
)

func CheckConnectivity(ip string, port int) {
	time.Sleep(1 * time.Second)
	_, err := http.Get(fmt.Sprintf("http://%s:%d/api/v1/health", ip, port))
	if err != nil {
		log.Errorf("Connectivity check failed. Please check the configuration: %v", err)
	}
}

func GetPublicIP() (string, error) {
	hostsToTry := []string{
		"https://icanhazip.com/",
		"https://ipinfo.io/ip",
		"https://domains.google.com/checkip",
		"https://ifconfig.io",
	}
	// Try each host until one works
	for _, host := range hostsToTry {
		resp, err := http.Get(host)
		if err != nil {
			continue
		}
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			_ = resp.Body.Close()
			continue
		}
		_ = resp.Body.Close()
		return strings.TrimSpace(string(body)), nil
	}
	return "", errors.New("no public IP found")
}
