package main

import (
	"testing"
	"time"

	jwt "github.com/golang-jwt/jwt/v5"
)

func TestSignTokenProducesAcceptedHS256Claims(t *testing.T) {
	now := time.Date(2026, 8, 28, 12, 0, 0, 0, time.UTC)
	raw, err := signToken("local-secret", "issuer", "audience", time.Hour, now)
	if err != nil {
		t.Fatal(err)
	}
	claims := jwt.RegisteredClaims{}
	parsed, err := jwt.ParseWithClaims(raw, &claims, func(*jwt.Token) (any, error) { return []byte("local-secret"), nil },
		jwt.WithValidMethods([]string{"HS256"}), jwt.WithIssuer("issuer"), jwt.WithAudience("audience"), jwt.WithTimeFunc(func() time.Time { return now.Add(time.Minute) }))
	if err != nil || !parsed.Valid {
		t.Fatalf("parse token: valid=%t err=%v", parsed.Valid, err)
	}
}

func TestSignTokenRejectsUnsafeConfiguration(t *testing.T) {
	for _, test := range []struct {
		secret, issuer, audience string
		ttl                      time.Duration
	}{{"", "issuer", "audience", time.Hour}, {"secret", "", "audience", time.Hour}, {"secret", "issuer", "", time.Hour}, {"secret", "issuer", "audience", 0}} {
		if _, err := signToken(test.secret, test.issuer, test.audience, test.ttl, time.Now()); err == nil {
			t.Fatalf("accepted invalid configuration: %+v", test)
		}
	}
}
