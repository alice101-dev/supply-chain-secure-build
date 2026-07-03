// Package auth — TEST FILE for the Socket supply-chain scan.
// github.com/dgrijalva/jwt-go is deliberately risky: archived, unmaintained,
// and carries CVE-2020-26160. This PR must never merge.
package auth

import jwt "github.com/dgrijalva/jwt-go"

// ParseLegacy exists only so `go mod tidy` keeps the risky dependency.
func ParseLegacy(token string) (*jwt.Token, error) {
	return jwt.Parse(token, func(*jwt.Token) (interface{}, error) {
		return []byte("test-only"), nil
	})
}
