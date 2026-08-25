// Package validationassets contains the immutable validation artefacts and
// provenance metadata shipped by e-ubl21.
package validationassets

import (
	"crypto/sha256"
	"embed"
	"fmt"
	"io/fs"
)

// Profile identifies one pinned semantic-validation ruleset. Treat profile
// values as immutable release metadata: changing one requires the documented
// asset upgrade review and fixture suite.
type Profile struct {
	Name             string
	Version          string
	XSLTName         string
	EmbeddedXSLTPath string
	RuntimeXSLTPath  string
	SourceRepository string
	SourceRelease    string
	XSLTSHA256       string
}

// en16931 is the single source of truth for the packaged core EN 16931 UBL
// ruleset. The release tag and digest deliberately do not follow a moving
// upstream branch.
var en16931 = Profile{
	Name:             "en16931",
	Version:          "1.3.16",
	XSLTName:         "EN16931-UBL-validation.xslt",
	EmbeddedXSLTPath: "en16931/1.3.16/EN16931-UBL-validation.xslt",
	RuntimeXSLTPath:  "/opt/e-ubl21/validation-assets/en16931/1.3.16/EN16931-UBL-validation.xslt",
	SourceRepository: "ConnectingEurope/eInvoicing-EN16931",
	SourceRelease:    "validation-1.3.16",
	XSLTSHA256:       "39f9d282867f1a49e7708d9e29a53da89643e1ee56f10cec1ebcf1277595fcbd",
}

// EN16931 returns the immutable release metadata for the packaged core EN
// 16931 UBL ruleset. Returning a value prevents callers from changing the
// package's source of truth.
func EN16931() Profile {
	return en16931
}

//go:embed en16931/1.3.16/EN16931-UBL-validation.xslt
var files embed.FS

// OpenEN16931XSLT opens the exact stylesheet represented by EN16931. The
// embedded copy makes the artefact available to Go verification without any
// network or machine-local dependency.
func OpenEN16931XSLT() (fs.File, error) {
	return files.Open(en16931.EmbeddedXSLTPath)
}

// VerifyEN16931 proves that the checked-in runtime artefact has the digest
// recorded in the EN16931 profile.
func VerifyEN16931() error {
	content, err := files.ReadFile(en16931.EmbeddedXSLTPath)
	if err != nil {
		return fmt.Errorf("read %s profile %s stylesheet: %w", en16931.Name, en16931.Version, err)
	}
	actual := fmt.Sprintf("%x", sha256.Sum256(content))
	if actual != en16931.XSLTSHA256 {
		return fmt.Errorf("verify %s profile %s stylesheet: SHA-256 mismatch: expected %s, got %s", en16931.Name, en16931.Version, en16931.XSLTSHA256, actual)
	}
	return nil
}
