package validationassets

import "testing"

func TestSchXsltPinnedMetadata(t *testing.T) {
	want := SchematronCompiler{
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
	if SchXslt() != want {
		t.Fatalf("SchXslt metadata = %#v, want %#v", SchXslt(), want)
	}

	changed := SchXslt()
	changed.Version = "changed-by-caller"
	if SchXslt() != want {
		t.Fatal("caller mutated the pinned SchXslt profile")
	}
}
