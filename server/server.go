package server

import (
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	jwt "github.com/golang-jwt/jwt/v5"
)

// Config holds HTTP server configuration.
type Config struct {
	OpenAPIPath   string
	OAuthIssuer   string
	OAuthAudience string
}

// NewHandler builds the OAuth2-protected HTTP API for UBL validation uploads.
func NewHandler(cfg Config) http.Handler {
	mux := http.NewServeMux()
	openAPIPath := cfg.OpenAPIPath
	if openAPIPath == "" {
		openAPIPath = "/openapi.json"
	}
	if !strings.HasPrefix(openAPIPath, "/") {
		openAPIPath = "/" + openAPIPath
	}

	mux.HandleFunc("/", requireOAuth2(cfg.OAuthIssuer, cfg.OAuthAudience, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"service": "e-ubl21",
			"oauth": map[string]string{
				"issuer":   cfg.OAuthIssuer,
				"audience": cfg.OAuthAudience,
			},
		})
	}))

	mux.HandleFunc("/healthz", requireOAuth2(cfg.OAuthIssuer, cfg.OAuthAudience, func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	}))

	mux.HandleFunc(openAPIPath, requireOAuth2(cfg.OAuthIssuer, cfg.OAuthAudience, func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, openAPIDocument())
	}))

	mux.HandleFunc("/api/v1/ubl/upload", requireOAuth2(cfg.OAuthIssuer, cfg.OAuthAudience, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		body, err := readUploadBody(r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if len(strings.TrimSpace(string(body))) == 0 {
			http.Error(w, "request body is empty", http.StatusBadRequest)
			return
		}

		if err := validateXMLPayload(body); err != nil {
			http.Error(w, "invalid XML payload: "+err.Error(), http.StatusBadRequest)
			return
		}

		writeJSON(w, http.StatusAccepted, map[string]any{
			"status":     "accepted",
			"document":   "ubl-2.1-xml",
			"receivedAt": time.Now().UTC().Format(time.RFC3339),
		})
	}))

	return mux
}

func requireOAuth2(issuer, audience string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tokenString, err := bearerToken(r)
		if err != nil {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": err.Error()})
			return
		}
		if err := validateBearerToken(tokenString, issuer, audience); err != nil {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": err.Error()})
			return
		}
		next(w, r)
	}
}

func bearerToken(r *http.Request) (string, error) {
	authorization := r.Header.Get("Authorization")
	if authorization == "" {
		return "", errors.New("missing bearer token")
	}
	parts := strings.SplitN(authorization, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") || strings.TrimSpace(parts[1]) == "" {
		return "", errors.New("authorization header must use Bearer token")
	}
	return strings.TrimSpace(parts[1]), nil
}

func validateBearerToken(rawToken, issuer, audience string) error {
	if rawToken == "" {
		return errors.New("missing bearer token")
	}
	if !strings.Contains(rawToken, ".") {
		return errors.New("token is not a JWT")
	}

	claims := jwt.MapClaims{}
	parser := new(jwt.Parser)
	_, _, err := parser.ParseUnverified(rawToken, claims)
	if err != nil {
		return fmt.Errorf("invalid JWT: %w", err)
	}

	if issuer != "" {
		iss, ok := claims["iss"].(string)
		if !ok || iss != issuer {
			return fmt.Errorf("issuer claim mismatch: want %q", issuer)
		}
	}

	if audience != "" {
		if err := validateAudience(claims["aud"], audience); err != nil {
			return err
		}
	}

	return nil
}

func validateAudience(raw any, expected string) error {
	switch v := raw.(type) {
	case string:
		if v == expected {
			return nil
		}
		return fmt.Errorf("audience claim mismatch: want %q", expected)
	case []any:
		for _, item := range v {
			if s, ok := item.(string); ok && s == expected {
				return nil
			}
		}
		return fmt.Errorf("audience claim mismatch: want %q", expected)
	case []string:
		for _, item := range v {
			if item == expected {
				return nil
			}
		}
		return fmt.Errorf("audience claim mismatch: want %q", expected)
	default:
		return fmt.Errorf("audience claim mismatch: want %q", expected)
	}
}

func openAPIDocument() map[string]any {
	return map[string]any{
		"openapi": "3.0.3",
		"info": map[string]any{
			"title":       "e-ubl21 API",
			"version":     "0.1.0",
			"description": "API for uploading and validating UBL 2.1 XML documents.",
		},
		"servers": []map[string]string{{"url": "https://api.example.com"}},
		"security": []map[string][]string{{"oauth2": {}}},
		"tags": []map[string]string{{"name": "UBL", "description": "UBL document upload and validation operations."}},
		"paths": map[string]any{
			"/api/v1/ubl/upload": map[string]any{
				"post": map[string]any{
					"operationId": "UBLUploadUploadXMLDocument",
					"tags":        []string{"UBL"},
					"summary":     "Upload a UBL 2.1 XML document",
					"description": "Accepts a UBL 2.1 XML payload for validation and processing.",
					"security":    []map[string][]string{{"oauth2": {}}},
					"requestBody": map[string]any{
						"required":    true,
						"description": "Raw XML document payload for a UBL 2.1 invoice or other supported document type.",
						"content": map[string]any{
							"application/xml": map[string]any{
								"schema": map[string]any{"type": "string"},
							},
						},
					},
					"responses": map[string]any{
						"202": map[string]any{
							"description": "Document accepted for validation.",
							"content": map[string]any{
								"application/json": map[string]any{
									"schema": map[string]any{
										"type": "object",
										"properties": map[string]any{
											"status": map[string]any{"type": "string"},
											"document": map[string]any{"type": "string"},
											"receivedAt": map[string]any{"type": "string", "format": "date-time"},
										},
										"required": []string{"status", "document", "receivedAt"},
									},
									"example": map[string]any{
										"status":     "accepted",
										"document":   "ubl-2.1-xml",
										"receivedAt": "2026-08-20T12:00:00Z",
									},
								},
							},
						},
						"400": map[string]any{
							"description": "Invalid XML payload or malformed request.",
							"content": map[string]any{"application/json": map[string]any{"schema": map[string]any{"$ref": "#/components/schemas/ErrorResponse"}}},
						},
						"401": map[string]any{
							"description": "Missing or invalid OAuth 2 bearer token.",
							"content": map[string]any{"application/json": map[string]any{"schema": map[string]any{"$ref": "#/components/schemas/ErrorResponse"}}},
						},
						"405": map[string]any{
							"description": "Method not allowed.",
							"content": map[string]any{"application/json": map[string]any{"schema": map[string]any{"$ref": "#/components/schemas/ErrorResponse"}}},
						},
					},
				},
			},
		},
		"components": map[string]any{
			"schemas": map[string]any{
				"ErrorResponse": map[string]any{
					"description": "Standard error payload returned when a request is rejected.",
					"type":        "object",
					"properties": map[string]any{"error": map[string]any{"type": "string"}},
					"required":    []string{"error"},
					"additionalProperties": false,
				},
			},
			"securitySchemes": map[string]any{
				"oauth2": map[string]any{
					"type":        "oauth2",
					"description": "OAuth 2 bearer token used to authenticate the caller.",
					"flows": map[string]any{
						"clientCredentials": map[string]any{
							"tokenUrl": "https://issuer.example.com/oauth/token",
							"scopes":   map[string]string{},
						},
					},
				},
			},
		},
	}
}

func readUploadBody(r *http.Request) ([]byte, error) {
	if r.ContentLength == 0 {
		return nil, errors.New("request body is required")
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, 10<<20))
	if err != nil {
		return nil, err
	}
	contentType := r.Header.Get("Content-Type")
	if !strings.Contains(contentType, "xml") && !strings.Contains(contentType, "octet-stream") {
		return nil, errors.New("content-type must be application/xml, text/xml, or application/octet-stream")
	}
	return body, nil
}

func validateXMLPayload(body []byte) error {
	trimmed := strings.TrimSpace(string(body))
	if trimmed == "" {
		return errors.New("empty XML payload")
	}
	if strings.HasPrefix(trimmed, "<") == false {
		return errors.New("payload is not XML")
	}

	var doc struct {
		XMLName xml.Name
	}
	if err := xml.Unmarshal(body, &doc); err != nil {
		return err
	}
	if doc.XMLName.Local == "" {
		return errors.New("XML document root element is missing")
	}
	return nil
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}
