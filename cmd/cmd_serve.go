package main

import (
	_ "embed"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/charmbracelet/log"
	"github.com/sagernet/sing-box/option"

	"github.com/getlantern/lantern-server-manager/auth"
	"github.com/getlantern/lantern-server-manager/common"
)

//go:embed ca.cert
var defaultCACert []byte

// ServeCmd defines the structure for the 'serve' subcommand.
// It holds the loaded server and sing-box configurations.
type ServeCmd struct {
	serverConfig  *common.ServerConfig
	singboxConfig *option.Options
	caCert        []byte

	SignCertificate     bool   `arg:"--sign-certificate" help:"sign TLS certificate using Lantern API" default:"true"`
	CustomCACertificate string `arg:"--ca-cert" help:"custom CA certificate file" default:""`
}

// readConfigs loads the server and sing-box configurations from the data directory.
// If the server configuration doesn't exist, it initializes both configurations.
// It validates the loaded or initialized sing-box config and restarts the sing-box service.
func (c *ServeCmd) readConfigs() error {
	var err error
	if c.CustomCACertificate != "" {
		certData, err := os.ReadFile(c.CustomCACertificate)
		if err != nil {
			return fmt.Errorf("failed to read custom CA certificate: %w", err)
		}
		c.caCert = certData
	} else {
		c.caCert = defaultCACert
	}
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

// Run executes the 'serve' subcommand logic.
// It checks if sing-box is installed, reads configurations, prints the root token,
// attempts to open firewall ports, starts a background connectivity check,
// sets up HTTP API endpoints, and starts the HTTPS server.
func (c *ServeCmd) Run() error {
	if !common.CheckSingBoxInstalled() {
		return fmt.Errorf("sing-box not found in PATH")
	}
	if err := c.readConfigs(); err != nil {
		return err
	}

	printRootToken(c.serverConfig, c.singboxConfig)
	attemptToOpenPorts(c.serverConfig, c.singboxConfig)
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

	return auth.ListenAndServeTLS(args.DataDir, c.SignCertificate, c.caCert, c.serverConfig.ExternalIP, c.serverConfig.Port, srv)
}

// getConnectConfigHandler handles requests for generating sing-box client configurations.
// It uses the username from the request context (validated by middleware) to generate
// a tailored configuration including the necessary credentials.
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

// ShareLinkExpiration defines the validity duration for generated share links (access tokens).
const ShareLinkExpiration = 24 * time.Hour

// getShareLinkHandler handles requests to generate a temporary access token (share link) for a user.
// This endpoint is admin-only. It extracts the username from the URL path.
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

// revokeAccess handles requests to revoke access for a specific user.
// This endpoint is admin-only. It extracts the username from the URL path
// and calls common.RevokeUser to remove the user from the sing-box config.
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

// healthCheckHandler provides a simple health check endpoint.
// It returns a JSON response indicating the server is running.
func (c *ServeCmd) healthCheckHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(`{"status": "ok"}`))
}
