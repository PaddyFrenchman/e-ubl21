// Command verify-validation-assets verifies all pinned runtime validation
// artefacts without fetching any external resource.
package main

import (
	"fmt"
	"os"

	validationassets "github.com/PaddyFrenchman/e-ubl21/validation-assets"
)

func main() {
	profile := validationassets.EN16931()
	if err := validationassets.VerifyEN16931(); err != nil {
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
}
