package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/mdp/qrterminal/v3"
	box "github.com/sagernet/sing-box"
	"github.com/sagernet/sing-box/include"
	"io"
	"math/rand/v2"
	"net/http"
	"net/netip"
	"os"
	"strings"

	"github.com/charmbracelet/log"
	"github.com/golang-jwt/jwt/v5"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common"
	singJson "github.com/sagernet/sing/common/json"
	"github.com/sagernet/sing/common/json/badjson"
	"github.com/sagernet/sing/common/json/badoption"
)

type InitCmd struct {
}

func GenerateAccessToken(hmacSecret []byte) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub": "admin",
		// no expiration
	})

	// Sign and get the complete encoded token as a string using the secret
	return token.SignedString(hmacSecret)
}

func PasswordGenerator(passwordLength int) string {
	lowerCase := "abcdefghijklmnopqrstuvwxyz" // lowercase
	upperCase := "ABCDEFGHIJKLMNOPQRSTUVWXYZ" // uppercase
	Numbers := "0123456789"                   // numbers
	specialChar := "!@#$%^&*()_-+={}[/?]"     // specialchar

	// variable for storing password
	password := ""

	for n := 0; n < passwordLength; n++ {
		// NOW RANDOM CHARACTER
		randNum := rand.N(4)

		switch randNum {
		case 0:
			randCharNum := rand.N(len(lowerCase))
			password += string(lowerCase[randCharNum])
		case 1:
			randCharNum := rand.N(len(upperCase))
			password += string(upperCase[randCharNum])
		case 2:
			randCharNum := rand.N(len(Numbers))
			password += string(Numbers[randCharNum])
		case 3:
			randCharNum := rand.N(len(specialChar))
			password += string(specialChar[randCharNum])
		}
	}

	return password
}

type ServerConfig struct {
	Port        int    `json:"port"`
	AccessToken string `json:"access_token"`
	HMACSecret  string `json:"hmac_secret"`
}

func (c *ServerConfig) GetNewServerURL(publicIP string) string {
	return fmt.Sprintf("lantern://new-private-server?ip=%s&port=%d&token=%s", publicIP, c.Port, c.AccessToken)
}
func (c *ServerConfig) GetQR(publicIP string) string {
	qrCode := bytes.NewBufferString("")
	qrterminal.Generate(c.GetNewServerURL(publicIP), qrterminal.L, qrCode)

	return qrCode.String()
}
func readServerConfig() (*ServerConfig, error) {
	data, err := os.ReadFile("server.json")
	if err != nil {
		return nil, err
	}
	var config ServerConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, err
	}
	if config.Port == 0 {
		return nil, fmt.Errorf("port not set")
	}
	if config.AccessToken == "" {
		return nil, fmt.Errorf("access token not set")
	}
	if config.HMACSecret == "" {
		return nil, fmt.Errorf("hmac secret not set")
	}
	return &config, nil
}

func generateServerConfig() (*ServerConfig, error) {
	// generate a number that is a valid non-privileged port
	port := rand.N(65535-1024) + 1024
	// generate hmac secret
	hmacSecret := PasswordGenerator(32)
	// generate an access token
	accessToken, err := GenerateAccessToken([]byte(hmacSecret))
	if err != nil {
		return nil, err
	}
	conf := &ServerConfig{
		Port:        port,
		AccessToken: accessToken,
		HMACSecret:  hmacSecret,
	}
	// write the config to a file
	data, err := json.Marshal(conf)
	if err != nil {
		return nil, err
	}
	log.Infof("Writing intial config to server.json")
	return conf, os.WriteFile("server.json", data, 0600)
}

func readSingBoxServerConfig() (*option.Options, error) {
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

func generateSingboxConnectConfig(publicIP, username string) ([]byte, error) {
	singBoxServerConfig, err := readSingBoxServerConfig()
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
	var password string
	if username == "admin" {
		password = inboundOptions.Password
	} else {
		for _, u := range inboundOptions.Users {
			if u.Name == username {
				password = u.Password
				break
			}
		}
	}
	if password == "" {
		return nil, fmt.Errorf("user not found")
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
					Password: password,
				},
			},
		},
	}
	return badjson.MarshallObjects(opt)
}

func generateBasicSingboxServerConfig() error {
	// generate a number that is a valid non-privileged port
	port := rand.N(65535-1024) + 1024
	// generate a password. we are using 2022-blake3-aes-128-gcm so length must be 16
	password := base64.StdEncoding.EncodeToString([]byte(PasswordGenerator(16)))
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
					Password: password,
				},
			},
		},
	}
	data, err := badjson.MarshallObjects(opt)
	if err != nil {
		return err
	}
	log.Infof("Writing intial vpn config to sing-box-config.json")
	return os.WriteFile("sing-box-config.json", data, 0644)
}

func getPublicIP() (string, error) {
	resp, err := http.Get("https://ifconfig.io")
	if err != nil {
		return "", err
	}
	defer func(Body io.ReadCloser) {
		_ = Body.Close()
	}(resp.Body)
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(body)), nil
}

func (c InitCmd) Run() error {
	var err error
	var config *ServerConfig
	if config, err = generateServerConfig(); err != nil {
		return err
	}
	if err = generateBasicSingboxServerConfig(); err != nil {
		return err
	}

	printRootToken(config)

	return nil
}

func printRootToken(config *ServerConfig) {
	publicIP, err := getPublicIP()
	if err != nil {
		log.Error("Cannot detect your public IP, please get if from your host provider")
		publicIP = "0.0.0.0"
	}
	log.Infof("Paste this link into Lantern VPN app:\n%s", config.GetNewServerURL(publicIP))
	log.Printf("Or scan this QR code in Lantern VPN app:\n%s", config.GetQR(publicIP))
}
