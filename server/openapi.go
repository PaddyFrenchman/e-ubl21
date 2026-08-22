// This file defines the in-code OpenAPI 3.0.3 document served at the
// configurable OpenAPIPath and rendered by the Swagger UI (see server.go).
// It documents every route NewHandler actually registers - including the
// operational "/", "/healthz" and "/metrics" endpoints, not just the UBL
// upload/retrieval API - and every response code the shared middleware
// chain (auth.go, ratelimit.go) can genuinely produce on each of them, so
// it stays an accurate description of server.go's behavior rather than
// only of the UBL-specific handlers.
//
// The 401/405/429 responses, and the plain (no-extra-detail) 500, are
// identical across every operation (they describe the same shared
// middleware behavior), so they are defined once under
// components.responses and referenced via $ref rather than repeated
// verbatim on every operation - mirroring openapi.yaml, which uses the
// same $ref-based structure to avoid duplicated descriptions.
package server

// responseRef returns a "$ref" pointer to the named entry under
// components.responses.
func responseRef(name string) map[string]any {
	return map[string]any{"$ref": "#/components/responses/" + name}
}

// errorResponseWithExample builds a standalone (non-shared) error response
// object: used for responses that are unique to a single operation (400,
// 404) rather than common across all of them.
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

// misconfiguredResponse documents the fail-closed 500 every protected route
// can return when the server has no configured OAuth signing secret (see
// oauthValidator.requireOAuth2 in auth.go). The UBL upload endpoint 500s
// for an additional, distinct reason (queue/storage failures) and so gets
// its own non-shared response with an extended description, built by
// calling this with a non-empty extra.
func misconfiguredResponse(extra string) map[string]any {
	description := "Server misconfiguration: no OAuth signing secret is configured."
	if extra != "" {
		description += " " + extra
	}
	return map[string]any{
		"description": description,
		"content": map[string]any{
			"application/json": map[string]any{
				"schema":  map[string]any{"$ref": "#/components/schemas/ErrorResponse"},
				"example": map[string]any{"error": "server is not configured with an OAuth signing secret"},
			},
		},
	}
}

// commonErrorResponses returns the 401/405/429/500 responses shared by
// every operation in the API (see components.responses in openAPIDocument).
func commonErrorResponses() map[string]any {
	return map[string]any{
		"401": responseRef("Unauthorized"),
		"405": responseRef("MethodNotAllowed"),
		"429": responseRef("RateLimited"),
		"500": responseRef("Misconfigured"),
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
				"get": mergeResponses(map[string]any{
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
											"service": map[string]any{"type": "string", "example": "e-ubl21"},
											"docs":    map[string]any{"type": "string", "description": "The path the Swagger UI is served from.", "example": "/swagger/"},
											"oauth": map[string]any{
												"type": "object",
												"properties": map[string]any{
													"issuer":   map[string]any{"type": "string", "example": "https://issuer.example.com"},
													"audience": map[string]any{"type": "string", "example": "e-ubl21"},
												},
												"example": map[string]any{"issuer": "https://issuer.example.com", "audience": "e-ubl21"},
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
					},
				}),
			},
			"/healthz": map[string]any{
				"get": mergeResponses(map[string]any{
					"operationId": "HealthCheck",
					"tags":        []string{"Service"},
					"summary":     "Liveness check",
					"responses": map[string]any{
						"200": map[string]any{
							"description": "The service is up.",
							"content": map[string]any{
								"application/json": map[string]any{
									"schema": map[string]any{
										"type":       "object",
										"properties": map[string]any{"status": map[string]any{"type": "string", "example": "ok"}},
										"required":   []string{"status"},
									},
									"example": map[string]any{"status": "ok"},
								},
							},
						},
					},
				}),
			},
			"/metrics": map[string]any{
				"get": mergeResponses(map[string]any{
					"operationId": "Metrics",
					"tags":        []string{"Service"},
					"summary":     "Prometheus metrics",
					"description": "Exposes e_ubl21_* and standard Go runtime/process metrics in the Prometheus text exposition format.",
					"responses": map[string]any{
						"200": map[string]any{
							"description": "Metrics in Prometheus text exposition format.",
							"content":     map[string]any{"text/plain": map[string]any{"schema": map[string]any{"type": "string"}}},
						},
					},
				}),
			},
			"/api/v1/ubl/upload": map[string]any{
				"post": mergeResponses(map[string]any{
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
											"status":     map[string]any{"type": "string", "enum": []string{"pending"}, "example": "pending"},
											"id":         map[string]any{"type": "string", "description": "Identifier to retrieve this upload's status/content via GET /api/v1/ubl/{id}.", "example": "3f9c2b1a7e4d4f0a9b1c2d3e4f5a6b7c"},
											"document":   map[string]any{"type": "string", "description": "The UBL 2.1 root element name (e.g. Invoice, Order).", "example": "Invoice"},
											"receivedAt": map[string]any{"type": "string", "format": "date-time", "example": "2026-08-20T12:00:00Z"},
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
						"500": misconfiguredResponse("Alternatively, the document could not be queued (e.g. the NATS/JetStream connection failed)."),
					},
				}),
			},
			"/api/v1/ubl/{id}": map[string]any{
				"get": mergeResponses(map[string]any{
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
											"id":          map[string]any{"type": "string", "example": "3f9c2b1a7e4d4f0a9b1c2d3e4f5a6b7c"},
											"document":    map[string]any{"type": "string", "example": "Invoice"},
											"status":      map[string]any{"type": "string", "enum": []string{"pending", "valid", "invalid"}, "example": "valid"},
											"receivedAt":  map[string]any{"type": "string", "format": "date-time", "example": "2026-08-20T12:00:00Z"},
											"validatedAt": map[string]any{"type": "string", "format": "date-time", "description": "Present once the worker has processed the document.", "example": "2026-08-20T12:00:01Z"},
											"error":       map[string]any{"type": "string", "description": "Present only when status is \"invalid\": the XSD validation failure detail.", "example": "element X is not allowed"},
											"content":     map[string]any{"type": "string", "description": "The original raw XML payload.", "example": "<Invoice>...</Invoice>"},
										},
										"required": []string{"id", "document", "status", "receivedAt", "content"},
									},
									"example": map[string]any{
										"id":          "3f9c2b1a7e4d4f0a9b1c2d3e4f5a6b7c",
										"document":    "Invoice",
										"status":      "valid",
										"receivedAt":  "2026-08-20T12:00:00Z",
										"validatedAt": "2026-08-20T12:00:01Z",
										"content":     "<Invoice>...</Invoice>",
									},
								},
							},
						},
						"404": errorResponseWithExample("No upload exists with the given id.", "upload not found"),
					},
				}),
			},
		},
		"components": map[string]any{
			"schemas": map[string]any{
				"ErrorResponse": map[string]any{
					"description": "Standard error payload returned when a request is rejected.",
					"type":        "object",
					"properties": map[string]any{
						"error": map[string]any{"type": "string", "example": "missing bearer token"},
					},
					"required":             []string{"error"},
					"additionalProperties": false,
					"example":              map[string]any{"error": "missing bearer token"},
				},
			},
			"responses": map[string]any{
				"Unauthorized":     errorResponseWithExample("Missing or invalid OAuth 2 bearer token.", "missing bearer token"),
				"MethodNotAllowed": errorResponseWithExample("Method not allowed.", "method not allowed"),
				"RateLimited":      errorResponseWithExample("Rate limit exceeded.", "rate limit exceeded"),
				"Misconfigured":    misconfiguredResponse(""),
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

// mergeResponses takes an operation object whose "responses" entry only
// lists the responses unique to that operation, and adds the shared
// 401/405/429/500 responses (see commonErrorResponses) every operation in
// this API can also produce, mutating and returning the same operation map.
func mergeResponses(operation map[string]any) map[string]any {
	responses := operation["responses"].(map[string]any)
	for status, resp := range commonErrorResponses() {
		responses[status] = resp
	}
	return operation
}
