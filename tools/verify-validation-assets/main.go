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
	flag.Parse()

	profile := validationassets.EN16931()
	staged := ""
	if *stageDir == "" {
		if err := validationassets.VerifyEN16931(); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	} else {
		var err error
		staged, err = validationassets.StageEN16931(*stageDir)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}
	fmt.Printf("verified profile=%s version=%s release=%s sha256=%s xslt=%s\n",
		profile.Name,
		profile.Version,
		profile.SourceRelease,
		profile.XSLTSHA256,
		profile.RuntimeXSLTPath,
	)
	if staged != "" {
		fmt.Printf("staged=%s\n", staged)
	}
}
