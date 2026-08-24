package queue_test

import (
	"encoding/json"
	"testing"

	"github.com/PaddyFrenchman/e-ubl21/queue"
)

// BenchmarkDocumentWorkItemEncoding records the encoding cost and payload
// size of the reference-based work message. The representative payload is
// about 102 bytes, versus up to 10 MiB when the work stream carried raw XML.
func BenchmarkDocumentWorkItemEncoding(b *testing.B) {
	item := queue.DocumentWorkItem{
		ID:           "00000000000000000000000000000000",
		DocumentType: "Invoice",
		ReceivedAt:   "2026-08-24T18:06:34Z",
	}
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		payload, err := json.Marshal(item)
		if err != nil {
			b.Fatal(err)
		}
		if len(payload) > 256 {
			b.Fatalf("descriptor grew unexpectedly: %d bytes", len(payload))
		}
	}
}
