package queue

import (
	"strings"
	"testing"
)

func TestUploadMetadataValuesRejectMalformedTimestamps(t *testing.T) {
	validID := "0123456789abcdef0123456789abcdef"
	for _, field := range []struct {
		name   string
		record UploadRecord
	}{
		{"receivedAt", UploadRecord{ID: validID, ReceivedAt: "bad-time"}},
		{"validatedAt", UploadRecord{ID: validID, ReceivedAt: "2026-09-28T00:00:00Z", ValidatedAt: "bad-time"}},
		{"processingFailedAt", UploadRecord{ID: validID, ReceivedAt: "2026-09-28T00:00:00Z", ProcessingFailedAt: "bad-time"}},
		{"contentExpiresAt", UploadRecord{ID: validID, ReceivedAt: "2026-09-28T00:00:00Z", ContentExpiresAt: "bad-time"}},
		{"contentExpiredAt", UploadRecord{ID: validID, ReceivedAt: "2026-09-28T00:00:00Z", ContentExpiredAt: "bad-time"}},
	} {
		t.Run(field.name, func(t *testing.T) {
			_, err := uploadMetadataValues(field.record)
			if err == nil || !strings.Contains(err.Error(), field.name) {
				t.Fatalf("uploadMetadataValues() error = %v, want %s validation error", err, field.name)
			}
		})
	}
}
