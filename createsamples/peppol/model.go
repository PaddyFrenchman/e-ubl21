// Package peppol builds deterministic Peppol BIS Billing candidates on top of
// the EN 16931 semantic model.
package peppol

import (
	"fmt"

	"github.com/PaddyFrenchman/e-ubl21/createsamples/en16931"
	peppolprofile "github.com/PaddyFrenchman/e-ubl21/peppol"
)

// ProcessProfile selects one supported Peppol Billing collaboration profile.
type ProcessProfile string

const (
	Profile01 ProcessProfile = "01"
	Profile02 ProcessProfile = "02"

	EndpointSchemeGLN = "0088"
	SellerGLN         = "7300010000001"
	BuyerGLN          = "7300010000018"
)

// Profiles returns the deterministic supported profile order.
func Profiles() []ProcessProfile {
	return []ProcessProfile{Profile01, Profile02}
}

// ProfileID returns the exact collaboration identifier for profile.
func ProfileID(profile ProcessProfile) (string, error) {
	switch profile {
	case Profile01:
		return peppolprofile.ProfileID01, nil
	case Profile02:
		return peppolprofile.ProfileID02, nil
	default:
		return "", fmt.Errorf("unsupported Peppol process profile %q", profile)
	}
}

// NewInvoice builds a standard-rated deterministic Invoice candidate.
func NewInvoice(profile ProcessProfile) (en16931.InvoiceDocument, error) {
	profileID, err := ProfileID(profile)
	if err != nil {
		return en16931.InvoiceDocument{}, err
	}
	document := en16931.NewInvoiceVariantS()
	document.CustomizationID = peppolprofile.CustomizationIDBase
	document.ProfileID = profileID
	document.ID = "PEPPOL-INV-P" + string(profile)
	document.BuyerReference = "BUYER-REF-P" + string(profile)
	applyPeppolParties(&document.Seller, &document.Buyer)
	return document, nil
}

// NewCreditNote builds a standard-rated deterministic CreditNote candidate.
func NewCreditNote(profile ProcessProfile) (en16931.CreditNoteDocument, error) {
	profileID, err := ProfileID(profile)
	if err != nil {
		return en16931.CreditNoteDocument{}, err
	}
	document := en16931.NewCreditNoteVariantS()
	document.CustomizationID = peppolprofile.CustomizationIDBase
	document.ProfileID = profileID
	document.ID = "PEPPOL-CN-P" + string(profile)
	document.BuyerReference = "BUYER-REF-P" + string(profile)
	applyPeppolParties(&document.Seller, &document.Buyer)
	return document, nil
}

func applyPeppolParties(seller, buyer *en16931.Party) {
	seller.EndpointID = SellerGLN
	seller.EndpointSchemeID = EndpointSchemeGLN
	seller.PartyIdentificationID = SellerGLN
	seller.PartyIdentificationSchemeID = EndpointSchemeGLN
	seller.Name = "Peppol Supplier Ltd"
	seller.StreetName = "1 Supplier Street"
	seller.CityName = "London"
	seller.PostalZone = "EC1A 1AA"
	seller.CountryCode = "GB"
	seller.VATSchemeID = "GB123456789"
	seller.LegalRegistrationID = "983294"
	seller.LegalRegistrationSchemeID = ""

	buyer.EndpointID = BuyerGLN
	buyer.EndpointSchemeID = EndpointSchemeGLN
	buyer.PartyIdentificationID = BuyerGLN
	buyer.PartyIdentificationSchemeID = EndpointSchemeGLN
	buyer.Name = "Peppol Buyer Ltd"
	buyer.StreetName = "2 Buyer Street"
	buyer.CityName = "London"
	buyer.PostalZone = "EC1A 2BB"
	buyer.CountryCode = "GB"
	buyer.VATSchemeID = ""
	buyer.LegalRegistrationID = "3993742"
	buyer.LegalRegistrationSchemeID = ""
}
