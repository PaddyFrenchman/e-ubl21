// This file implements OAuth2 bearer-token verification for protected
// routes: unlike the earlier implementation, it verifies the JWT's HMAC
// (HS256) signature against a configured shared secret and enforces
// standard claim validation (expiration, not-before, issuer, audience) via
// golang-jwt/jwt/v5's built-in parser validation, rather than merely
// inspecting unverified claims.
package server

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	jwt "github.com/golang-jwt/jwt/v5"
)

// oauthValidator verifies bearer tokens presented to protected routes.
type oauthValidator struct {
	secret   []byte
	issuer   string
	audience string
}

func newOAuthValidator(secret, issuer, audience string) *oauthValidator {
	return &oauthValidator{secret: []byte(secret), issuer: issuer, audience: audience}
}

// requireOAuth2 wraps next so it only runs once a valid bearer token has
// been presented. If the server has no configured signing secret, every
// request is rejected with 500 rather than silently skipping signature
// verification - failing closed instead of open.
func (v *oauthValidator) requireOAuth2(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if len(v.secret) == 0 {
			writeJSON(w, http.StatusInternalServerError, map[string]string{
				"error": "server is not configured with an OAuth signing secret",
			})
			return
		}
		tokenString, err := bearerToken(r)
		if err != nil {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": err.Error()})
			return
		}
		if err := v.validate(tokenString); err != nil {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": err.Error()})
			return
		}
		next(w, r)
	}
}

// validate parses rawToken, verifying its HS256 signature against the
// configured secret and enforcing exp/nbf plus, when configured,
// issuer/audience claims.
func (v *oauthValidator) validate(rawToken string) error {
	options := []jwt.ParserOption{
		jwt.WithValidMethods([]string{"HS256"}),
		jwt.WithExpirationRequired(),
	}
	if v.issuer != "" {
		options = append(options, jwt.WithIssuer(v.issuer))
	}
	if v.audience != "" {
		options = append(options, jwt.WithAudience(v.audience))
	}

	claims := jwt.MapClaims{}
	keyFunc := func(*jwt.Token) (any, error) { return v.secret, nil }
	if _, err := jwt.ParseWithClaims(rawToken, claims, keyFunc, options...); err != nil {
		return fmt.Errorf("invalid bearer token: %w", err)
	}
	return nil
}

// bearerToken extracts the raw token string from a request's Authorization
// header, requiring the "Bearer" scheme.
func bearerToken(r *http.Request) (string, error) {
	authorization := r.Header.Get("Authorization")
	if authorization == "" {
		return "", errors.New("missing bearer token")
	}
	parts := strings.SplitN(authorization, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") || strings.TrimSpace(parts[1]) == "" {
		return "", errors.New("authorization header must use Bearer scheme")
	}
	return strings.TrimSpace(parts[1]), nil
}
