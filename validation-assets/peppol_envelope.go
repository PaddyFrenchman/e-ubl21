package validationassets

import (
	"crypto/sha256"
	"fmt"
	"io/fs"
	"path"
)

const (
	peppolEnvelopeRoot       = "peppol-envelope/3.0.21"
	peppolEnvelopeRulesPath  = peppolEnvelopeRoot + "/rules/PeppolBIS-Envelope-Rules.sch"
	PeppolEnvelopeXSLTSHA256 = "46866949b94aabdc873622d5d4fec9be41903e20845a1587d6501c7b19ad1d6e"
)

// PeppolEnvelopeVersion identifies the independently packaged optional SBDH
// conformity rules. They are deliberately not part of payload validation.
const PeppolEnvelopeVersion = "3.0.21"

// PeppolEnvelopeRuntimeXSLTPath is built from the pinned source using the
// same deterministic compiler as the payload rules and installed in the
// production image. It is intentionally a distinct artefact from payload
// Peppol validation.
var PeppolEnvelopeRuntimeXSLTPath = path.Join("/opt/e-ubl21/validation-assets", peppolEnvelopeRoot, "runtime", "PeppolBIS-Envelope-Rules.xslt")

func OpenPeppolEnvelopeSchematron() (fs.File, error) { return files.Open(peppolEnvelopeRulesPath) }

func VerifyPeppolEnvelope() error {
	b, err := files.ReadFile(peppolEnvelopeRulesPath)
	if err != nil {
		return err
	}
	if got := fmt.Sprintf("%x", sha256.Sum256(b)); got != "96648791fb824b6b58ddae9cc5575e8298e0061fc43dd0934a0b1459396d76f9" {
		return fmt.Errorf("Peppol envelope rules SHA-256 mismatch: %s", got)
	}
	return nil
}
