package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/PaddyFrenchman/e-ubl21/peppol"
	"github.com/PaddyFrenchman/e-ubl21/queue"
	"github.com/PaddyFrenchman/e-ubl21/queue/queuetest"
)

func directUploadHandler(t *testing.T) (*uploadHandlers, *queue.Client) {
	t.Helper()
	s := queuetest.StartServer(t)
	q, err := queue.ConnectWithOptions(s.URL, queue.DefaultNamespace, s.Options...)
	if err != nil {
		t.Fatal(err)
	}
	if err := q.EnsureInfra(context.Background()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(q.Close)
	return &uploadHandlers{
		catalog: &documentCatalog{entryByElement: map[string]schemaEntry{
			"Invoice": {file: "unused", namespace: "urn:test"},
		}},
		queue: q,
	}, q
}

func TestUploadHandlerBadRequestBranches(t *testing.T) {
	h, _ := directUploadHandler(t)
	tests := []struct {
		body, contentType string
	}{
		{"", "application/xml"},
		{"not xml", "application/xml"},
		{"<Invoice xmlns=", "application/xml"},
		{"<Invoice xmlns=\"urn:test\"/>", "text/plain"},
	}
	for _, tc := range tests {
		req := httptest.NewRequest(http.MethodPost, "/upload", strings.NewReader(tc.body))
		req.Header.Set("Content-Type", tc.contentType)
		res := httptest.NewRecorder()
		h.handleUpload(res, req)
		if res.Code != http.StatusBadRequest {
			t.Errorf("body %q: status = %d, body=%s", tc.body, res.Code, res.Body)
		}
	}
}

func TestUploadHandlerPersistsExplicitRegressionProfile(t *testing.T) {
	h, q := directUploadHandler(t)
	req := httptest.NewRequest(http.MethodPost, "/upload?profile=ubl21&documentType=Invoice", strings.NewReader(`<Invoice xmlns="urn:deliberately-invalid"/>`))
	req.Header.Set("Content-Type", "application/xml")
	res := httptest.NewRecorder()
	h.handleUpload(res, req)
	if res.Code != http.StatusAccepted {
		t.Fatalf("upload status = %d body=%s", res.Code, res.Body.String())
	}
	var accepted struct {
		ID               string                  `json:"id"`
		Profile          queue.ValidationProfile `json:"profile"`
		RequestedProfile queue.ValidationProfile `json:"requestedProfile"`
		ResolvedProfile  queue.ValidationProfile `json:"resolvedProfile"`
	}
	if err := json.Unmarshal(res.Body.Bytes(), &accepted); err != nil {
		t.Fatal(err)
	}
	record, err := q.GetUploadRecord(context.Background(), accepted.ID)
	if err != nil {
		t.Fatal(err)
	}
	if accepted.Profile != queue.ValidationProfileUBL21 || accepted.RequestedProfile != queue.ValidationProfileUBL21 ||
		accepted.ResolvedProfile != queue.ValidationProfileUBL21 || record.Profile != queue.ValidationProfileUBL21 ||
		record.RequestedProfile != queue.ValidationProfileUBL21 || record.ResolvedProfile != queue.ValidationProfileUBL21 {
		t.Fatalf("accepted/record profiles = %+v/%+v", accepted, record)
	}

	req = httptest.NewRequest(http.MethodPost, "/upload?validationProfile=peppol-bis-billing", strings.NewReader(`<Invoice xmlns="urn:test"/>`))
	req.Header.Set("Content-Type", "application/xml")
	res = httptest.NewRecorder()
	h.handleUpload(res, req)
	if res.Code != http.StatusAccepted {
		t.Fatalf("Peppol profile status = %d body=%s", res.Code, res.Body.String())
	}
	if err := json.Unmarshal(res.Body.Bytes(), &accepted); err != nil {
		t.Fatal(err)
	}
	if accepted.RequestedProfile != queue.ValidationProfilePeppolBISBilling || accepted.ResolvedProfile != queue.ValidationProfilePeppolBISBilling {
		t.Fatalf("Peppol accepted profiles = %+v", accepted)
	}
}

func TestRequestedValidationProfileResolvesAutoAndPreservesLegacyAlias(t *testing.T) {
	tests := []struct {
		name, target, documentType  string
		document                    []byte
		wantRequested, wantResolved queue.ValidationProfile
		wantError                   bool
	}{
		{
			name: "omitted auto detects Peppol", target: "/upload", documentType: "Invoice",
			document:      peppolBillingDocument("Invoice", peppol.CustomizationIDBase, peppol.ProfileID01),
			wantRequested: queue.ValidationProfileAuto, wantResolved: queue.ValidationProfilePeppolBISBilling,
		},
		{
			name: "auto falls back to EN", target: "/upload?validationProfile=auto", documentType: "Invoice",
			document:      peppolBillingDocument("Invoice", "urn:not-peppol", "urn:not-supported"),
			wantRequested: queue.ValidationProfileAuto, wantResolved: queue.ValidationProfileUBL21EN16931,
		},
		{
			name: "explicit Peppol never downgrades", target: "/upload?validationProfile=peppol-bis-billing", documentType: "Invoice",
			document:      peppolBillingDocument("Invoice", peppol.CustomizationIDBase, "urn:unsupported-profile"),
			wantRequested: queue.ValidationProfilePeppolBISBilling, wantResolved: queue.ValidationProfilePeppolBISBilling,
		},
		{
			name: "legacy query alias", target: "/upload?profile=ubl21-en16931", documentType: "Invoice",
			document:      peppolBillingDocument("Invoice", peppol.CustomizationIDBase, peppol.ProfileID01),
			wantRequested: queue.ValidationProfileUBL21EN16931, wantResolved: queue.ValidationProfileUBL21EN16931,
		},
		{name: "conflicting aliases", target: "/upload?validationProfile=ubl21&profile=ubl21-en16931", documentType: "Invoice", wantError: true},
		{name: "inapplicable Peppol", target: "/upload?validationProfile=peppol-bis-billing", documentType: "Order", wantError: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodPost, test.target, nil)
			resolution, err := requestedValidationProfile(request, test.documentType, test.document)
			if (err != nil) != test.wantError {
				t.Fatalf("resolution = %+v, err=%v", resolution, err)
			}
			if err == nil && (resolution.RequestedProfile != test.wantRequested || resolution.ResolvedProfile != test.wantResolved) {
				t.Fatalf("resolution = %+v", resolution)
			}
		})
	}
}

func TestPeppolHTTPSelectionRequiresSplitAndFreezesThreeStageProfile(t *testing.T) {
	handler, client := directUploadHandler(t)
	handler.deploymentMode = func(context.Context) (queue.DeploymentMode, error) {
		return queue.DeploymentModeLegacy, nil
	}
	document := string(peppolBillingDocument("Invoice", peppol.CustomizationIDBase, peppol.ProfileID01))
	request := httptest.NewRequest(http.MethodPost, "/upload?validationProfile=auto&documentType=Invoice", strings.NewReader(document))
	request.Header.Set("Content-Type", "application/xml")
	response := httptest.NewRecorder()
	handler.handleUpload(response, request)
	if response.Code != http.StatusServiceUnavailable || !strings.Contains(response.Body.String(), "split validation deployment") {
		t.Fatalf("legacy Peppol response = %d %s", response.Code, response.Body.String())
	}
	keys, err := client.UploadRecordKeys(context.Background())
	if err != nil || len(keys) != 0 {
		t.Fatalf("legacy Peppol request persisted uploads = %v, err=%v", keys, err)
	}

	var accepted queue.UploadRecord
	handler.deploymentMode = func(context.Context) (queue.DeploymentMode, error) {
		return queue.DeploymentModeSplit, nil
	}
	handler.publishWork = func(_ context.Context, record queue.UploadRecord, _ time.Time) error {
		accepted = record
		return nil
	}
	request = httptest.NewRequest(http.MethodPost, "/upload?validationProfile=auto&documentType=Invoice", strings.NewReader(document))
	request.Header.Set("Content-Type", "application/xml")
	response = httptest.NewRecorder()
	handler.handleUpload(response, request)
	if response.Code != http.StatusAccepted || accepted.RequestedProfile != queue.ValidationProfileAuto ||
		accepted.ResolvedProfile != queue.ValidationProfilePeppolBISBilling || accepted.Profile != accepted.ResolvedProfile {
		t.Fatalf("split Peppol acceptance = %d record=%+v body=%s", response.Code, accepted, response.Body.String())
	}
	_, plan, err := FrozenValidationPlanFor(accepted.DocumentType, accepted.ResolvedProfile)
	if err != nil || len(plan.Stages) != 3 || plan.Stages[2].ID != queue.ValidationLayerPeppolBISBilling {
		t.Fatalf("split Peppol plan = %+v, err=%v", plan, err)
	}
}

func TestUploadHandlerStorageFailuresAndContentResponse(t *testing.T) {
	h, q := directUploadHandler(t)
	q.Close()
	req := httptest.NewRequest(http.MethodPost, "/upload", strings.NewReader(`<Invoice xmlns="urn:test"/>`))
	req.Header.Set("Content-Type", "application/xml")
	res := httptest.NewRecorder()
	h.handleUpload(res, req)
	if res.Code != http.StatusInternalServerError {
		t.Fatalf("closed queue upload status = %d", res.Code)
	}

	h, q = directUploadHandler(t)
	id, _ := queue.NewID()
	validatedAt := time.Now().UTC().Format(time.RFC3339)
	record := queue.UploadRecord{
		ID: id, DocumentType: "Invoice", Status: queue.StatusInvalid, Error: "bad",
		RequestedProfile: queue.ValidationProfileAuto, ResolvedProfile: queue.ValidationProfileUBL21EN16931,
		Profile:    queue.ValidationProfileUBL21EN16931,
		ReceivedAt: validatedAt, ValidatedAt: validatedAt,
		ValidationStages: queue.ValidationStages{
			{Layer: queue.ValidationLayerUBL21XSD, Version: "2.1", Valid: true, ValidatedAt: validatedAt},
			{
				Layer: queue.ValidationLayerEN16931, Version: "1.3.16", Valid: false, ValidatedAt: validatedAt,
				Issues: []queue.ValidationIssue{{
					Kind: queue.ValidationIssueFailedAssert, Layer: queue.ValidationLayerEN16931,
					Rule: "BR-CO-13", Severity: "fatal", Location: "/ubl:Invoice/cac:LegalMonetaryTotal",
					Message: "Invoice totals do not reconcile.",
				}},
			},
		},
	}
	if err := q.PutUploadRecord(context.Background(), record); err != nil {
		t.Fatal(err)
	}
	if err := q.PutUploadContent(context.Background(), id, []byte("<Invoice/>")); err != nil {
		t.Fatal(err)
	}
	req = httptest.NewRequest(http.MethodGet, "/upload?includeContent=true", nil)
	req.SetPathValue("id", id)
	res = httptest.NewRecorder()
	h.handleGetUpload(res, req)
	if res.Code != http.StatusOK || !strings.Contains(res.Body.String(), `"content":"\u003cInvoice/\u003e"`) || !strings.Contains(res.Body.String(), `"validatedAt"`) {
		t.Fatalf("content response = %d %s", res.Code, res.Body)
	}
	var response struct {
		RequestedProfile queue.ValidationProfile `json:"requestedProfile"`
		ResolvedProfile  queue.ValidationProfile `json:"resolvedProfile"`
		Profile          queue.ValidationProfile `json:"profile"`
		ValidationStages queue.ValidationStages  `json:"validationStages"`
	}
	if err := json.Unmarshal(res.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response.RequestedProfile != queue.ValidationProfileAuto ||
		response.ResolvedProfile != queue.ValidationProfileUBL21EN16931 || response.Profile != response.ResolvedProfile {
		t.Fatalf("status profiles = %+v", response)
	}
	semantic, ok := response.ValidationStages.Find(queue.ValidationLayerEN16931)
	if !ok || semantic.Version != "1.3.16" || semantic.Valid || len(semantic.Issues) != 1 ||
		semantic.Issues[0].Rule != "BR-CO-13" || semantic.Issues[0].Severity != "fatal" {
		t.Fatalf("structured EN 16931 response = %+v", semantic)
	}

	expiredID, _ := queue.NewID()
	expiredAt := time.Now().UTC().Format(time.RFC3339)
	if err := q.PutUploadRecord(context.Background(), queue.UploadRecord{
		ID: expiredID, DocumentType: "Invoice", Status: queue.StatusValid,
		ReceivedAt: expiredAt, ValidatedAt: expiredAt,
		ContentExpiresAt: expiredAt, ContentExpiredAt: expiredAt,
	}); err != nil {
		t.Fatal(err)
	}
	req = httptest.NewRequest(http.MethodGet, "/upload", nil)
	req.SetPathValue("id", expiredID)
	res = httptest.NewRecorder()
	h.handleGetUpload(res, req)
	if res.Code != http.StatusOK || !strings.Contains(res.Body.String(), `"contentAvailable":false`) || !strings.Contains(res.Body.String(), `"contentExpiredAt"`) {
		t.Fatalf("expired metadata response = %d %s", res.Code, res.Body)
	}
	req = httptest.NewRequest(http.MethodGet, "/upload?includeContent=true", nil)
	req.SetPathValue("id", expiredID)
	res = httptest.NewRecorder()
	h.handleGetUpload(res, req)
	if res.Code != http.StatusGone || !strings.Contains(res.Body.String(), "upload content has expired") {
		t.Fatalf("expired content response = %d %s", res.Code, res.Body)
	}

	deadlinePassedID, _ := queue.NewID()
	if err := q.PutUploadRecord(context.Background(), queue.UploadRecord{
		ID: deadlinePassedID, DocumentType: "Invoice", Status: queue.StatusValid,
		ReceivedAt: expiredAt, ValidatedAt: expiredAt,
		ContentExpiresAt: time.Now().Add(-time.Minute).UTC().Format(time.RFC3339),
	}); err != nil {
		t.Fatal(err)
	}
	req = httptest.NewRequest(http.MethodGet, "/upload?includeContent=true", nil)
	req.SetPathValue("id", deadlinePassedID)
	res = httptest.NewRecorder()
	h.handleGetUpload(res, req)
	if res.Code != http.StatusGone {
		t.Fatalf("due content missing before sweep response = %d %s", res.Code, res.Body)
	}

	missingID, _ := queue.NewID()
	if err := q.PutUploadRecord(context.Background(), queue.UploadRecord{ID: missingID}); err != nil {
		t.Fatal(err)
	}
	req = httptest.NewRequest(http.MethodGet, "/upload?includeContent=true", nil)
	req.SetPathValue("id", missingID)
	res = httptest.NewRecorder()
	h.handleGetUpload(res, req)
	if res.Code != http.StatusInternalServerError {
		t.Fatalf("missing content status = %d", res.Code)
	}
}

func TestUploadHandlerRecordPublishAndReadFailures(t *testing.T) {
	makeRequest := func(h *uploadHandlers) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "/upload", strings.NewReader(`<Invoice xmlns="urn:test"/>`))
		req.Header.Set("Content-Type", "application/xml")
		res := httptest.NewRecorder()
		h.handleUpload(res, req)
		return res
	}

	h, q := directUploadHandler(t)
	if err := q.JS.DeleteKeyValue(context.Background(), q.Namespace.UploadsBucketName()); err != nil {
		t.Fatal(err)
	}
	if res := makeRequest(h); res.Code != http.StatusInternalServerError || !strings.Contains(res.Body.String(), "persist upload intent") {
		t.Fatalf("record failure = %d %s", res.Code, res.Body)
	}

	h, q = directUploadHandler(t)
	if err := q.JS.DeleteStream(context.Background(), q.Namespace.DocumentsStreamName()); err != nil {
		t.Fatal(err)
	}
	if res := makeRequest(h); res.Code != http.StatusAccepted || !strings.Contains(res.Body.String(), string(queue.StatusEnqueuePending)) {
		t.Fatalf("publish failure = %d %s", res.Code, res.Body)
	}

	h, q = directUploadHandler(t)
	q.Close()
	req := httptest.NewRequest(http.MethodGet, "/upload", nil)
	req.SetPathValue("id", "00000000000000000000000000000000")
	res := httptest.NewRecorder()
	h.handleGetUpload(res, req)
	if res.Code != http.StatusInternalServerError {
		t.Fatalf("read failure = %d %s", res.Code, res.Body)
	}
}
