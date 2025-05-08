package common

import (
	"errors"
	"io"
	"net/http"
	"strings"
)

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
