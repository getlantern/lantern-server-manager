package common

import (
	"context"
	"encoding/base64"
	"fmt"
	"math/rand/v2"
	"net/netip"
	"os"
	"os/exec"
	"path"

	"github.com/charmbracelet/log"
	box "github.com/sagernet/sing-box"
	"github.com/sagernet/sing-box/include"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common"
	singJson "github.com/sagernet/sing/common/json"
	"github.com/sagernet/sing/common/json/badjson"
	"github.com/sagernet/sing/common/json/badoption"
	"github.com/sethvargo/go-password/password"
)

// ReadSingBoxServerConfig reads and parses the sing-box server configuration
// from "sing-box-config.json" located in the specified data directory.
// It uses sing-box's internal JSON parsing capabilities.
func ReadSingBoxServerConfig(dataDir string) (*option.Options, error) {
	data, err := os.ReadFile(path.Join(dataDir, "sing-box-config.json"))
	if err != nil {
		return nil, err
	}
	globalCtx := box.Context(context.Background(), include.InboundRegistry(), include.OutboundRegistry(), include.EndpointRegistry())

	opt, err := singJson.UnmarshalExtendedContext[option.Options](globalCtx, data)
	if err != nil {
		return nil, err
	}
	return &opt, nil
}

// RevokeUser removes a user from the sing-box Shadowsocks inbound configuration.
// It reads the current config, finds the user by name in the first inbound's user list,
// removes them, writes the updated config back, and restarts the sing-box service.
func RevokeUser(dataDir, username string) error {
	singBoxServerConfig, err := ReadSingBoxServerConfig(dataDir)
	if err != nil {
		return err
	}
	if len(singBoxServerConfig.Inbounds) == 0 {
		return fmt.Errorf("no inbounds found, invalid config")
	}
	inboundOptions, ok := singBoxServerConfig.Inbounds[0].Options.(*option.ShadowsocksInboundOptions)
	if !ok {
		return fmt.Errorf("inbound is not shadowsocks")
	}
	for i, u := range inboundOptions.Users {
		if u.Name == username {
			inboundOptions.Users = append(inboundOptions.Users[:i], inboundOptions.Users[i+1:]...)
			break
		}
	}
	if err = WriteSingBoxServerConfig(dataDir, singBoxServerConfig); err != nil {
		return err
	}

	// restart singbox
	return RestartSingBox(dataDir)
}

// GetShadowsocksInboundConfig extracts the Shadowsocks inbound options from a given
// sing-box configuration. It assumes the first inbound defined in the config
// is the relevant Shadowsocks inbound.
func GetShadowsocksInboundConfig(singBoxServerConfig *option.Options) (*option.ShadowsocksInboundOptions, error) {
	if len(singBoxServerConfig.Inbounds) == 0 {
		return nil, fmt.Errorf("no inbounds found, invalid config")
	}
	inboundOptions, ok := singBoxServerConfig.Inbounds[0].Options.(*option.ShadowsocksInboundOptions)
	if !ok {
		return nil, fmt.Errorf("inbound is not shadowsocks")
	}
	return inboundOptions, nil
}

// GenerateSingBoxConnectConfig creates a sing-box client configuration JSON for a specific user.
// It reads the server's sing-box config, finds or creates the user's Shadowsocks credentials,
// constructs a client config pointing to the server's public IP and Shadowsocks port,
// and returns the marshalled JSON configuration. If the user doesn't exist, they are added
// to the server config, and sing-box is restarted.
func GenerateSingBoxConnectConfig(dataDir, publicIP, username string) ([]byte, error) {
	singBoxServerConfig, err := ReadSingBoxServerConfig(dataDir)
	if err != nil {
		return nil, err
	}
	inboundOptions, err := GetShadowsocksInboundConfig(singBoxServerConfig)
	if err != nil {
		return nil, err
	}
	var pw string
	if username == "admin" {
		pw = inboundOptions.Password
	} else {
		for _, u := range inboundOptions.Users {
			if u.Name == username {
				pw = u.Password
				break
			}
		}
	}
	if pw == "" {
		pw = makeShadowsocksPassword()
		inboundOptions.Users = append(inboundOptions.Users, option.ShadowsocksUser{
			Name:     username,
			Password: pw,
		})
		// user now found. add the user
		if err = WriteSingBoxServerConfig(dataDir, singBoxServerConfig); err != nil {
			return nil, err
		}
		// restart singbox
		if err = RestartSingBox(dataDir); err != nil {
			return nil, err
		}
	}
	opt := option.Options{
		Log: &option.LogOptions{
			Level:  "debug",
			Output: "stdout",
		},
		// the block below is only used for testing, when Lantern VPN imports the config
		// it should discard the Inbounds section and replace it with the one in the app (TUN)
		Inbounds: []option.Inbound{
			{
				Type: "socks5",
				Options: &option.SocksInboundOptions{
					ListenOptions: option.ListenOptions{
						ListenPort: 8888,
						Listen:     common.Ptr(badoption.Addr(netip.AddrFrom4([4]byte{127, 0, 0, 1}))),
					},
				},
			},
		},
		Outbounds: []option.Outbound{
			{
				Type: "shadowsocks",
				Tag:  "ss-outbound",
				Options: &option.ShadowsocksOutboundOptions{
					DialerOptions: option.DialerOptions{},
					ServerOptions: option.ServerOptions{
						Server:     publicIP,
						ServerPort: inboundOptions.ListenPort,
					},
					Method:   "chacha20-ietf-poly1305",
					Password: pw,
				},
			},
		},
	}
	return badjson.MarshallObjects(opt)
}

// WriteSingBoxServerConfig marshals the provided sing-box options into JSON
// and writes it to "sing-box-config.json" in the specified data directory.
func WriteSingBoxServerConfig(dataDir string, opt *option.Options) error {
	data, err := badjson.MarshallObjects(opt)
	if err != nil {
		return err
	}
	if err = os.WriteFile(path.Join(dataDir, "sing-box-config.json"), data, 0644); err != nil {
		return err
	}
	if !noSystemd {
		// in systemd mode, sing-box-extensions expects the config to be in /etc/sing-box-extensions/config.json
		// make sure that the path exists and copy the config
		if err = os.MkdirAll("/etc/sing-box-extensions", 0755); err != nil {
			return err
		}
		return os.WriteFile("/etc/sing-box-extensions/config.json", data, 0644)
	}
	// in non-systemd mode, we just write the config to the data directory
	return nil
}

// makeShadowsocksPassword generates a secure random password suitable for Shadowsocks
// (specifically for chacha20-ietf-poly1305, though the length is flexible).
// It returns the base64 encoded version of the generated password bytes.
func makeShadowsocksPassword() string {
	// generate a password. we are using chacha20-ietf-poly1305 so length can be anything
	passwordStr := password.MustGenerate(32, 10, 6, false, false)

	return base64.StdEncoding.EncodeToString([]byte(passwordStr))
}

// GenerateBasicSingBoxServerConfig creates a minimal initial sing-box server configuration.
// It sets up logging, a single Shadowsocks inbound listener (on a specified or random port)
// with a generated password, and writes the configuration to file.
func GenerateBasicSingBoxServerConfig(dataDir string, listenPort int) (*option.Options, error) {
	port := listenPort
	if port == 0 {
		// generate a number that is a valid non-privileged port
		port = rand.N(65535-1024) + 1024
	}
	pw := makeShadowsocksPassword()
	// generate basic shadowsocks config
	opt := option.Options{
		Log: &option.LogOptions{
			Level:  "debug",
			Output: "stdout",
		},
		Inbounds: []option.Inbound{
			{
				Type: "shadowsocks",
				Tag:  "ss-inbound",

				Options: &option.ShadowsocksInboundOptions{
					Method: "chacha20-ietf-poly1305",
					ListenOptions: option.ListenOptions{
						ListenPort: uint16(port),
						Listen:     common.Ptr(badoption.Addr(netip.AddrFrom4([4]byte{0, 0, 0, 0}))),
					},
					Password: pw,
				},
			},
		},
	}
	log.Infof("Writing intial vpn config to sing-box-config.json")
	return &opt, WriteSingBoxServerConfig(dataDir, &opt)
}

// CheckSingBoxInstalled checks if the 'sing-box' executable is available in the system's PATH.
func CheckSingBoxInstalled() bool {
	_, err := exec.LookPath("sing-box-extensions")
	return err == nil
}

// ValidateSingBoxConfig uses the 'sing-box check' command to validate the syntax
// of the configuration file located at "sing-box-config.json" in the data directory.
func ValidateSingBoxConfig(dataDir string) error {
	singBoxPath, err := exec.LookPath("sing-box-extensions")
	if err != nil {
		return fmt.Errorf("sing-box not found in PATH: %w", err)
	}
	// check for non-zero exit code
	if err = exec.Command(singBoxPath, "check", "--config", path.Join(dataDir, "sing-box-config.json")).Run(); err != nil {
		return fmt.Errorf("failed to validate sing-box config: %w", err)
	}
	return nil
}

// noSystemd controls whether systemd is used for service management.
// If the environment variable NO_SYSTEMD is set to any non-empty value,
// sing-box will be managed directly using pkill and running the command.
// Otherwise, it assumes systemd is available and uses `systemctl restart sing-box`.
var noSystemd = os.Getenv("NO_SYSTEMD") != ""

// RestartSingBox restarts the sing-box service.
// It either uses `systemctl restart sing-box` or, if noSystemd is true,
// kills any existing sing-box process and starts a new one directly using the
// configuration file in the data directory.
func RestartSingBox(dataDir string) error {
	if noSystemd {
		singBoxPath, _ := exec.LookPath("sing-box-extensions")
		// kill process
		_ = exec.Command("pkill", "-9", "sing-box-extensions").Run()
		// start process
		return exec.Command(singBoxPath, "run", "--config", path.Join(dataDir, "sing-box-config.json")).Start()
	} else {
		return exec.Command("systemctl", "restart", "sing-box-extensions").Run()
	}
}
