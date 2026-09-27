package validationassets

import "testing"

func TestVerifyPeppolEnvelope(t *testing.T) {
	if err := VerifyPeppolEnvelope(); err != nil {
		t.Fatal(err)
	}
	f, err := OpenPeppolEnvelopeSchematron()
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
}
