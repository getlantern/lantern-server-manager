package auth

import (
	"github.com/golang-jwt/jwt/v5"
	"time"
)

// GenerateAccessToken creates a new JWT access token signed with the HS256 algorithm.
// It includes the username as the subject ("sub") claim and sets the expiration time ("exp").
// The token is signed using the provided hmacSecret.
func GenerateAccessToken(hmacSecret []byte, username string, expiration time.Time) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub": username,
		"exp": expiration.Unix(), // Set the expiration time
	})

	// Sign and get the complete encoded token as a string using the secret
	return token.SignedString(hmacSecret)
}
