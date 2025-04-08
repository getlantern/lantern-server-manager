package common

import (
	"context"
	"encoding/base64"
	"fmt"
	"math/rand/v2"
	"net/netip"
	"os"
	"os/exec"

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

func ReadSingBoxServerConfig() (*option.Options, error) {
	data, err := os.ReadFile("sing-box-config.json")
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

func RevokeUser(username string) error {
	singBoxServerConfig, err := ReadSingBoxServerConfig()
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
	if err = WriteSingBoxServerConfig(singBoxServerConfig); err != nil {
		return err
	}

	// restart singbox
	return RestartSingBox()
}

func GenerateSingboxConnectConfig(publicIP, username string) ([]byte, error) {
	singBoxServerConfig, err := ReadSingBoxServerConfig()
	if err != nil {
		return nil, err
	}
	if len(singBoxServerConfig.Inbounds) == 0 {
		return nil, fmt.Errorf("no inbounds found, invalid config")
	}
	inboundOptions, ok := singBoxServerConfig.Inbounds[0].Options.(*option.ShadowsocksInboundOptions)
	if !ok {
		return nil, fmt.Errorf("inbound is not shadowsocks")
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
		if err = WriteSingBoxServerConfig(singBoxServerConfig); err != nil {
			return nil, err
		}
		// restart singbox
		if err = RestartSingBox(); err != nil {
			return nil, err
		}
	}
	opt := option.Options{
		Log: &option.LogOptions{
			Level:  "debug",
			Output: "stdout",
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
					Method:   "2022-blake3-aes-128-gcm",
					Password: pw,
				},
			},
		},
	}
	return badjson.MarshallObjects(opt)
}

func WriteSingBoxServerConfig(opt *option.Options) error {
	data, err := badjson.MarshallObjects(opt)
	if err != nil {
		return err
	}
	return os.WriteFile("sing-box-config.json", data, 0644)
}

func makeShadowsocksPassword() string {
	// generate a password. we are using 2022-blake3-aes-128-gcm so length must be 16
	passwordStr := password.MustGenerate(16, 10, 6, false, false)

	return base64.StdEncoding.EncodeToString([]byte(passwordStr))
}

func GenerateBasicSingboxServerConfig() error {
	// generate a number that is a valid non-privileged port
	port := rand.N(65535-1024) + 1024

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
					Method: "2022-blake3-aes-128-gcm",
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
	return WriteSingBoxServerConfig(&opt)
}

func CheckSingboxInstalled() bool {
	_, err := exec.LookPath("sing-box")
	return err == nil
}

func RestartSingBox() error {
	singBoxPath, _ := exec.LookPath("sing-box")
	// kill process
	_ = exec.Command("pkill", "-9", "sing-box").Run()
	// start process
	return exec.Command(singBoxPath, "run", "--config", "sing-box-config.json").Start()
}
