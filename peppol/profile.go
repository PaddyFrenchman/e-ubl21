// Package peppol resolves Peppol Billing applicability independently from
// Schematron rule execution.
package peppol

import (
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"strings"
	"unicode/utf8"

	"github.com/PaddyFrenchman/e-ubl21/queue"
)

const (
	CustomizationIDBase = "urn:cen.eu:en16931:2017#compliant#urn:fdc:peppol.eu:2017:poacc:billing:3.0"
	ProfileID01         = "urn:fdc:peppol.eu:2017:poacc:billing:01:1.0"
	ProfileID02         = "urn:peppol:bis:billing_with_response"

	MaxResolutionDocumentBytes = 10 << 20
	MaxIdentifierBytes         = 1024

	cbcNamespace        = "urn:oasis:names:specification:ubl:schema:xsd:CommonBasicComponents-2"
	invoiceNamespace    = "urn:oasis:names:specification:ubl:schema:xsd:Invoice-2"
	creditNoteNamespace = "urn:oasis:names:specification:ubl:schema:xsd:CreditNote-2"
)

// ProfileFailureCode identifies an e-ubl21 product applicability failure.
// Official PEPPOL-* rule IDs are reserved for evidence emitted by Schematron.
type ProfileFailureCode string

const (
	FailureCustomizationIDMissing     ProfileFailureCode = "EUBL21-PEPPOL-CUSTOMIZATION-ID-MISSING"
	FailureCustomizationIDAmbiguous   ProfileFailureCode = "EUBL21-PEPPOL-CUSTOMIZATION-ID-AMBIGUOUS"
	FailureCustomizationIDInvalid     ProfileFailureCode = "EUBL21-PEPPOL-CUSTOMIZATION-ID-INVALID"
	FailureCustomizationIDUnsupported ProfileFailureCode = "EUBL21-PEPPOL-CUSTOMIZATION-ID-UNSUPPORTED"
	FailureProfileIDMissing           ProfileFailureCode = "EUBL21-PEPPOL-PROFILE-ID-MISSING"
	FailureProfileIDAmbiguous         ProfileFailureCode = "EUBL21-PEPPOL-PROFILE-ID-AMBIGUOUS"
	FailureProfileIDInvalid           ProfileFailureCode = "EUBL21-PEPPOL-PROFILE-ID-INVALID"
	FailureProfileIDUnsupported       ProfileFailureCode = "EUBL21-PEPPOL-PROFILE-ID-UNSUPPORTED"
)

var (
	ErrUnsupportedProfile   = errors.New("unsupported requested validation profile")
	ErrProfileNotApplicable = errors.New("requested validation profile is not applicable")
	ErrMalformedDocument    = errors.New("cannot resolve profile from malformed XML")
	ErrDocumentTooLarge     = errors.New("document exceeds Peppol profile-resolution limit")
)

// Identifiers are normalized top-level Peppol Billing identifiers. Values are
// empty when absent, duplicated, nested, in the wrong namespace, or oversized.
type Identifiers struct {
	CustomizationID string
	ProfileID       string
	ProcessProfile  queue.PeppolProcessProfile
}

// ProfileFailure is bounded product-specific evidence from applicability
// resolution. It must never be presented as an official Schematron rule.
type ProfileFailure struct {
	Code    ProfileFailureCode `json:"code"`
	Field   string             `json:"field"`
	Message string             `json:"message"`
}

// Resolution records a requested profile, its concrete durable profile, the
// identifiers inspected during automatic or explicit Peppol resolution, and
// any fail-closed e-ubl21 product applicability failures.
type Resolution struct {
	RequestedProfile queue.ValidationProfile
	ResolvedProfile  queue.ValidationProfile
	Identifiers      Identifiers
	Failures         []ProfileFailure
}

// PeppolIdentifiersValid reports whether the resolution selected Peppol and
// all supported identifier checks passed.
func (resolution Resolution) PeppolIdentifiersValid() bool {
	return (resolution.ResolvedProfile == queue.ValidationProfilePeppolBISBilling || resolution.ResolvedProfile == queue.ValidationProfilePeppolEnvelope) &&
		len(resolution.Failures) == 0 &&
		resolution.Identifiers.CustomizationID != "" &&
		resolution.Identifiers.ProfileID != "" &&
		resolution.Identifiers.ProcessProfile != ""
}

// Resolve selects a concrete durable profile. An empty requested profile is
// the same as auto. Explicit UBL/EN requests ignore Peppol identifiers.
// Explicit Peppol never falls back: unsupported identifiers remain a Peppol
// resolution with typed product applicability failures.
func Resolve(documentType string, requested queue.ValidationProfile, document []byte) (Resolution, error) {
	if requested == "" {
		requested = queue.ValidationProfileAuto
	}
	switch requested {
	case queue.ValidationProfileAuto, queue.ValidationProfileUBL21,
		queue.ValidationProfileUBL21EN16931, queue.ValidationProfilePeppolBISBilling, queue.ValidationProfilePeppolEnvelope:
	default:
		return Resolution{}, fmt.Errorf("%w %q", ErrUnsupportedProfile, requested)
	}

	billingDocument := documentType == "Invoice" || documentType == "CreditNote"
	if !billingDocument {
		switch requested {
		case queue.ValidationProfileAuto, queue.ValidationProfileUBL21:
			return Resolution{RequestedProfile: requested, ResolvedProfile: queue.ValidationProfileUBL21}, nil
		default:
			return Resolution{}, fmt.Errorf("%w: profile %q requires Invoice or CreditNote", ErrProfileNotApplicable, requested)
		}
	}
	if requested == queue.ValidationProfileUBL21 || requested == queue.ValidationProfileUBL21EN16931 {
		return Resolution{RequestedProfile: requested, ResolvedProfile: requested}, nil
	}

	parsed, err := parseIdentifiers(documentType, document)
	if err != nil {
		return Resolution{}, err
	}
	identifiers := parsed.identifiers()
	autoPeppol := !parsed.customizationAmbiguous() &&
		strings.HasPrefix(identifiers.CustomizationID, CustomizationIDBase) &&
		identifiers.ProcessProfile != ""
	if requested == queue.ValidationProfileAuto && !autoPeppol {
		return Resolution{
			RequestedProfile: requested,
			ResolvedProfile:  queue.ValidationProfileUBL21EN16931,
			Identifiers:      identifiers,
		}, nil
	}

	failures := parsed.profileFailures()
	resolved := queue.ValidationProfilePeppolBISBilling
	if requested == queue.ValidationProfilePeppolEnvelope {
		resolved = queue.ValidationProfilePeppolEnvelope
	}
	return Resolution{
		RequestedProfile: requested,
		ResolvedProfile:  resolved,
		Identifiers:      identifiers,
		Failures:         failures,
	}, nil
}

type parsedIdentifiers struct {
	customizationID      string
	profileID            string
	customizationCount   int
	profileCount         int
	customizationInvalid bool
	profileInvalid       bool
}

func (parsed parsedIdentifiers) identifiers() Identifiers {
	identifiers := Identifiers{}
	if parsed.customizationCount == 1 && !parsed.customizationInvalid {
		identifiers.CustomizationID = parsed.customizationID
	}
	if parsed.profileCount == 1 && !parsed.profileInvalid {
		identifiers.ProfileID = parsed.profileID
		switch parsed.profileID {
		case ProfileID01:
			identifiers.ProcessProfile = queue.PeppolProcessProfile01
		case ProfileID02:
			identifiers.ProcessProfile = queue.PeppolProcessProfile02
		}
	}
	return identifiers
}

func (parsed parsedIdentifiers) customizationAmbiguous() bool {
	return parsed.customizationCount != 1 || parsed.customizationInvalid
}

func (parsed parsedIdentifiers) profileFailures() []ProfileFailure {
	identifiers := parsed.identifiers()
	var failures []ProfileFailure
	switch {
	case parsed.customizationCount == 0:
		failures = append(failures, profileFailure(FailureCustomizationIDMissing, "customizationId", "Peppol Billing requires a CustomizationID."))
	case parsed.customizationCount != 1:
		failures = append(failures, profileFailure(FailureCustomizationIDAmbiguous, "customizationId", "Peppol Billing requires one unambiguous top-level CustomizationID."))
	case parsed.customizationInvalid:
		failures = append(failures, profileFailure(FailureCustomizationIDInvalid, "customizationId", "Peppol Billing CustomizationID must be simple bounded text."))
	case !strings.HasPrefix(identifiers.CustomizationID, CustomizationIDBase) ||
		strings.Contains(identifiers.CustomizationID, "::"):
		failures = append(failures, profileFailure(FailureCustomizationIDUnsupported, "customizationId", "CustomizationID is not supported by the Peppol Billing product profile."))
	}
	switch {
	case parsed.profileCount == 0:
		failures = append(failures, profileFailure(FailureProfileIDMissing, "profileId", "Peppol Billing requires a ProfileID."))
	case parsed.profileCount != 1:
		failures = append(failures, profileFailure(FailureProfileIDAmbiguous, "profileId", "Peppol Billing requires one unambiguous top-level ProfileID."))
	case parsed.profileInvalid:
		failures = append(failures, profileFailure(FailureProfileIDInvalid, "profileId", "Peppol Billing ProfileID must be simple bounded text."))
	case identifiers.ProcessProfile == "":
		failures = append(failures, profileFailure(FailureProfileIDUnsupported, "profileId", "ProfileID is not supported by the Peppol Billing product profile."))
	}
	return failures
}

func profileFailure(code ProfileFailureCode, field, message string) ProfileFailure {
	return ProfileFailure{Code: code, Field: field, Message: message}
}

func parseIdentifiers(documentType string, document []byte) (parsedIdentifiers, error) {
	if len(document) > MaxResolutionDocumentBytes {
		return parsedIdentifiers{}, fmt.Errorf("%w: maximum is %d bytes", ErrDocumentTooLarge, MaxResolutionDocumentBytes)
	}
	expectedNamespace := invoiceNamespace
	if documentType == "CreditNote" {
		expectedNamespace = creditNoteNamespace
	}
	decoder := xml.NewDecoder(bytes.NewReader(document))
	decoder.Strict = true
	var parsed parsedIdentifiers
	rootSeen := false
	depth := 0
	for {
		token, err := decoder.Token()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return parsedIdentifiers{}, fmt.Errorf("%w: %v", ErrMalformedDocument, err)
		}
		switch value := token.(type) {
		case xml.Directive:
			return parsedIdentifiers{}, fmt.Errorf("%w: XML directives are not allowed", ErrMalformedDocument)
		case xml.StartElement:
			if !rootSeen {
				rootSeen = true
				if value.Name.Local != documentType || value.Name.Space != expectedNamespace {
					return parsedIdentifiers{}, fmt.Errorf("%w: root {%s}%s does not match %s", ErrMalformedDocument, value.Name.Space, value.Name.Local, documentType)
				}
				depth = 1
				continue
			}
			if depth == 0 {
				return parsedIdentifiers{}, fmt.Errorf("%w: multiple root elements", ErrMalformedDocument)
			}
			if depth == 1 && value.Name.Space == cbcNamespace &&
				(value.Name.Local == "CustomizationID" || value.Name.Local == "ProfileID") {
				text, invalid, err := identifierElementText(decoder, value)
				if err != nil {
					return parsedIdentifiers{}, fmt.Errorf("%w: %v", ErrMalformedDocument, err)
				}
				switch value.Name.Local {
				case "CustomizationID":
					parsed.customizationCount++
					parsed.customizationInvalid = parsed.customizationInvalid || invalid
					if parsed.customizationCount == 1 && !invalid {
						parsed.customizationID = text
					}
				case "ProfileID":
					parsed.profileCount++
					parsed.profileInvalid = parsed.profileInvalid || invalid
					if parsed.profileCount == 1 && !invalid {
						parsed.profileID = text
					}
				}
				continue
			}
			depth++
		case xml.EndElement:
			depth--
			if depth < 0 {
				return parsedIdentifiers{}, fmt.Errorf("%w: invalid element depth", ErrMalformedDocument)
			}
		case xml.CharData:
			if rootSeen && depth == 0 && strings.TrimSpace(string(value)) != "" {
				return parsedIdentifiers{}, fmt.Errorf("%w: content follows root element", ErrMalformedDocument)
			}
		}
	}
	if !rootSeen || depth != 0 {
		return parsedIdentifiers{}, fmt.Errorf("%w: incomplete document", ErrMalformedDocument)
	}
	return parsed, nil
}

func identifierElementText(decoder *xml.Decoder, start xml.StartElement) (string, bool, error) {
	var text normalizedXMLTextBuilder
	invalid := false
	depth := 1
	for {
		token, err := decoder.Token()
		if err != nil {
			return "", false, err
		}
		switch value := token.(type) {
		case xml.CharData:
			if depth == 1 && !invalid {
				if !utf8.Valid(value) || !text.Write(value) {
					invalid = true
					text = normalizedXMLTextBuilder{}
				}
			}
		case xml.StartElement:
			depth++
			invalid = true
			text = normalizedXMLTextBuilder{}
		case xml.EndElement:
			depth--
			if depth == 0 {
				if value.Name != start.Name {
					return "", false, fmt.Errorf("unexpected closing element %s", value.Name.Local)
				}
				return text.String(), invalid, nil
			}
		case xml.Directive:
			return "", false, errors.New("XML directives are not allowed")
		}
	}
}

type normalizedXMLTextBuilder struct {
	text         strings.Builder
	pendingSpace bool
	wroteText    bool
}

func (builder *normalizedXMLTextBuilder) Write(value []byte) bool {
	for _, char := range string(value) {
		switch char {
		case ' ', '\t', '\n', '\r':
			if builder.wroteText {
				builder.pendingSpace = true
			}
		default:
			addedBytes := utf8.RuneLen(char)
			if builder.pendingSpace {
				addedBytes++
			}
			if builder.text.Len()+addedBytes > MaxIdentifierBytes {
				return false
			}
			if builder.pendingSpace {
				builder.text.WriteByte(' ')
				builder.pendingSpace = false
			}
			builder.text.WriteRune(char)
			builder.wroteText = true
		}
	}
	return true
}

func (builder *normalizedXMLTextBuilder) String() string {
	return builder.text.String()
}
