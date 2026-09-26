package notify

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestResendSenderSendOtp(t *testing.T) {
	var gotAuth, gotPath string
	var gotBody map[string]any

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotPath = r.URL.Path
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	sender := ResendSender{APIKey: "test-key", From: "Boat Booking <no-reply@boatbooking.local>", BaseURL: srv.URL}
	if err := sender.SendOtp(context.Background(), "someone@example.com", "123456"); err != nil {
		t.Fatalf("SendOtp: %v", err)
	}

	if gotPath != "/emails" {
		t.Errorf("path = %q, want /emails", gotPath)
	}
	if gotAuth != "Bearer test-key" {
		t.Errorf("Authorization header = %q, want %q", gotAuth, "Bearer test-key")
	}
	if gotBody["from"] != "Boat Booking <no-reply@boatbooking.local>" {
		t.Errorf("from = %v", gotBody["from"])
	}
	to, ok := gotBody["to"].([]any)
	if !ok || len(to) != 1 || to[0] != "someone@example.com" {
		t.Errorf("to = %v, want [someone@example.com]", gotBody["to"])
	}
	text, _ := gotBody["text"].(string)
	if !strings.Contains(text, "123456") {
		t.Errorf("text %q does not contain the code", text)
	}
}

func TestResendSenderNonSuccessErrorHasNoCode(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("boom 654321"))
	}))
	defer srv.Close()

	sender := ResendSender{APIKey: "test-key", From: "Boat Booking <no-reply@boatbooking.local>", BaseURL: srv.URL}
	err := sender.SendOtp(context.Background(), "someone@example.com", "123456")
	if err == nil {
		t.Fatal("SendOtp: err = nil, want non-nil on 500")
	}
	if strings.Contains(err.Error(), "123456") {
		t.Errorf("error message contains the code: %v", err)
	}
}
