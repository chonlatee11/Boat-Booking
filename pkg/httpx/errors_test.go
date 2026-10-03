package httpx

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"connectrpc.com/connect"
)

// TestWriteErrorHidesMessageForUnmappedCode is a regression test for WR-02:
// WriteError's switch only maps a handful of connect codes to a specific
// HTTP status; every other code fell through to 500 but still leaked the
// real error message, because the message-hiding check only looked at
// CodeInternal/CodeUnknown instead of the resulting status. CodeAborted is
// one such unmapped code — it must get 500 with the generic message, not
// 500 plus the real detail.
func TestWriteErrorHidesMessageForUnmappedCode(t *testing.T) {
	rec := httptest.NewRecorder()
	WriteError(rec, connect.NewError(connect.CodeAborted, errAborted))

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
	var body errorBody
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body.Message != "internal error" {
		t.Errorf("message = %q, want %q (unmapped code must not leak detail behind a 500)", body.Message, "internal error")
	}
}

// TestWriteErrorShowsMessageForMappedCode confirms the fix didn't also hide
// messages for codes that DO get a specific (non-500) status.
func TestWriteErrorShowsMessageForMappedCode(t *testing.T) {
	rec := httptest.NewRecorder()
	WriteError(rec, connect.NewError(connect.CodeNotFound, errNotFound))

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
	var body errorBody
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body.Message != "boat not found" {
		t.Errorf("message = %q, want %q", body.Message, "boat not found")
	}
}

// TestWriteErrorMapsResourceExhaustedTo429 confirms rate-limit style errors
// (D-44) surface as 429 with the message preserved.
func TestWriteErrorMapsResourceExhaustedTo429(t *testing.T) {
	rec := httptest.NewRecorder()
	WriteError(rec, connect.NewError(connect.CodeResourceExhausted, errResourceExhausted))

	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusTooManyRequests)
	}
	var body errorBody
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body.Message != "resend requested too soon" {
		t.Errorf("message = %q, want %q", body.Message, "resend requested too soon")
	}
}

// TestWriteErrorMapsFailedPreconditionTo400 confirms an expired/locked OTP
// code (D-44) surfaces as 400 with the message preserved.
func TestWriteErrorMapsFailedPreconditionTo400(t *testing.T) {
	rec := httptest.NewRecorder()
	WriteError(rec, connect.NewError(connect.CodeFailedPrecondition, errFailedPrecondition))

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
	var body errorBody
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body.Message != "code expired, locked, or already used" {
		t.Errorf("message = %q, want %q", body.Message, "code expired, locked, or already used")
	}
}

var (
	errAborted            = errString("mid-transaction detail that must not leak")
	errNotFound           = errString("boat not found")
	errResourceExhausted  = errString("resend requested too soon")
	errFailedPrecondition = errString("code expired, locked, or already used")
)

type errString string

func (e errString) Error() string { return string(e) }
