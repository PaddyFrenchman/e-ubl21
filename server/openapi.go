// This file defines the in-code OpenAPI 3.0.3 document served at the
// configurable OpenAPIPath and rendered by the Swagger UI (see server.go).
// It documents every route NewHandler actually registers - including the
// operational "/", "/healthz" and "/metrics" endpoints, not just the UBL
// upload/retrieval API - and every response code the shared middleware
// chain (auth.go, ratelimit.go) can genuinely produce on each of them, so
// it stays an accurate description of server.go's behavior rather than
// only of the UBL-specific handlers.
package server

// errorResponseContent is the reusable "application/json" content object
// referencing the shared ErrorResponse schema, used by every non-2xx
// response below.
func errorResponseContent() map[string]any {
	return map[string]any{
		"application/json": map[string]any{
			"schema": map[string]any{"$ref": "#/components/schemas/ErrorResponse"},
		},
	}
}

func errorResponseWithExample(description, exampleError string) map[string]any {
	return map[string]any{
		"description": description,
		"content": map[string]any{
			"application/json": map[string]any{
				"schema":  map[string]any{"$ref": "#/components/schemas/ErrorResponse"},
				"example": map[string]any{"error": exampleError},
			},
		},
	}
}

// unauthorizedResponse, methodNotAllowedResponse, rateLimitedResponse and
// misconfiguredResponse describe response codes every route behind
// requireOAuth2 and the rate limiter can produce (see auth.go,
// ratelimit.go, and requireGET in server.go), so they are attached to every
// operation below rather than only the UBL endpoints.
func unauthorizedResponse() map[string]any {
	return errorResponseWithExample("Missing or invalid OAuth 2 bearer token.", "missing bearer token")
}

func methodNotAllowedResponse() map[string]any {
	return errorResponseWithExample("Method not allowed.", "method not allowed")
}

func rateLimitedResponse() map[string]any {
	return errorResponseWithExample("Rate limit exceeded.", "rate limit exceeded")
}

// misconfiguredResponse documents the fail-closed 500 every protected route
// can return when the server has no configured OAuth signing secret (see
// oauthValidator.requireOAuth2 in auth.go); the UBL upload endpoint can
// additionally 500 for its own internal reasons (queue/storage failures),
// documented separately on that operation.
func misconfiguredResponse(extra string) map[string]any {
	description := "Server misconfiguration: no OAuth signing secret is configured."
	if extra != "" {
		description += " " + extra
	}
	return map[string]any{
		"description": description,
		"content":     errorResponseContent(),
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
		"servers":  []map[string]string{{"url": "https://api.example.com"}},
		"security": []map[string][]string{{"oauth2": {}}},
		"tags": []map[string]string{
			{"name": "UBL", "description": "UBL document upload and validation operations."},
			{"name": "Service", "description": "Service metadata, liveness and metrics endpoints."},
		},
		"paths": map[string]any{
			"/": map[string]any{
				"get": map[string]any{
					"operationId": "ServiceInfo",
					"tags":        []string{"Service"},
					"summary":     "Service information",
					"description": "Returns the service name, the configured Swagger UI path, and the configured OAuth issuer/audience.",
					"responses": map[string]any{
						"200": map[string]any{
							"description": "Service information.",
							"content": map[string]any{
								"application/json": map[string]any{
									"schema": map[string]any{
										"type": "object",
										"properties": map[string]any{
											"service": map[string]any{"type": "string"},
											"docs":    map[string]any{"type": "string", "description": "The path the Swagger UI is served from."},
											"oauth": map[string]any{
												"type": "object",
												"properties": map[string]any{
													"issuer":   map[string]any{"type": "string"},
													"audience": map[string]any{"type": "string"},
												},
											},
										},
										"required": []string{"service", "docs", "oauth"},
									},
									"example": map[string]any{
										"service": "e-ubl21",
										"docs":    "/swagger/",
										"oauth":   map[string]any{"issuer": "https://issuer.example.com", "audience": "e-ubl21"},
									},
								},
							},
						},
						"401": unauthorizedResponse(),
						"405": methodNotAllowedResponse(),
						"429": rateLimitedResponse(),
						"500": misconfiguredResponse(""),
					},
				},
			},
			"/healthz": map[string]any{
				"get": map[string]any{
					"operationId": "HealthCheck",
					"tags":        []string{"Service"},
					"summary":     "Liveness check",
					"responses": map[string]any{
						"200": map[string]any{
							"description": "The service is up.",
							"content": map[string]any{
								"application/json": map[string]any{
									"schema":  map[string]any{"type": "object", "properties": map[string]any{"status": map[string]any{"type": "string"}}, "required": []string{"status"}},
									"example": map[string]any{"status": "ok"},
								},
							},
						},
						"401": unauthorizedResponse(),
						"405": methodNotAllowedResponse(),
						"429": rateLimitedResponse(),
						"500": misconfiguredResponse(""),
					},
				},
			},
			"/metrics": map[string]any{
				"get": map[string]any{
					"operationId": "Metrics",
					"tags":        []string{"Service"},
					"summary":     "Prometheus metrics",
					"description": "Exposes e_ubl21_* and standard Go runtime/process metrics in the Prometheus text exposition format.",
					"responses": map[string]any{
						"200": map[string]any{
							"description": "Metrics in Prometheus text exposition format.",
							"content":     map[string]any{"text/plain": map[string]any{"schema": map[string]any{"type": "string"}}},
						},
						"401": unauthorizedResponse(),
						"405": methodNotAllowedResponse(),
						"429": rateLimitedResponse(),
						"500": misconfiguredResponse(""),
					},
				},
			},
			"/api/v1/ubl/upload": map[string]any{
				"post": map[string]any{
					"operationId": "UBLUploadUploadXMLDocument",
					"tags":        []string{"UBL"},
					"summary":     "Upload a UBL 2.1 XML document",
					"description": "Accepts an XML payload, and if it is well-formed and its root element is a recognized UBL 2.1 main document type, queues it on its per-document-type NATS JetStream subject for asynchronous XSD validation and returns immediately with a pending status. Poll GET /api/v1/ubl/{id} for the outcome once the worker has processed it.",
					"requestBody": map[string]any{
						"required":    true,
						"description": "Raw XML document payload for a UBL 2.1 invoice or other supported main document type.",
						"content": map[string]any{
							"application/xml":          map[string]any{"schema": map[string]any{"type": "string"}},
							"text/xml":                 map[string]any{"schema": map[string]any{"type": "string"}},
							"application/octet-stream": map[string]any{"schema": map[string]any{"type": "string", "format": "binary"}},
						},
					},
					"responses": map[string]any{
						"202": map[string]any{
							"description": "Document is well-formed and its root element is a recognized UBL 2.1 document type. It has been queued for asynchronous XSD validation; its status starts as \"pending\" and later becomes \"valid\" or \"invalid\" once the worker processes it (see GET /api/v1/ubl/{id}).",
							"content": map[string]any{
								"application/json": map[string]any{
									"schema": map[string]any{
										"type": "object",
										"properties": map[string]any{
											"status":     map[string]any{"type": "string", "enum": []string{"pending"}},
											"id":         map[string]any{"type": "string", "description": "Identifier to retrieve this upload's status/content via GET /api/v1/ubl/{id}."},
											"document":   map[string]any{"type": "string", "description": "The UBL 2.1 root element name (e.g. Invoice, Order)."},
											"receivedAt": map[string]any{"type": "string", "format": "date-time"},
										},
										"required": []string{"status", "id", "document", "receivedAt"},
									},
									"example": map[string]any{
										"status":     "pending",
										"id":         "3f9c2b1a7e4d4f0a9b1c2d3e4f5a6b7c",
										"document":   "Invoice",
										"receivedAt": "2026-08-20T12:00:00Z",
									},
								},
							},
						},
						"400": errorResponseWithExample(
							"Invalid XML payload or an unrecognized document type. Note: this endpoint no longer performs synchronous XSD validation, so a schema-invalid (but well-formed, recognized-type) document is still accepted with 202 - check its status via GET /api/v1/ubl/{id} once processed.",
							"invalid XML payload: XML document root element is missing",
						),
						"401": unauthorizedResponse(),
						"405": methodNotAllowedResponse(),
						"429": rateLimitedResponse(),
						"500": misconfiguredResponse("Alternatively, the document could not be queued (e.g. the NATS/JetStream connection failed)."),
					},
				},
			},
			"/api/v1/ubl/{id}": map[string]any{
				"get": map[string]any{
					"operationId": "UBLUploadGetXMLDocument",
					"tags":        []string{"UBL"},
					"summary":     "Retrieve a previously accepted UBL 2.1 XML document",
					"description": "Returns the upload's current status - \"pending\" (queued, not yet processed), \"valid\" or \"invalid\" (processed by the worker) - along with its original content and, once processed, the validation error (if any) and timestamp.",
					"parameters": []map[string]any{
						{
							"name":        "id",
							"in":          "path",
							"required":    true,
							"schema":      map[string]any{"type": "string"},
							"description": "The id returned by a previous successful upload.",
						},
					},
					"responses": map[string]any{
						"200": map[string]any{
							"description": "The stored document, its current status, and its metadata.",
							"content": map[string]any{
								"application/json": map[string]any{
									"schema": map[string]any{
										"type": "object",
										"properties": map[string]any{
											"id":          map[string]any{"type": "string"},
											"document":    map[string]any{"type": "string"},
											"status":      map[string]any{"type": "string", "enum": []string{"pending", "valid", "invalid"}},
											"receivedAt":  map[string]any{"type": "string", "format": "date-time"},
											"validatedAt": map[string]any{"type": "string", "format": "date-time", "description": "Present once the worker has processed the document."},
											"error":       map[string]any{"type": "string", "description": "Present only when status is \"invalid\": the XSD validation failure detail."},
											"content":     map[string]any{"type": "string", "description": "The original raw XML payload."},
										},
										"required": []string{"id", "document", "status", "receivedAt", "content"},
									},
								},
							},
						},
						"401": unauthorizedResponse(),
						"404": errorResponseWithExample("No upload exists with the given id.", "upload not found"),
						"405": methodNotAllowedResponse(),
						"429": rateLimitedResponse(),
						"500": misconfiguredResponse(""),
					},
				},
			},
		},
		"components": map[string]any{
			"schemas": map[string]any{
				"ErrorResponse": map[string]any{
					"description":          "Standard error payload returned when a request is rejected.",
					"type":                 "object",
					"properties":           map[string]any{"error": map[string]any{"type": "string"}},
					"required":             []string{"error"},
					"additionalProperties": false,
				},
			},
			"securitySchemes": map[string]any{
				"oauth2": map[string]any{
					"type":        "oauth2",
					"description": "OAuth 2 bearer token (HS256-signed JWT) used to authenticate the caller.",
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
