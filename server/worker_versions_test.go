package server

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"testing"

	xsdschemas "github.com/PaddyFrenchman/e-ubl21/XSD"
	validationassets "github.com/PaddyFrenchman/e-ubl21/validation-assets"
)

func TestValidationRuntimeVersionLogIdentifiesPinnedStandardsAndTools(t *testing.T) {
	var output bytes.Buffer
	logValidationRuntimeVersions(slog.New(slog.NewJSONHandler(&output, nil)))

	var record map[string]any
	if err := json.Unmarshal(output.Bytes(), &record); err != nil {
		t.Fatalf("decode runtime version log: %v", err)
	}
	profile := validationassets.EN16931()
	saxon := validationassets.Saxon()
	want := map[string]string{
		"msg":                    "validation runtime verified",
		"ublSchemaVersion":       xsdschemas.Version,
		"en16931ArtifactVersion": profile.Version,
		"en16931SourceRelease":   profile.SourceRelease,
		"en16931XSLTSHA256":      profile.XSLTSHA256,
		"saxonVersion":           saxon.SaxonVersion,
		"saxonJARSHA256":         saxon.SaxonJARSHA256,
		"xmlResolverVersion":     saxon.XMLResolverVersion,
		"xmlResolverJARSHA256":   saxon.XMLResolverSHA256,
	}
	for field, expected := range want {
		if got := record[field]; got != expected {
			t.Errorf("%s = %v, want %q", field, got, expected)
		}
	}
}
