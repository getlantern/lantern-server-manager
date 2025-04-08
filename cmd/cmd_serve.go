package main

import (
	"context"
	"fmt"
	"net/http"
	"os/exec"

	"github.com/charmbracelet/log"
	"github.com/golang-jwt/jwt/v5"
)

var singBoxPath string

type ServeCmd struct {
}

func restartSingBox() error {
	// kill process
	_ = exec.Command("pkill", "-9", "sing-box").Run()
	// start process
	return exec.Command(singBoxPath, "run", "--config", "sing-box-config.json").Start()
}

type ctxUserKey struct{}

func authMiddleware(hmacSecret string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check for the presence of the Authorization header
		authHeader := r.Header.Get("Authorization")
		var tokenStr string
		if authHeader == "" {
			// try getting from query
			tokenStr = r.URL.Query().Get("token")
		} else {
			// Split the header into "Bearer" and the token
			tokenStr = authHeader[len("Bearer "):]
		}

		if tokenStr == "" {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		// Check if the token is valid
		token, err := jwt.Parse(tokenStr, func(token *jwt.Token) (interface{}, error) {
			return []byte(hmacSecret), nil
		}, jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}))
		if err != nil {
			log.Errorf("Error parsing token: %v", err)
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		if claims, err := token.Claims.GetSubject(); err != nil {
			log.Errorf("Error parsing token: %v", err)
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		} else {
			// Store the claims in the request context
			ctx := context.WithValue(r.Context(), ctxUserKey{}, claims)
			next.ServeHTTP(w, r.WithContext(ctx))
		}
	})
}

func (c ServeCmd) Run() error {
	var err error
	singBoxPath, err = exec.LookPath("sing-box")
	if err != nil {
		return fmt.Errorf("sing-box not found: %w", err)
	}

	serverConfig, err := readServerConfig()
	if err != nil {
		// no config found. init
		cmd := InitCmd{}
		if err = cmd.Run(); err != nil {
			return fmt.Errorf("failed to init server: %w", err)
		}
		serverConfig, err = readServerConfig()
		if err != nil {
			return fmt.Errorf("failed to read server config: %w", err)
		}
	} else {
		printRootToken(serverConfig)
	}

	if err := restartSingBox(); err != nil {
		return fmt.Errorf("failed to start sing-box: %w", err)
	}

	srv := http.NewServeMux()
	srv.Handle("GET /api/v1/connect-config", authMiddleware(serverConfig.HMACSecret, http.HandlerFunc(c.getConnectConfigHandler)))
	srv.Handle("GET /api/v1/share-link", authMiddleware(serverConfig.HMACSecret, http.HandlerFunc(c.getShareLinkHandler)))
	srv.Handle("POST /api/v1/process-share-link", authMiddleware(serverConfig.HMACSecret, http.HandlerFunc(c.processShareLinkHandler)))
	srv.HandleFunc("/", func(w http.ResponseWriter, req *http.Request) {
		// The "/" pattern matches everything, so we need to check
		// that we're at the root here.
		if req.URL.Path != "/" {
			http.NotFound(w, req)
			return
		}
		fmt.Fprintf(w, "Welcome to Lantern Server Manager. In future, there will be UI here!")
	})

	return http.ListenAndServe(fmt.Sprintf(":%d", serverConfig.Port), srv)
}

func (c ServeCmd) getConnectConfigHandler(writer http.ResponseWriter, r *http.Request) {
	publicIP, err := getPublicIP()
	if err != nil {
		log.Errorf("Cannot detect server public ip: %v", err)
		http.Error(writer, "Cannot detect server public ip", http.StatusInternalServerError)
		return
	}

	cfg, err := generateSingboxConnectConfig(publicIP, c.getRequestUsername(r))
	if err != nil {
		log.Errorf("failed to generate connect config: %v", err)
		http.Error(writer, "failed to generate connect config", http.StatusInternalServerError)
		return
	}
	writer.Header().Set("Content-Type", "application/json")
	_, _ = writer.Write(cfg)
}

func (c ServeCmd) processShareLinkHandler(w http.ResponseWriter, r *http.Request) {

}

func (c ServeCmd) getShareLinkHandler(w http.ResponseWriter, r *http.Request) {

}

func (c ServeCmd) getRequestUsername(r *http.Request) string {
	if claims, ok := r.Context().Value(ctxUserKey{}).(string); ok {
		return claims
	}
	return ""

}
