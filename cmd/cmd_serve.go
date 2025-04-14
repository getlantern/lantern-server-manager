package main

import (
	"fmt"
	"net/http"
	"time"

	"github.com/charmbracelet/log"
	"github.com/sagernet/sing-box/option"

	"github.com/getlantern/lantern-server-manager/auth"
	"github.com/getlantern/lantern-server-manager/common"
)

type ServeCmd struct {
	serverConfig  *common.ServerConfig
	singboxConfig *option.Options
}

func (c *ServeCmd) readConfigs() error {
	var err error
	c.serverConfig, err = common.ReadServerConfig(args.DataDir)
	if err != nil {
		// no config found. init
		c.serverConfig, c.singboxConfig, err = InitializeConfigs()
		if err != nil {
			return fmt.Errorf("failed to init server: %w", err)
		}
	} else {
		c.singboxConfig, err = common.ReadSingBoxServerConfig(args.DataDir)
		if err != nil {
			return fmt.Errorf("failed to read sing-box config: %w", err)
		}
	}
	if err = common.ValidateSingBoxConfig(args.DataDir); err != nil {
		return fmt.Errorf("failed to validate sing-box config: %w", err)
	}

	if err = common.RestartSingBox(args.DataDir); err != nil {
		return fmt.Errorf("failed to start sing-box: %w", err)
	}

	return nil
}

func (c *ServeCmd) Run() error {
	if !common.CheckSingBoxInstalled() {
		return fmt.Errorf("sing-box not found in PATH")
	}
	if err := c.readConfigs(); err != nil {
		return err
	}

	printRootToken(c.serverConfig, c.singboxConfig)
	attemptToOpenPorts(c.serverConfig, c.singboxConfig)
	go common.CheckConnectivity(c.serverConfig.ExternalIP, c.serverConfig.Port)
	srv := http.NewServeMux()
	srv.Handle("GET /api/v1/health", http.HandlerFunc(c.healthCheckHandler))
	srv.Handle("GET /api/v1/connect-config", auth.Middleware(c.serverConfig.HMACSecret, http.HandlerFunc(c.getConnectConfigHandler)))
	srv.Handle("GET /api/v1/share-link/{name}", auth.Middleware(c.serverConfig.HMACSecret, auth.AdminOnly(http.HandlerFunc(c.getShareLinkHandler))))
	srv.Handle("POST /api/v1/revoke/{name}", auth.Middleware(c.serverConfig.HMACSecret, auth.AdminOnly(http.HandlerFunc(c.revokeAccess))))
	srv.HandleFunc("/", func(w http.ResponseWriter, req *http.Request) {
		// The "/" pattern matches everything, so we need to check
		// that we're at the root here.
		if req.URL.Path != "/" {
			http.NotFound(w, req)
			return
		}
		fmt.Fprintf(w, "Welcome to Lantern Server Manager. In future, there will be UI here!")
	})

	return auth.SelfSignedListenAndServeTLS(args.DataDir, c.serverConfig.ExternalIP, fmt.Sprintf(":%d", c.serverConfig.Port), srv)
}

func (c *ServeCmd) getConnectConfigHandler(writer http.ResponseWriter, r *http.Request) {
	cfg, err := common.GenerateSingBoxConnectConfig(args.DataDir, c.serverConfig.ExternalIP, auth.GetRequestUsername(r))
	if err != nil {
		log.Errorf("failed to generate connect config: %v", err)
		http.Error(writer, "failed to generate connect config", http.StatusInternalServerError)
		return
	}
	writer.Header().Set("Content-Type", "application/json")
	_, _ = writer.Write(cfg)
}

const ShareLinkExpiration = 24 * time.Hour

func (c *ServeCmd) getShareLinkHandler(w http.ResponseWriter, r *http.Request) {
	username := r.PathValue("name")
	accessToken, err := auth.GenerateAccessToken(c.serverConfig.HMACSecret, username, time.Now().Add(ShareLinkExpiration))
	if err != nil {
		log.Errorf("failed to generate access token: %v", err)
		http.Error(w, "failed to generate access token", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(fmt.Sprintf(`{"token": "%s"}`, accessToken)))
}

func (c *ServeCmd) revokeAccess(w http.ResponseWriter, r *http.Request) {
	username := r.PathValue("name")
	if err := common.RevokeUser(args.DataDir, username); err != nil {
		log.Errorf("failed to revoke user: %v", err)
		http.Error(w, "failed to revoke user", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(fmt.Sprintf(`{"status": "ok"}`)))
}

func (c *ServeCmd) healthCheckHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(`{"status": "ok"}`))
}
