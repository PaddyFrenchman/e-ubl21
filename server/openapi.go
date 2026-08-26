// This file loads the checked-in OpenAPI 3.0.3 document served at the
// configurable OpenAPIPath and rendered by the Swagger UI (see server.go).
// Embedding the reviewed YAML makes the static and runtime representations one
// contract and prevents the two hand-maintained documents from drifting.
package server

import (
	_ "embed"
	"encoding/json"
	"fmt"

	"go.yaml.in/yaml/v3"
)

//go:embed openapi.yaml
var embeddedOpenAPIYAML []byte

var cachedOpenAPIDocument = buildOpenAPIDocument()
var cachedOpenAPIJSON = mustMarshalOpenAPI(cachedOpenAPIDocument)

func openAPIDocument() map[string]any {
	return cachedOpenAPIDocument
}

func buildOpenAPIDocument() map[string]any {
	var document map[string]any
	if err := yaml.Unmarshal(embeddedOpenAPIYAML, &document); err != nil {
		panic(fmt.Sprintf("parse embedded OpenAPI document: %v", err))
	}
	return document
}

func mustMarshalOpenAPI(document map[string]any) []byte {
	encoded, err := json.Marshal(document)
	if err != nil {
		panic(fmt.Sprintf("marshal cached OpenAPI document: %v", err))
	}
	return encoded
}
