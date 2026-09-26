// Package notify sends OTP codes over email or phone. Sender is the one
// seam both channels implement (D-01) — app code never knows which
// transport delivered a code.
package notify

import (
	"context"
	"fmt"
	"net/mail"
	"net/smtp"
)

// Sender delivers a 6-digit OTP code to a normalised destination (email
// address or E.164 phone number). Implementations must never let code reach
// a returned error's message.
type Sender interface {
	SendOtp(ctx context.Context, to, code string) error
}

// SMTPSender sends OTP emails via net/smtp — the dev transport, pointed at
// Mailpit (no auth required).
type SMTPSender struct {
	Addr string
	From string
}

func (s SMTPSender) SendOtp(_ context.Context, to, code string) error {
	subject := "รหัสเข้าสู่ระบบ Boat Booking / Your Boat Booking login code"
	body := fmt.Sprintf("รหัสของคุณ / Your code: %s\r\nหมดอายุใน 5 นาที / expires in 5 minutes.\r\n", code)
	msg := fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\n\r\n%s", s.From, to, subject, body)

	// The SMTP envelope MAIL FROM must be a bare address (no display name) —
	// s.From ("Boat Booking <no-reply@...>") is only valid in the message's
	// From header, not the envelope command.
	envelopeFrom := s.From
	if addr, err := mail.ParseAddress(s.From); err == nil {
		envelopeFrom = addr.Address
	}

	if err := smtp.SendMail(s.Addr, nil, envelopeFrom, []string{to}, []byte(msg)); err != nil {
		return fmt.Errorf("notify: send otp email: %w", err)
	}
	return nil
}
