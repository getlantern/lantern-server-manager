package auth

import (
	"github.com/golang-jwt/jwt/v5"
	"time"
)

func GenerateAccessToken(hmacSecret []byte, username string, expiration time.Time) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub": username,
		"exp": expiration.Unix(), // Set the expiration time
	})

	// Sign and get the complete encoded token as a string using the secret
	return token.SignedString(hmacSecret)
}
