package server

import (
	"bytes"
	"fmt"
	"io"

	"github.com/PaddyFrenchman/e-ubl21/peppol"
	"github.com/PaddyFrenchman/e-ubl21/queue"
)

// stageValidationReader preserves the complete submitted envelope for the
// envelope stage and exposes only its byte-exact embedded UBL payload to all
// payload stages in the same frozen execution.
func stageValidationReader(input io.Reader, execution queue.ValidationExecution, layer queue.ValidationLayer) (io.Reader, error) {
	if execution.RequestedProfile != queue.ValidationProfilePeppolEnvelope || layer == queue.ValidationLayerPeppolEnvelope {
		return input, nil
	}
	document, err := io.ReadAll(io.LimitReader(input, peppol.MaxResolutionDocumentBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read Peppol envelope: %w", err)
	}
	if len(document) > peppol.MaxResolutionDocumentBytes {
		return nil, peppol.ErrDocumentTooLarge
	}
	payload, err := peppol.ExtractEnvelopePayload(document)
	if err != nil {
		return nil, err
	}
	return bytes.NewReader(payload), nil
}
