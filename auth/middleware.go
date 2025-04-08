package auth

import (
	"context"
	"net/http"

	"github.com/charmbracelet/log"
	"github.com/golang-jwt/jwt/v5"
)

type ctxUserKey struct{}

func GetRequestUsername(r *http.Request) string {
	if claims, ok := r.Context().Value(ctxUserKey{}).(string); ok {
		return claims
	}
	return ""
}

func AdminOnly(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if username := GetRequestUsername(r); username != "admin" {
			http.Error(w, "Forbidden", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

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
