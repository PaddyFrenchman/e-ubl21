package main

import (
	"encoding/base64"
	"io"
	"os"
	"strings"
	"testing"
)

func TestMainPrintsRandomURLSafeSecret(t *testing.T) {
	original := os.Stdout
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		os.Stdout = original
		_ = reader.Close()
		_ = writer.Close()
	})
	os.Stdout = writer
	main()
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	content, err := io.ReadAll(reader)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := base64.RawURLEncoding.DecodeString(strings.TrimSpace(string(content)))
	if err != nil || len(decoded) != 32 {
		t.Fatalf("secret output = %q, decoded bytes = %d, error = %v", content, len(decoded), err)
	}
}
