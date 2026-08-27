package en16931

import (
	"crypto/sha256"
	_ "embed"
	"fmt"
)

const (
	officialSourceVersion = "validation-1.3.16"
	officialLicense       = "European Union Public Licence (EUPL) version 1.2"
)

// OfficialFixture is one upstream EN 16931 example pinned byte-for-byte with
// the provenance needed by the common regression manifest.
type OfficialFixture struct {
	Name          string
	XML           []byte
	SourceURL     string
	SourceVersion string
	SourceSHA256  string
	License       string
}

//go:embed testdata/official/guide-example1.xml
var officialGuideExample1 []byte

//go:embed testdata/official/guide-example2.xml
var officialGuideExample2 []byte

// OfficialFixtures returns verified copies of the pinned upstream examples.
// Embedding keeps the semantic generator usable without importing server test
// packages or depending on the process working directory.
func OfficialFixtures() ([]OfficialFixture, error) {
	fixtures := []OfficialFixture{
		{
			Name: "guide-example1.xml", XML: officialGuideExample1,
			SourceURL:     "https://github.com/ConnectingEurope/eInvoicing-EN16931/blob/validation-1.3.16/ubl/examples/guide-example1.xml",
			SourceVersion: officialSourceVersion,
			SourceSHA256:  "8cf298f82ccc0b1e0a3fd57d07dd39f4c174ee6739e147020f22c8aebb364ca5",
			License:       officialLicense,
		},
		{
			Name: "guide-example2.xml", XML: officialGuideExample2,
			SourceURL:     "https://github.com/ConnectingEurope/eInvoicing-EN16931/blob/validation-1.3.16/ubl/examples/guide-example2.xml",
			SourceVersion: officialSourceVersion,
			SourceSHA256:  "7c073be56039d35d1caad07ecc407ff85910925b206a59a92fa5e3048bb891b5",
			License:       officialLicense,
		},
	}
	for index := range fixtures {
		actual := fmt.Sprintf("%x", sha256.Sum256(fixtures[index].XML))
		if actual != fixtures[index].SourceSHA256 {
			return nil, fmt.Errorf("verify official fixture %s: SHA-256 mismatch: expected %s, got %s", fixtures[index].Name, fixtures[index].SourceSHA256, actual)
		}
		fixtures[index].XML = append([]byte(nil), fixtures[index].XML...)
	}
	return fixtures, nil
}
