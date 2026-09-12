package validationassets

import (
	"crypto/sha256"
	"fmt"
	"io/fs"
)

// PeppolEnvelopeProfile versions the envelope rules independently from the
// payload rules. The Schematron bytes are from the same immutable upstream
// Peppol BIS Billing release, while the stage version identifies this product
// contract (official BR-SBDH rules plus document/process consistency checks).
type PeppolEnvelopeProfile struct {
	Name, Version, BISVersion                                        string
	SourceRepository, SourceCommit, SchematronPath, SchematronSHA256 string
}

var peppolEnvelope = PeppolEnvelopeProfile{
	Name: "peppol-envelope", Version: "3.0.21-envelope.1", BISVersion: "3.0.21",
	SourceRepository: "OpenPEPPOL/peppol-bis-invoice-3",
	SourceCommit:     "806866bd2bd91d7e9623b68f08164e8fbe9e67a0",
	SchematronPath:   "peppol-envelope/3.0.21/rules/PeppolBIS-Envelope-Rules.sch",
	SchematronSHA256: "96648791fb824b6b58ddae9cc5575e8298e0061fc43dd0934a0b1459396d76f9",
}

func PeppolEnvelope() PeppolEnvelopeProfile { return peppolEnvelope }

func OpenPeppolEnvelopeSchematron() (fs.File, error) {
	return files.Open(peppolEnvelope.SchematronPath)
}

func VerifyPeppolEnvelope() error {
	content, err := files.ReadFile(peppolEnvelope.SchematronPath)
	if err != nil {
		return fmt.Errorf("read Peppol envelope Schematron: %w", err)
	}
	actual := fmt.Sprintf("%x", sha256.Sum256(content))
	if actual != peppolEnvelope.SchematronSHA256 {
		return fmt.Errorf("verify Peppol envelope Schematron: SHA-256 mismatch: expected %s, got %s", peppolEnvelope.SchematronSHA256, actual)
	}
	return nil
}
