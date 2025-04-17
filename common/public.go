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

// CheckConnectivity periodically checks the health endpoint of the server using its public IP and port.
// It runs in a loop, making requests every minute with jitter.
// It uses an HTTP client configured to skip TLS verification, suitable for self-signed certificates.
// Errors during the check are logged.
func CheckConnectivity(ip string, port int) {
	time.Sleep(1 * time.Second) // Initial delay before the first check
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

// GetPublicIP attempts to determine the server's public IP address by querying several external services.
// It iterates through a predefined list of "what's my IP" services and returns the first successful result.
// If all attempts fail, it returns an error.
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
