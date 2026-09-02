package validationassets

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// SchematronCompiler identifies the immutable SchXslt distribution used to
// compile ISO Schematron into the executable XSLT shipped by e-ubl21.
type SchematronCompiler struct {
	Name                       string
	Version                    string
	ReleaseURL                 string
	ArchiveURL                 string
	ArchiveSHA256              string
	DistributionDirectory      string
	DistributionManifestPath   string
	DistributionManifestSHA256 string
	CompilerRelativePath       string
	CompileMetadataParameter   string
}

var schxslt = SchematronCompiler{
	Name:                       "SchXslt",
	Version:                    "1.10.1",
	ReleaseURL:                 "https://codeberg.org/SchXslt/schxslt/releases/tag/v1.10.1",
	ArchiveURL:                 "https://codeberg.org/SchXslt/schxslt/releases/download/v1.10.1/schxslt-1.10.1-xslt-only.zip",
	ArchiveSHA256:              "e141fabd4804a0ea527945dc7253dd38f9ef8957d9b219b94e13d16859cff7f0",
	DistributionDirectory:      "schxslt-1.10.1",
	DistributionManifestPath:   "schematron/schxslt/1.10.1/sha256.txt",
	DistributionManifestSHA256: "27d6c0549c211e8847e278fe5d53153f0d9ba0494fe5d8e83c19bd1caada1fc7",
	CompilerRelativePath:       "2.0/pipeline-for-svrl.xsl",
	CompileMetadataParameter:   "?schxslt.compile.metadata=false()",
}

// SchXslt returns immutable metadata for the pinned Schematron compiler.
func SchXslt() SchematronCompiler {
	return schxslt
}

// VerifySchXslt verifies the complete extracted compiler distribution and
// rejects missing, changed, extra, non-regular, or unsafe files.
func VerifySchXslt(root string) error {
	if root == "" {
		return fmt.Errorf("verify %s %s: root is empty", schxslt.Name, schxslt.Version)
	}
	manifest, err := files.ReadFile(schxslt.DistributionManifestPath)
	if err != nil {
		return fmt.Errorf("read %s %s manifest: %w", schxslt.Name, schxslt.Version, err)
	}
	actualManifestSHA256 := fmt.Sprintf("%x", sha256.Sum256(manifest))
	if actualManifestSHA256 != schxslt.DistributionManifestSHA256 {
		return fmt.Errorf("verify %s %s manifest: SHA-256 mismatch: expected %s, got %s", schxslt.Name, schxslt.Version, schxslt.DistributionManifestSHA256, actualManifestSHA256)
	}

	distributionRoot := filepath.Join(root, schxslt.DistributionDirectory)
	entries := make(map[string]string)
	scanner := bufio.NewScanner(strings.NewReader(string(manifest)))
	for scanner.Scan() {
		checksum, relativePath, ok := strings.Cut(scanner.Text(), "  ")
		if !ok || len(checksum) != sha256.Size*2 {
			return fmt.Errorf("verify %s %s manifest: malformed line %q", schxslt.Name, schxslt.Version, scanner.Text())
		}
		if _, err := hex.DecodeString(checksum); err != nil {
			return fmt.Errorf("verify %s %s manifest checksum for %q: %w", schxslt.Name, schxslt.Version, relativePath, err)
		}
		if relativePath == "" || relativePath != path.Clean(relativePath) || relativePath == ".." || strings.HasPrefix(relativePath, "../") || strings.HasPrefix(relativePath, "/") || strings.Contains(relativePath, "\\") {
			return fmt.Errorf("verify %s %s manifest: unsafe path %q", schxslt.Name, schxslt.Version, relativePath)
		}
		if _, exists := entries[relativePath]; exists {
			return fmt.Errorf("verify %s %s manifest: duplicate path %q", schxslt.Name, schxslt.Version, relativePath)
		}
		if err := verifyFileSHA256(filepath.Join(distributionRoot, filepath.FromSlash(relativePath)), checksum); err != nil {
			return fmt.Errorf("verify %s %s file %q: %w", schxslt.Name, schxslt.Version, relativePath, err)
		}
		entries[relativePath] = checksum
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("read %s %s manifest: %w", schxslt.Name, schxslt.Version, err)
	}

	seen := 0
	err = filepath.WalkDir(distributionRoot, func(filePath string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		relativePath, err := filepath.Rel(distributionRoot, filePath)
		if err != nil {
			return err
		}
		relativePath = filepath.ToSlash(relativePath)
		if _, ok := entries[relativePath]; !ok {
			return fmt.Errorf("unmanifested file %q", relativePath)
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("%q is not a regular file", relativePath)
		}
		seen++
		return nil
	})
	if err != nil {
		return fmt.Errorf("verify %s %s inventory: %w", schxslt.Name, schxslt.Version, err)
	}
	if seen != len(entries) {
		return fmt.Errorf("verify %s %s inventory: found %d files, manifest records %d", schxslt.Name, schxslt.Version, seen, len(entries))
	}
	return nil
}

func schxsltCompilerPath(root string) string {
	return filepath.Join(root, schxslt.DistributionDirectory, filepath.FromSlash(schxslt.CompilerRelativePath))
}

func verifyFileSHA256(filePath, expected string) error {
	content, err := os.ReadFile(filepath.Clean(filePath))
	if err != nil {
		return err
	}
	actual := fmt.Sprintf("%x", sha256.Sum256(content))
	if actual != expected {
		return fmt.Errorf("SHA-256 mismatch: expected %s, got %s", expected, actual)
	}
	return nil
}
