package validationassets

import "testing"

func TestPeppolEnvelopeOfficialRulesArePinned(t *testing.T) {
	if err := VerifyPeppolEnvelope(); err != nil {
		t.Fatal(err)
	}
	if PeppolEnvelope().Version == Peppol().ValidationArtefactsVersion {
		t.Fatal("envelope rules must be independently versioned")
	}
}
