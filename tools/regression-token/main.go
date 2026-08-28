package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"time"

	jwt "github.com/golang-jwt/jwt/v5"
)

func main() {
	issuer := flag.String("issuer", "https://regression.e-ubl21.local", "JWT issuer")
	audience := flag.String("audience", "e-ubl21-regression", "JWT audience")
	ttl := flag.Duration("ttl", 2*time.Hour, "JWT lifetime")
	flag.Parse()
	token, err := signToken(os.Getenv("EUBL21_REGRESSION_HMAC_SECRET"), *issuer, *audience, *ttl, time.Now().UTC())
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println(token)
}

func signToken(secret, issuer, audience string, ttl time.Duration, now time.Time) (string, error) {
	if secret == "" {
		return "", errors.New("EUBL21_REGRESSION_HMAC_SECRET is required")
	}
	if issuer == "" || audience == "" || ttl <= 0 {
		return "", errors.New("issuer, audience, and a positive TTL are required")
	}
	claims := jwt.RegisteredClaims{
		Issuer: issuer, Audience: jwt.ClaimStrings{audience},
		IssuedAt: jwt.NewNumericDate(now), ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(secret))
}
