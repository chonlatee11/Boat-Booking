package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"mime"
	"net/http"
	"net/http/httptest"
	"net/mail"
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

// TestOtpMessageIsUTF8MIME proves otpMessage declares UTF-8 (MIME-Version,
// Content-Type with charset=UTF-8, Content-Transfer-Encoding: 8bit) and
// RFC 2047 encodes the Thai Subject, so Mailpit renders the Thai body
// correctly instead of the mojibake from gap G-02-3.
func TestOtpMessageIsUTF8MIME(t *testing.T) {
	from := "Boat Booking <no-reply@boatbooking.local>"
	to := "someone@example.com"
	const subject = "รหัสเข้าสู่ระบบ Boat Booking / Your Boat Booking login code"
	code := "123456"

	raw := otpMessage(from, to, subject, code)

	msg, err := mail.ReadMessage(bytes.NewReader(raw))
	if err != nil {
		t.Fatalf("mail.ReadMessage: %v", err)
	}

	if got := msg.Header.Get("MIME-Version"); got != "1.0" {
		t.Errorf("MIME-Version = %q, want 1.0", got)
	}

	mediaType, params, err := mime.ParseMediaType(msg.Header.Get("Content-Type"))
	if err != nil {
		t.Fatalf("mime.ParseMediaType: %v", err)
	}
	if mediaType != "text/plain" {
		t.Errorf("media type = %q, want text/plain", mediaType)
	}
	if !strings.EqualFold(params["charset"], "UTF-8") {
		t.Errorf("charset = %q, want UTF-8", params["charset"])
	}

	if got := msg.Header.Get("Content-Transfer-Encoding"); got != "8bit" {
		t.Errorf("Content-Transfer-Encoding = %q, want 8bit", got)
	}

	rawSubject := msg.Header.Get("Subject")
	for i := 0; i < len(rawSubject); i++ {
		if rawSubject[i] >= 0x80 {
			t.Fatalf("raw Subject header contains a non-ASCII byte at %d: %q", i, rawSubject)
		}
	}

	dec := new(mime.WordDecoder)
	decodedSubject, err := dec.DecodeHeader(rawSubject)
	if err != nil {
		t.Fatalf("DecodeHeader: %v", err)
	}
	if decodedSubject != subject {
		t.Errorf("decoded subject = %q, want %q", decodedSubject, subject)
	}

	bodyBytes, err := io.ReadAll(msg.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	body := string(bodyBytes)
	if !strings.Contains(body, "รหัสของคุณ") {
		t.Errorf("body does not contain Thai phrase: %q", body)
	}
	if !strings.Contains(body, code) {
		t.Errorf("body does not contain code %q: %q", code, body)
	}

	if got := msg.Header.Get("From"); got != from {
		t.Errorf("From = %q, want %q", got, from)
	}
	if got := msg.Header.Get("To"); got != to {
		t.Errorf("To = %q, want %q", got, to)
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
