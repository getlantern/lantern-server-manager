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

func ReadSingBoxServerConfig(dataDir string) (*option.Options, error) {
	data, err := os.ReadFile(path.Join(dataDir, "sing-box-config.json"))
	if err != nil {
		return nil, err
	}
	globalCtx := box.Context(context.Background(), include.InboundRegistry(), include.OutboundRegistry(), include.EndpointRegistry(), include.DNSTransportRegistry())

	opt, err := singJson.UnmarshalExtendedContext[option.Options](globalCtx, data)
	if err != nil {
		return nil, err
	}
	return &opt, nil
}

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

func WriteSingBoxServerConfig(dataDir string, opt *option.Options) error {
	data, err := badjson.MarshallObjects(opt)
	if err != nil {
		return err
	}
	return os.WriteFile(path.Join(dataDir, "sing-box-config.json"), data, 0644)
}

func makeShadowsocksPassword() string {
	// generate a password. we are using chacha20-ietf-poly1305 so length can be anything
	passwordStr := password.MustGenerate(32, 10, 6, false, false)

	return base64.StdEncoding.EncodeToString([]byte(passwordStr))
}

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

func CheckSingBoxInstalled() bool {
	_, err := exec.LookPath("sing-box")
	return err == nil
}

func ValidateSingBoxConfig(dataDir string) error {
	singBoxPath, err := exec.LookPath("sing-box")
	if err != nil {
		return fmt.Errorf("sing-box not found in PATH: %w", err)
	}
	// check for non-zero exit code
	if err = exec.Command(singBoxPath, "check", "--config", path.Join(dataDir, "sing-box-config.json")).Run(); err != nil {
		return fmt.Errorf("failed to validate sing-box config: %w", err)
	}
	return nil
}

// RestartSingBox restarts the sing-box service. If NO_SYSTEMD is set, it will use pkill and run the command directly.
// this is useful for local testing without install of the service
var noSystemd = os.Getenv("NO_SYSTEMD") != ""

func RestartSingBox(dataDir string) error {
	if noSystemd {
		singBoxPath, _ := exec.LookPath("sing-box")
		// kill process
		_ = exec.Command("pkill", "-9", "sing-box").Run()
		// start process
		return exec.Command(singBoxPath, "run", "--config", path.Join(dataDir, "sing-box-config.json")).Start()
	} else {
		return exec.Command("systemctl", "restart", "sing-box").Run()
	}
}
