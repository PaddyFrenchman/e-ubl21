// Command verify-validation-assets verifies all pinned runtime validation
// artefacts without fetching any external resource.
package main

import (
	"flag"
	"fmt"
	"os"

	validationassets "github.com/PaddyFrenchman/e-ubl21/validation-assets"
)

func main() {
	stageDir := flag.String("stage-dir", "", "optional directory beneath which to stage verified runtime assets")
	saxonDir := flag.String("saxon-dir", "", "optional directory containing downloaded Saxon and xmlresolver JARs to verify")
	schxsltDir := flag.String("schxslt-dir", "", "optional directory containing the extracted SchXslt distribution to verify")
	flag.Parse()

	profile := validationassets.EN16931()
	var staged []string
	if *stageDir == "" {
		if err := validationassets.VerifyEN16931(); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	} else {
		var err error
		staged, err = validationassets.StageRuntimeAssets(*stageDir)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}
	if err := validationassets.VerifyPeppol(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Printf("verified profile=%s version=%s release=%s sha256=%s xslt=%s\n",
		profile.Name,
		profile.Version,
		profile.SourceRelease,
		profile.XSLTSHA256,
		profile.RuntimeXSLTPath,
	)
	peppol := validationassets.Peppol()
	compiler := validationassets.SchXslt()
	fmt.Printf("verified profile=%s version=%s publication_date=%s mandatory_from=%s artefacts=%s source_commit=%s source_archive_sha256=%s manifest_sha256=%s schematron=%s schematron_sha256=%s xslt_sha256=%s compiler=%s compiler_version=%s compiler_archive_sha256=%s compiler_metadata_sha256=%s xslt=%s\n",
		peppol.Name,
		peppol.Version,
		peppol.PublicationDate,
		peppol.MandatoryFrom,
		peppol.ValidationArtefactsVersion,
		peppol.SourceCommit,
		peppol.SourceArchiveSHA256,
		peppol.ManifestSHA256,
		peppol.SchematronPath,
		peppol.SchematronSHA256,
		peppol.XSLTSHA256,
		compiler.Name,
		compiler.Version,
		compiler.ArchiveSHA256,
		peppol.CompilerMetadataSHA256,
		peppol.RuntimeXSLTPath,
	)
	for _, path := range staged {
		fmt.Printf("staged=%s\n", path)
	}
	if *saxonDir != "" {
		if err := validationassets.VerifySaxonJARs(*saxonDir); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		saxon := validationassets.Saxon()
		fmt.Printf("verified saxon=%s xmlresolver=%s directory=%s\n", saxon.SaxonVersion, saxon.XMLResolverVersion, *saxonDir)
	}
	if *schxsltDir != "" {
		if err := validationassets.VerifySchXslt(*schxsltDir); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		compiler := validationassets.SchXslt()
		fmt.Printf("verified schematron_compiler=%s version=%s archive_sha256=%s directory=%s\n",
			compiler.Name, compiler.Version, compiler.ArchiveSHA256, *schxsltDir)
	}
}
