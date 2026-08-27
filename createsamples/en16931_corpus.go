package main

import (
	"crypto/sha256"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/PaddyFrenchman/e-ubl21/createsamples/en16931"
	validationassets "github.com/PaddyFrenchman/e-ubl21/validation-assets"
)

type en16931PositiveVariant struct {
	id           string
	documentType string
	fileName     string
	tags         []string
	build        func() (string, error)
}

func generateEN16931PositiveCorpus(xsdDir, generatorVersion string) (generatedCorpus, error) {
	if err := validationassets.VerifyEN16931(); err != nil {
		return generatedCorpus{}, err
	}
	profile := validationassets.EN16931()
	version := profile.Version
	rulesets := &fixtureRulesets{EN16931: &fixtureRuleset{Version: version, SHA256: profile.XSLTSHA256}}
	generatedProvenance := fixtureProvenance{
		Kind: "generated", Generator: "createsamples/en16931", GeneratorVersion: generatorVersion,
	}

	invoiceVariant := func(build func() en16931.InvoiceDocument) func() (string, error) {
		return func() (string, error) { return en16931.BuildInvoiceXML(build()) }
	}
	variants := []en16931PositiveVariant{
		{id: "en16931.invoice.vat-ae", documentType: "Invoice", fileName: "EN16931-Invoice-VAT-AE.xml", tags: []string{"en16931", "invoice", "positive", "vat-ae"}, build: invoiceVariant(en16931.NewInvoiceVariantAE)},
		{id: "en16931.invoice.vat-e", documentType: "Invoice", fileName: "EN16931-Invoice-VAT-E.xml", tags: []string{"en16931", "invoice", "positive", "vat-e"}, build: invoiceVariant(en16931.NewInvoiceVariantE)},
		{id: "en16931.invoice.vat-g", documentType: "Invoice", fileName: "EN16931-Invoice-VAT-G.xml", tags: []string{"en16931", "invoice", "positive", "vat-g"}, build: invoiceVariant(en16931.NewInvoiceVariantG)},
		{id: "en16931.invoice.vat-k", documentType: "Invoice", fileName: "EN16931-Invoice-VAT-K.xml", tags: []string{"en16931", "invoice", "positive", "vat-k"}, build: invoiceVariant(en16931.NewInvoiceVariantK)},
		{id: "en16931.invoice.vat-o", documentType: "Invoice", fileName: "EN16931-Invoice-VAT-O.xml", tags: []string{"en16931", "invoice", "positive", "vat-o"}, build: invoiceVariant(en16931.NewInvoiceVariantO)},
		{id: "en16931.invoice.vat-s", documentType: "Invoice", fileName: "EN16931-Invoice-VAT-S.xml", tags: []string{"en16931", "invoice", "positive", "vat-s"}, build: invoiceVariant(en16931.NewInvoiceVariantS)},
		{id: "en16931.invoice.vat-z", documentType: "Invoice", fileName: "EN16931-Invoice-VAT-Z.xml", tags: []string{"en16931", "invoice", "positive", "vat-z"}, build: invoiceVariant(en16931.NewInvoiceVariantZ)},
		{id: "en16931.creditnote.vat-s", documentType: "CreditNote", fileName: "EN16931-CreditNote-VAT-S.xml", tags: []string{"creditnote", "en16931", "positive", "vat-s"}, build: func() (string, error) { return en16931.BuildCreditNoteXML(en16931.NewCreditNoteVariantS()) }},
	}

	fixtures := make([]generatedFixture, 0, len(variants)+2)
	for _, variant := range variants {
		xml, err := variant.build()
		if err != nil {
			return generatedCorpus{}, fmt.Errorf("build %s: %w", variant.id, err)
		}
		fixtures = append(fixtures, newEN16931PositiveFixture(
			variant.id, variant.documentType, variant.fileName, []byte(xml), xsdDir,
			version, rulesets, generatedProvenance, variant.tags,
		))
	}

	official, err := en16931.OfficialFixtures()
	if err != nil {
		return generatedCorpus{}, err
	}
	for _, imported := range official {
		base := strings.TrimSuffix(imported.Name, filepath.Ext(imported.Name))
		provenance := fixtureProvenance{
			Kind: "official-import", Publisher: "ConnectingEurope/eInvoicing-EN16931",
			SourceURL: imported.SourceURL, SourceVersion: imported.SourceVersion,
			SourceSHA256: imported.SourceSHA256, SourceMember: (*string)(nil),
			Extraction: "Vendored byte-for-byte from the tagged upstream file.", License: imported.License,
		}
		fixtures = append(fixtures, newEN16931PositiveFixture(
			"en16931.invoice.official-"+base, "Invoice", "EN16931-Official-"+imported.Name,
			imported.XML, xsdDir, version, rulesets, provenance,
			[]string{"en16931", "invoice", "official", "positive"},
		))
	}
	return buildGeneratedCorpus(fixtures)
}

func newEN16931PositiveFixture(id, documentType, fileName string, content []byte, xsdDir, version string, rulesets *fixtureRulesets, provenance fixtureProvenance, tags []string) generatedFixture {
	versionCopy := version
	return generatedFixture{
		entry: fixtureManifestEntry{
			ID: id, DocumentType: documentType, Profile: "ubl21-en16931",
			File:       filepath.ToSlash(filepath.Join(documentType, fileName)),
			SHA256:     fmt.Sprintf("%x", sha256.Sum256(content)),
			Standards:  fixtureStandards{UBL: "2.1", EN16931: &versionCopy},
			Provenance: provenance, Rulesets: rulesets, ExpectedFinal: "valid",
			ExpectedStages: []fixtureExpectedStage{{Stage: "ubl21-xsd", Outcome: "passed"}, {Stage: "en16931", Outcome: "passed"}},
			Tags:           tags,
		},
		content:    content,
		schemaPath: filepath.Join(xsdDir, "maindoc", "UBL-"+documentType+"-2.1.xsd"),
	}
}
