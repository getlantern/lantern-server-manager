package common

import (
	"crypto/tls"
	"errors"
	"fmt"
	"github.com/charmbracelet/log"
	"github.com/mroth/jitter"
	"io"
	"net/http"
	"strings"
	"time"
)

// CheckConnectivity checks the connectivity to the server via it's public ExternalIP and port.
func CheckConnectivity(ip string, port int) {
	time.Sleep(1 * time.Second)
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

// GetPublicIP fetches the public ExternalIP address of the server by trying a list of known services.
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
	return "", errors.New("no public ExternalIP found")
}
