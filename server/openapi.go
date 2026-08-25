// This file defines the in-code OpenAPI 3.0.3 document served at the
// configurable OpenAPIPath and rendered by the Swagger UI (see server.go).
package server

import (
	"encoding/json"
	"fmt"
)

func responseRef(name string) map[string]any {
	return map[string]any{"$ref": "#/components/responses/" + name}
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

func commonErrorResponses() map[string]any {
	return map[string]any{
		"401": responseRef("Unauthorized"),
		"405": responseRef("MethodNotAllowed"),
		"429": responseRef("RateLimited"),
		"500": responseRef("Misconfigured"),
	}
}

var cachedOpenAPIDocument = buildOpenAPIDocument()
var cachedOpenAPIJSON = mustMarshalOpenAPI(cachedOpenAPIDocument)

func openAPIDocument() map[string]any {
	return cachedOpenAPIDocument
}

func mustMarshalOpenAPI(document map[string]any) []byte {
	encoded, err := json.Marshal(document)
	if err != nil {
		panic(fmt.Sprintf("marshal cached OpenAPI document: %v", err))
	}
	return encoded
}

func buildOpenAPIDocument() map[string]any {
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
					"description": "Accepts an XML payload, stores the raw XML once, and if its root local name and namespace identify a recognized UBL 2.1 main document type, queues a compact work descriptor for asynchronous XSD validation. Full XML well-formedness and UBL 2.1 schema validity are established asynchronously by the worker.",
					"requestBody": map[string]any{
						"required":    true,
						"description": "Raw XML document payload for a UBL 2.1 invoice or other supported main document type. Limited to 10 MiB; see the 413 response.",
						"content": map[string]any{
							"application/xml":          map[string]any{"schema": map[string]any{"type": "string"}},
							"text/xml":                 map[string]any{"schema": map[string]any{"type": "string"}},
							"application/octet-stream": map[string]any{"schema": map[string]any{"type": "string", "format": "binary"}},
						},
					},
					"responses": map[string]any{
						"202": map[string]any{
							"description": "The payload has a recognized UBL 2.1 root element and is durably accepted for asynchronous validation. Status is accepting or enqueue_pending during recovery, pending once queued, and later valid or invalid.",
							"content": map[string]any{
								"application/json": map[string]any{
									"schema": map[string]any{
										"type": "object",
										"properties": map[string]any{
											"status":     map[string]any{"type": "string", "enum": []string{"accepting", "enqueue_pending", "pending"}, "example": "pending"},
											"id":         map[string]any{"type": "string", "example": "3f9c2b1a7e4d4f0a9b1c2d3e4f5a6b7c"},
											"document":   map[string]any{"type": "string", "example": "Invoice"},
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
						"400": errorResponseWithExample("The payload has no recognized UBL 2.1 root element.", "invalid XML payload: XML document root element is missing"),
						"413": errorResponseWithExample("The request body exceeds the maximum allowed upload size (10 MiB).", fmt.Sprintf("request body exceeds the maximum allowed size of %d bytes", maxUploadBytes)),
						"500": misconfiguredResponse("Alternatively, the document could not be stored or queued."),
					},
				}),
			},
			"/api/v1/ubl/{id}": map[string]any{
				"get": mergeResponses(map[string]any{
					"operationId": "UBLUploadGetXMLDocument",
					"tags":        []string{"UBL"},
					"summary":     "Retrieve upload status and metadata",
					"description": "Returns retained upload status and metadata without loading raw XML by default. Raw content expires independently after validation; set includeContent=true only while contentAvailable is true.",
					"parameters": []map[string]any{
						{
							"name":        "id",
							"in":          "path",
							"required":    true,
							"schema":      map[string]any{"type": "string"},
							"description": "The id returned by a previous successful upload.",
						},
						{
							"name":        "includeContent",
							"in":          "query",
							"required":    false,
							"description": "When true, include the original raw XML payload. Returns 410 after raw content expires; retained validation metadata remains available.",
							"schema":      map[string]any{"type": "boolean", "default": false},
						},
					},
					"responses": map[string]any{
						"200": map[string]any{
							"description": "Upload status and metadata. Content is present only when includeContent=true.",
							"content": map[string]any{
								"application/json": map[string]any{
									"schema": map[string]any{
										"type": "object",
										"properties": map[string]any{
											"id":               map[string]any{"type": "string", "example": "3f9c2b1a7e4d4f0a9b1c2d3e4f5a6b7c"},
											"document":         map[string]any{"type": "string", "example": "Invoice"},
											"status":           map[string]any{"type": "string", "enum": []string{"accepting", "cleanup_pending", "enqueue_pending", "pending", "valid", "invalid", "acceptance_failed"}, "example": "valid"},
											"receivedAt":       map[string]any{"type": "string", "format": "date-time", "example": "2026-08-20T12:00:00Z"},
											"validatedAt":      map[string]any{"type": "string", "format": "date-time", "description": "Present once the worker has processed the document.", "example": "2026-08-20T12:00:01Z"},
											"contentAvailable": map[string]any{"type": "boolean", "description": "Whether lifecycle metadata indicates that raw XML is retained and eligible for retrieval; this does not perform an Object Store read.", "example": true},
											"contentExpiresAt": map[string]any{"type": "string", "format": "date-time", "description": "Scheduled raw-content expiry time, present after validation when retention is enabled.", "example": "2026-09-19T12:00:01Z"},
											"contentExpiredAt": map[string]any{"type": "string", "format": "date-time", "description": "Time raw content became unavailable under the retention policy. Physical deletion is retried independently; validation metadata remains available.", "example": "2026-09-19T12:00:30Z"},
											"error":            map[string]any{"type": "string", "description": "Present only when status is invalid.", "example": "Invoice: element content does not follow the schema"},
											"content":          map[string]any{"type": "string", "description": "Original raw XML. Present only when includeContent=true.", "example": `<?xml version="1.0" encoding="UTF-8"?><Invoice xmlns="urn:oasis:names:specification:ubl:schema:xsd:Invoice-2">...</Invoice>`},
										},
										"required": []string{"id", "document", "status", "receivedAt", "contentAvailable"},
									},
								},
							},
						},
						"400": errorResponseWithExample("Invalid includeContent query parameter.", "includeContent must be true or false"),
						"404": errorResponseWithExample("No upload exists with the given id.", "upload not found"),
						"410": errorResponseWithExample("The upload metadata exists, but its raw XML content has expired.", "upload content has expired"),
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

func mergeResponses(operation map[string]any) map[string]any {
	responses := operation["responses"].(map[string]any)
	for status, resp := range commonErrorResponses() {
		responses[status] = resp
	}
	return operation
}
