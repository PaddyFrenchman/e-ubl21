package main

import (
	"crypto/sha256"
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/PaddyFrenchman/e-ubl21/createsamples/en16931"
	peppolsamples "github.com/PaddyFrenchman/e-ubl21/createsamples/peppol"
	validationassets "github.com/PaddyFrenchman/e-ubl21/validation-assets"
)

func generatePeppolNegativeCorpus(xsdDir, generatorVersion string) (generatedCorpus, error) {
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
	negative, err := peppolsamples.NegativeInvoiceFixtures()
	if err != nil {
		return generatedCorpus{}, err
	}
	fixtures := make([]generatedFixture, 0, len(negative))
	for _, fixture := range negative {
		content := []byte(strings.TrimRight(fixture.XML, "\n") + "\n")
		ruleIDs := append([]string(nil), fixture.ExpectedRuleIDs...)
		sort.Strings(ruleIDs)
		fixtures = append(fixtures, generatedFixture{
			entry: fixtureManifestEntry{
				ID: "peppol.invoice." + fixture.ID, DocumentType: "Invoice",
				Profile:    "peppol-bis-billing",
				File:       filepath.ToSlash(filepath.Join("negative", "Invoice", fixture.ID+".xml")),
				SHA256:     fmt.Sprintf("%x", sha256.Sum256(content)),
				Standards:  fixtureStandards{UBL: "2.1", EN16931: &enVersion, Peppol: &bisVersion},
				Provenance: provenance, Rulesets: rulesets,
				Mutation:      &fixtureMutation{ID: fixture.ID, Description: fixture.Mutation},
				ExpectedFinal: "invalid",
				ExpectedStages: []fixtureExpectedStage{
					{Stage: "ubl21-xsd", Outcome: "passed"},
					{Stage: "en16931", Outcome: "passed"},
					{Stage: "peppol-bis-billing", Outcome: "standards-invalid", RuleIDs: ruleIDs},
				},
				Tags: []string{"invoice", "negative", "peppol", fixture.Family, fixture.ID},
			},
			content:    content,
			schemaPath: filepath.Join(xsdDir, "maindoc", "UBL-Invoice-2.1.xsd"),
		})
	}
	return buildGeneratedCorpus(fixtures)
}

type peppolPositiveVariant struct {
	id             string
	documentType   string
	fileName       string
	processProfile peppolsamples.ProcessProfile
	build          func() (string, error)
}

type peppolOfficialExampleMetadata struct {
	documentType   string
	processProfile peppolsamples.ProcessProfile
	tags           []string
}

var peppolOfficialExamples = map[string]peppolOfficialExampleMetadata{
	"Allowance-example.xml": {
		documentType: "Invoice", processProfile: peppolsamples.Profile01,
		tags: []string{"allowance", "attachment", "charge", "eas-0002", "eas-0088", "payment-30", "reference", "vat-s"},
	},
	"Vat-category-S.xml": {
		documentType: "Invoice", processProfile: peppolsamples.Profile01,
		tags: []string{"allowance", "charge", "eas-0002", "eas-0088", "payment-30", "vat-s"},
	},
	"base-creditnote-correction.xml": {
		documentType: "CreditNote", processProfile: peppolsamples.Profile01,
		tags: []string{"billing-reference", "eas-0002", "eas-0088", "payment-30", "vat-s"},
	},
	"base-example.xml": {
		documentType: "Invoice", processProfile: peppolsamples.Profile01,
		tags: []string{"allowance", "eas-0002", "eas-0088", "payment-30", "vat-s"},
	},
	"base-example_profile02.xml": {
		documentType: "Invoice", processProfile: peppolsamples.Profile02,
		tags: []string{"allowance", "eas-0002", "eas-0088", "payment-30", "vat-s"},
	},
	"base-negative-inv-correction.xml": {
		documentType: "Invoice", processProfile: peppolsamples.Profile01,
		tags: []string{"allowance", "billing-reference", "eas-0002", "eas-0088", "payment-30", "vat-s"},
	},
	"sales-order-example.xml": {
		documentType: "Invoice", processProfile: peppolsamples.Profile01,
		tags: []string{"allowance", "eas-0002", "eas-0088", "order-reference", "payment-30", "vat-s"},
	},
	"vat-category-E.xml": {
		documentType: "Invoice", processProfile: peppolsamples.Profile01,
		tags: []string{"country-dk", "eas-0088", "eas-0184", "payment-30", "vat-e"},
	},
	"vat-category-O.xml": {
		documentType: "Invoice", processProfile: peppolsamples.Profile01,
		tags: []string{"country-no", "country-se", "eas-0088", "eas-0192", "payment-30", "vat-o"},
	},
	"vat-category-Z.xml": {
		documentType: "Invoice", processProfile: peppolsamples.Profile01,
		tags: []string{"country-dk", "eas-0088", "eas-0184", "payment-30", "vat-z"},
	},
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

	fixtures := make([]generatedFixture, 0, len(variants)+len(peppolOfficialExamples))
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

	official, err := validationassets.PeppolExamples()
	if err != nil {
		return generatedCorpus{}, err
	}
	for _, example := range official {
		metadata, ok := peppolOfficialExamples[example.Name]
		if !ok {
			return generatedCorpus{}, fmt.Errorf("official Peppol example %s lacks corpus metadata", example.Name)
		}
		base := strings.TrimSuffix(example.Name, filepath.Ext(example.Name))
		tags := []string{lowerDocumentType(metadata.documentType), "official", "peppol", "positive", "profile-" + string(metadata.processProfile)}
		tags = append(tags, metadata.tags...)
		sort.Strings(tags)
		content := append([]byte(nil), example.XML...)
		sourceMember := "rules/examples/" + example.Name
		fixtures = append(fixtures, generatedFixture{
			entry: fixtureManifestEntry{
				ID:           "peppol.official." + lowerDocumentType(metadata.documentType) + "." + strings.ToLower(strings.ReplaceAll(base, "_", "-")),
				DocumentType: metadata.documentType, Profile: "peppol-bis-billing",
				File:      filepath.ToSlash(filepath.Join("official", metadata.documentType, "OpenPeppol-"+example.Name)),
				SHA256:    example.SourceSHA256,
				Standards: fixtureStandards{UBL: "2.1", EN16931: &enVersion, Peppol: &bisVersion},
				Provenance: fixtureProvenance{
					Kind: "official-import", Publisher: "OpenPeppol",
					SourceURL: example.SourceURL, SourceVersion: example.SourceVersion,
					SourceSHA256: example.SourceSHA256, SourceMember: sourceMember,
					Extraction: "Vendored byte-for-byte from the pinned upstream commit.", License: example.License,
				},
				Rulesets: rulesets, ExpectedFinal: "valid",
				ExpectedStages: []fixtureExpectedStage{
					{Stage: "ubl21-xsd", Outcome: "passed"},
					{Stage: "en16931", Outcome: "passed"},
					{Stage: "peppol-bis-billing", Outcome: "passed"},
				},
				Tags: tags,
			},
			content:    content,
			schemaPath: filepath.Join(xsdDir, "maindoc", "UBL-"+metadata.documentType+"-2.1.xsd"),
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
