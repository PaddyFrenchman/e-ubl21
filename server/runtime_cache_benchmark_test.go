package server

import (
	"testing"
	"time"

	jwt "github.com/golang-jwt/jwt/v5"
)

func BenchmarkOAuthValidationParser(b *testing.B) {
	const secret = "benchmark-secret"
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"exp": time.Now().Add(time.Hour).Unix(),
		"iss": "benchmark-issuer",
		"aud": "benchmark-audience",
	})
	raw, err := token.SignedString([]byte(secret))
	if err != nil {
		b.Fatal(err)
	}
	validator := newOAuthValidator(secret, "benchmark-issuer", "benchmark-audience")
	b.Run("cached", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			if err := validator.validate(raw); err != nil {
				b.Fatal(err)
			}
		}
	})
	b.Run("rebuild_each_request", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			if err := newOAuthValidator(secret, "benchmark-issuer", "benchmark-audience").validate(raw); err != nil {
				b.Fatal(err)
			}
		}
	})
}

func BenchmarkOpenAPIDocument(b *testing.B) {
	b.Run("cached", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			if openAPIDocument() == nil {
				b.Fatal("nil OpenAPI document")
			}
		}
	})
	b.Run("rebuild_each_request", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			if buildOpenAPIDocument() == nil {
				b.Fatal("nil OpenAPI document")
			}
		}
	})
}
