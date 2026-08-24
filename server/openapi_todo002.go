package server

// todo002OpenAPIDocument adjusts the generated OpenAPI document for the
// TODO-002 retrieval contract: status polling is metadata-only by default
// and raw XML is opt-in through includeContent=true.
func todo002OpenAPIDocument(doc map[string]any) map[string]any {
	paths := doc["paths"].(map[string]any)
	get := paths["/api/v1/ubl/{id}"].(map[string]any)["get"].(map[string]any)

	get["summary"] = "Retrieve upload status and metadata"
	get["description"] = "Returns the upload's current status and metadata without loading the raw XML by default. Set includeContent=true only when the original XML payload is required."
	params := get["parameters"].([]map[string]any)
	get["parameters"] = append(params, map[string]any{
		"name":        "includeContent",
		"in":          "query",
		"required":    false,
		"description": "When true, include the original raw XML payload in the response. Defaults to false so normal status polling remains small.",
		"schema": map[string]any{
			"type":    "boolean",
			"default": false,
		},
	})

	responses := get["responses"].(map[string]any)
	ok := responses["200"].(map[string]any)
	ok["description"] = "Upload status and metadata. The content field is present only when includeContent=true."
	media := ok["content"].(map[string]any)["application/json"].(map[string]any)
	schema := media["schema"].(map[string]any)
	props := schema["properties"].(map[string]any)
	props["content"] = map[string]any{
		"type":        "string",
		"description": "The original raw XML payload. Present only when includeContent=true.",
		"example":     "<Invoice>...</Invoice>",
	}
	schema["required"] = []string{"id", "document", "status", "receivedAt"}
	media["example"] = map[string]any{
		"id":          "3f9c2b1a7e4d4f0a9b1c2d3e4f5a6b7c",
		"document":    "Invoice",
		"status":      "valid",
		"receivedAt":  "2026-08-20T12:00:00Z",
		"validatedAt": "2026-08-20T12:00:01Z",
	}
	responses["400"] = errorResponseWithExample("Invalid includeContent query parameter.", "includeContent must be true or false")
	return doc
}
