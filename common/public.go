package common

import (
	"context"
	"errors"
	"io"
	"net"
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
		"https://ifconfig.io",
	}

	client := &http.Client{
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, network string, addr string) (net.Conn, error) {
				return (&net.Dialer{}).DialContext(ctx, "tcp4", addr)
			},
		},
	}
	// Try each host until one works
	for _, host := range hostsToTry {
		resp, err := client.Get(host)
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
