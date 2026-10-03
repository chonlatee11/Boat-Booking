// Package notify sends OTP codes over email or phone. Sender is the one
// seam both channels implement (D-01) — app code never knows which
// transport delivered a code.
package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"mime"
	"net/http"
	"net/mail"
	"net/smtp"
	"strings"
	"time"
)

// Sender delivers a 6-digit OTP code to a normalised destination (email
// address or E.164 phone number). Implementations must never let code reach
// a returned error's message.
type Sender interface {
	SendOtp(ctx context.Context, to, code string) error
}

// sendSMTPMessage builds and sends one OTP message over SMTP, shared by
// SMTPSender and DevSMSSender. The SMTP envelope MAIL FROM must be a bare
// address (no display name) — from ("Boat Booking <no-reply@...>") is only
// valid in the message's From header, not the envelope command.
func sendSMTPMessage(addr, from, to, subject, code string) error {
	msg := otpMessage(from, to, subject, code)

	envelopeFrom := from
	if parsed, err := mail.ParseAddress(from); err == nil {
		envelopeFrom = parsed.Address
	}

	if err := smtp.SendMail(addr, nil, envelopeFrom, []string{to}, msg); err != nil {
		return fmt.Errorf("notify: send smtp message: %w", err)
	}
	return nil
}

// otpMessage builds the raw RFC 5322 message bytes for an OTP notification.
// Gap G-02-3: the previous version declared no MIME headers at all, so an
// undeclared body defaulted to us-ascii per RFC 2045 and Mailpit decoded the
// UTF-8 Thai bytes as Latin-1 (mojibake). This declares MIME-Version,
// Content-Type: text/plain; charset=UTF-8, and RFC 2047-encodes the Subject
// so it stays pure ASCII on the wire. Content-Transfer-Encoding is 8bit, not
// quoted-printable/base64, because this sender only ever talks to Mailpit
// (dev, D-02) -- production email goes through ResendSender's JSON API,
// which is unaffected by this bug.
func otpMessage(from, to, subject, code string) []byte {
	body := fmt.Sprintf("รหัสของคุณ / Your code: %s\r\nหมดอายุใน 5 นาที / expires in 5 minutes.\r\n", code)

	var buf bytes.Buffer
	fmt.Fprintf(&buf, "From: %s\r\n", from)
	fmt.Fprintf(&buf, "To: %s\r\n", to)
	fmt.Fprintf(&buf, "Subject: %s\r\n", mime.BEncoding.Encode("UTF-8", subject))
	buf.WriteString("MIME-Version: 1.0\r\n")
	buf.WriteString("Content-Type: text/plain; charset=UTF-8\r\n")
	buf.WriteString("Content-Transfer-Encoding: 8bit\r\n")
	buf.WriteString("\r\n")
	buf.WriteString(body)
	return buf.Bytes()
}

// SMTPSender sends OTP emails via net/smtp — the dev transport, pointed at
// Mailpit (no auth required).
type SMTPSender struct {
	Addr string
	From string
}

func (s SMTPSender) SendOtp(_ context.Context, to, code string) error {
	const subject = "รหัสเข้าสู่ระบบ Boat Booking / Your Boat Booking login code"
	return sendSMTPMessage(s.Addr, s.From, to, subject, code)
}

// DevSMSSender delivers phone OTPs to Mailpit as an email to
// "<E.164 digits without the leading +>@sms.local", so dev/CI phone OTPs
// are test-retrievable without ever appearing in a log (research Open
// Question 1). Not a real SMS provider — production phone login is
// unavailable until one is chosen (deferred per CONTEXT).
type DevSMSSender struct {
	SMTP SMTPSender
}

func (s DevSMSSender) SendOtp(_ context.Context, to, code string) error {
	addr := strings.TrimPrefix(to, "+") + "@sms.local"
	const subject = "SMS OTP (dev)"
	return sendSMTPMessage(s.SMTP.Addr, s.SMTP.From, addr, subject, code)
}

// ResendSender sends OTP emails via the Resend REST API — the prod
// transport. No SDK dependency: one POST, JSON body, bearer auth.
type ResendSender struct {
	APIKey  string
	From    string
	BaseURL string // defaults to https://api.resend.com
	Client  *http.Client
}

type resendEmailRequest struct {
	From    string   `json:"from"`
	To      []string `json:"to"`
	Subject string   `json:"subject"`
	Text    string   `json:"text"`
}

func (s ResendSender) SendOtp(ctx context.Context, to, code string) error {
	baseURL := s.BaseURL
	if baseURL == "" {
		baseURL = "https://api.resend.com"
	}
	client := s.Client
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}

	text := fmt.Sprintf("รหัสของคุณ / Your code: %s\r\nหมดอายุใน 5 นาที / expires in 5 minutes.\r\n", code)
	body, err := json.Marshal(resendEmailRequest{
		From:    s.From,
		To:      []string{to},
		Subject: "รหัสเข้าสู่ระบบ Boat Booking / Your Boat Booking login code",
		Text:    text,
	})
	if err != nil {
		return fmt.Errorf("notify: marshal resend request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/emails", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("notify: build resend request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+s.APIKey)

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("notify: resend request failed: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck // response body close, nothing actionable

	// Deliberately never include resp.Body in the error — only the status —
	// so a code can never leak back out through a delivery-failure message.
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("notify: resend returned status %d", resp.StatusCode)
	}
	return nil
}
