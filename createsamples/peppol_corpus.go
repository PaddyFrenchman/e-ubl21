package main

import (
	"crypto/sha256"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/PaddyFrenchman/e-ubl21/createsamples/en16931"
	peppolsamples "github.com/PaddyFrenchman/e-ubl21/createsamples/peppol"
	validationassets "github.com/PaddyFrenchman/e-ubl21/validation-assets"
)

type peppolPositiveVariant struct {
	id             string
	documentType   string
	fileName       string
	processProfile peppolsamples.ProcessProfile
	build          func() (string, error)
}

func generatePeppolPositiveCorpus(xsdDir, generatorVersion string) (generatedCorpus, error) {
	if err := validationassets.VerifyEN16931(); err != nil {
		return generatedCorpus{}, err
	}
	if err := validationassets.VerifyPeppol(); err != nil {
		return generatedCorpus{}, err
	}
	enProfile := validationassets.EN16931()
	peppolProfile := validationassets.Peppol()
	enVersion := enProfile.Version
	bisVersion := peppolProfile.Version
	rulesets := &fixtureRulesets{
		EN16931: &fixtureRuleset{Version: enVersion, SHA256: enProfile.XSLTSHA256},
		Peppol: &fixtureRuleset{
			Version: peppolProfile.ValidationArtefactsVersion,
			SHA256:  peppolProfile.XSLTSHA256,
		},
	}
	provenance := fixtureProvenance{
		Kind: "generated", Generator: "createsamples/peppol", GeneratorVersion: generatorVersion,
	}

	variants := make([]peppolPositiveVariant, 0, 4)
	for _, processProfile := range peppolsamples.Profiles() {
		processProfile := processProfile
		suffix := "profile-" + string(processProfile)
		variants = append(variants,
			peppolPositiveVariant{
				id: "peppol.creditnote." + suffix, documentType: "CreditNote",
				fileName:       "Peppol-CreditNote-Profile-" + string(processProfile) + ".xml",
				processProfile: processProfile,
				build: func() (string, error) {
					document, err := peppolsamples.NewCreditNote(processProfile)
					if err != nil {
						return "", err
					}
					return en16931.BuildCreditNoteXML(document)
				},
			},
			peppolPositiveVariant{
				id: "peppol.invoice." + suffix, documentType: "Invoice",
				fileName:       "Peppol-Invoice-Profile-" + string(processProfile) + ".xml",
				processProfile: processProfile,
				build: func() (string, error) {
					document, err := peppolsamples.NewInvoice(processProfile)
					if err != nil {
						return "", err
					}
					return en16931.BuildInvoiceXML(document)
				},
			},
		)
	}

	fixtures := make([]generatedFixture, 0, len(variants))
	for _, variant := range variants {
		contentText, err := variant.build()
		if err != nil {
			return generatedCorpus{}, fmt.Errorf("build %s: %w", variant.id, err)
		}
		content := []byte(strings.TrimRight(contentText, "\n") + "\n")
		processTag := "profile-" + string(variant.processProfile)
		fixtures = append(fixtures, generatedFixture{
			entry: fixtureManifestEntry{
				ID: variant.id, DocumentType: variant.documentType, Profile: "peppol-bis-billing",
				File:   filepath.ToSlash(filepath.Join(variant.documentType, variant.fileName)),
				SHA256: fmt.Sprintf("%x", sha256.Sum256(content)),
				Standards: fixtureStandards{
					UBL: "2.1", EN16931: &enVersion, Peppol: &bisVersion,
				},
				Provenance:    provenance,
				Rulesets:      rulesets,
				ExpectedFinal: "valid",
				ExpectedStages: []fixtureExpectedStage{
					{Stage: "ubl21-xsd", Outcome: "passed"},
					{Stage: "en16931", Outcome: "passed"},
					{Stage: "peppol-bis-billing", Outcome: "passed"},
				},
				Tags: []string{
					lowerDocumentType(variant.documentType), "peppol", "positive", processTag,
				},
			},
			content: content,
			schemaPath: filepath.Join(
				xsdDir, "maindoc", "UBL-"+variant.documentType+"-2.1.xsd",
			),
		})
	}
	return buildGeneratedCorpus(fixtures)
}

func lowerDocumentType(documentType string) string {
	switch documentType {
	case "CreditNote":
		return "creditnote"
	case "Invoice":
		return "invoice"
	default:
		return documentType
	}
}
