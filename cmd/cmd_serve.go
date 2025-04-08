package main

import (
	"fmt"
	"net/http"
	"time"

	"github.com/charmbracelet/log"

	"github.com/getlantern/lantern-server-manager/auth"
	"github.com/getlantern/lantern-server-manager/common"
)

type ServeCmd struct {
	serverConfig *common.ServerConfig
}

func (c ServeCmd) Run() error {
	if !common.CheckSingboxInstalled() {
		return fmt.Errorf("sing-box not found in PATH")
	}
	var err error
	c.serverConfig, err = common.ReadServerConfig()
	if err != nil {
		// no config found. init
		c.serverConfig, err = InitializeConfigs()
		if err != nil {
			return fmt.Errorf("failed to init server: %w", err)
		}
	}

	if err := common.RestartSingBox(); err != nil {
		return fmt.Errorf("failed to start sing-box: %w", err)
	}

	printRootToken(c.serverConfig)

	srv := http.NewServeMux()
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

	return http.ListenAndServe(fmt.Sprintf(":%d", c.serverConfig.Port), srv)
}

func (c ServeCmd) getConnectConfigHandler(writer http.ResponseWriter, r *http.Request) {
	publicIP, err := getPublicIP()
	if err != nil {
		log.Errorf("Cannot detect server public ip: %v", err)
		http.Error(writer, "Cannot detect server public ip", http.StatusInternalServerError)
		return
	}

	cfg, err := common.GenerateSingboxConnectConfig(publicIP, auth.GetRequestUsername(r))
	if err != nil {
		log.Errorf("failed to generate connect config: %v", err)
		http.Error(writer, "failed to generate connect config", http.StatusInternalServerError)
		return
	}
	writer.Header().Set("Content-Type", "application/json")
	_, _ = writer.Write(cfg)
}

const ShareLinkExpiration = 24 * time.Hour

func (c ServeCmd) getShareLinkHandler(w http.ResponseWriter, r *http.Request) {
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

func (c ServeCmd) revokeAccess(w http.ResponseWriter, r *http.Request) {
	username := r.PathValue("name")
	if err := common.RevokeUser(username); err != nil {
		log.Errorf("failed to revoke user: %v", err)
		http.Error(w, "failed to revoke user", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(fmt.Sprintf(`{"status": "ok"}`)))
}
