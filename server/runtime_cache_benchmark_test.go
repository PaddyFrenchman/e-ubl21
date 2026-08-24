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

func BenchmarkOpenAPIRepresentation(b *testing.B) {
	b.Run("cached_json", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			if len(cachedOpenAPIJSON) == 0 {
				b.Fatal("empty OpenAPI JSON")
			}
		}
	})
	b.Run("marshal_cached_map_each_request", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			if len(mustMarshalOpenAPI(openAPIDocument())) == 0 {
				b.Fatal("empty OpenAPI JSON")
			}
		}
	})
	b.Run("rebuild_and_marshal_each_request", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			if len(mustMarshalOpenAPI(buildOpenAPIDocument())) == 0 {
				b.Fatal("empty OpenAPI JSON")
			}
		}
	})
}
