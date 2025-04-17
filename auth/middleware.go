package auth

import (
	"context"
	"net/http"

	"github.com/charmbracelet/log"
	"github.com/golang-jwt/jwt/v5"
)

// ctxUserKey is the context key for storing the username from the JWT claims.
type ctxUserKey struct{}

// GetRequestUsername retrieves the username stored in the request context by the Middleware.
// It returns an empty string if the username is not found.
func GetRequestUsername(r *http.Request) string {
	if claims, ok := r.Context().Value(ctxUserKey{}).(string); ok {
		return claims
	}
	return ""
}

// AdminOnly is a middleware that restricts access to admin users only.
// It checks if the username retrieved by GetRequestUsername is "admin".
func AdminOnly(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if username := GetRequestUsername(r); username != "admin" {
			http.Error(w, "Forbidden", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// Middleware is an HTTP middleware that validates JWT tokens from the Authorization header or "token" query parameter.
// If the token is valid, it extracts the username (subject claim) and stores it in the request context.
// If the token is missing or invalid, it returns an Unauthorized error.
func Middleware(hmacSecret []byte, next http.Handler) http.Handler {
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
			return hmacSecret, nil
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
