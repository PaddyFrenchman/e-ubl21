package validationassets

import (
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
)

// SaxonRuntime identifies the exact local processor dependencies used for EN
// 16931 transforms. Paths are derived from the pinned versions so consumers do
// not independently select a JAR release.
type SaxonRuntime struct {
	SaxonVersion        string
	XMLResolverVersion  string
	SaxonJARName        string
	XMLResolverJARName  string
	SaxonJARPath        string
	XMLResolverJARPath  string
	ConfigurationPath   string
	SaxonJARSHA256      string
	XMLResolverSHA256   string
	ConfigurationSHA256 string
}

var saxon = newSaxonRuntime("12.7", "6.0.17")

func newSaxonRuntime(saxonVersion, resolverVersion string) SaxonRuntime {
	const runtimeDir = "/opt/e-ubl21/saxon"
	saxonName := "saxon-he-" + saxonVersion + ".jar"
	resolverName := "xmlresolver-" + resolverVersion + ".jar"
	return SaxonRuntime{
		SaxonVersion:        saxonVersion,
		XMLResolverVersion:  resolverVersion,
		SaxonJARName:        saxonName,
		XMLResolverJARName:  resolverName,
		SaxonJARPath:        path.Join(runtimeDir, saxonName),
		XMLResolverJARPath:  path.Join(runtimeDir, resolverName),
		ConfigurationPath:   "/opt/e-ubl21/validation-assets/saxon/saxon-config.xml",
		SaxonJARSHA256:      "c607e8b395d4ee237495ad163183971ca3453d491534c847ebb55e0462772086",
		XMLResolverSHA256:   "b52e89a81fe86161bf2422997c041856791b3a78dd13824d958170b443b88c56",
		ConfigurationSHA256: "ac0c9071bb5b4040029ac009a54183bebec1399dcdbbdbaa27591fbe9d2bbe4a",
	}
}

// Saxon returns immutable metadata for the packaged Saxon runtime.
func Saxon() SaxonRuntime {
	return saxon
}

// VerifySaxonJARs verifies the downloaded build artefacts before the container
// copies them into its runtime stage.
func VerifySaxonJARs(root string) error {
	if root == "" {
		return fmt.Errorf("verify Saxon JARs: root is empty")
	}
	for _, jar := range []struct {
		name, checksum string
	}{
		{saxon.SaxonJARName, saxon.SaxonJARSHA256},
		{saxon.XMLResolverJARName, saxon.XMLResolverSHA256},
	} {
		jarPath := filepath.Join(root, jar.name)
		file, err := os.Open(jarPath)
		if err != nil {
			return fmt.Errorf("verify %s: %w", jar.name, err)
		}
		hash := sha256.New()
		_, copyErr := io.Copy(hash, file)
		closeErr := file.Close()
		if copyErr != nil {
			return fmt.Errorf("verify %s: %w", jar.name, copyErr)
		}
		if closeErr != nil {
			return fmt.Errorf("verify %s: %w", jar.name, closeErr)
		}
		actual := fmt.Sprintf("%x", hash.Sum(nil))
		if actual != jar.checksum {
			return fmt.Errorf("verify %s: SHA-256 mismatch: expected %s, got %s", jar.name, jar.checksum, actual)
		}
	}
	return nil
}
