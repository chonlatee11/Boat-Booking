// Package app holds thin use-case functions taking pgx.Tx directly — no
// repository interfaces, no mocks (D-14).
package app

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	identityv1 "github.com/chonlatee11/boat-booking/gen/go/identity/v1"
	"github.com/chonlatee11/boat-booking/pkg/auth"
	"github.com/chonlatee11/boat-booking/pkg/events"
	"github.com/chonlatee11/boat-booking/pkg/outbox"
	bbpgx "github.com/chonlatee11/boat-booking/pkg/pgx"
	"github.com/chonlatee11/boat-booking/services/identity/internal/adapters/notify"
	"github.com/chonlatee11/boat-booking/services/identity/internal/adapters/postgres"
	"github.com/chonlatee11/boat-booking/services/identity/internal/domain"
)

// EventUserCreated is the past-tense fact published the first time a
// destination completes VerifyOtp (auto-created as role customer, D-04).
// Ids and role only — no personal data (D-45).
const EventUserCreated = "identity.UserCreated"

// D-03 tuning: how long an issued code is valid, the resend cooldown, the
// per-destination hourly send cap, and the wrong-attempt lockout threshold.
const (
	otpCodeTTL     = 300 * time.Second
	otpCooldownTTL = 60 * time.Second
	otpHourlyTTL   = time.Hour
	otpHourlyLimit = 5
	otpMaxAttempts = 5
)

// Auth implements the OTP login use case (D-01, D-02, D-03). RDB is the
// Valkey client holding every OTP code/counter — no otps table exists.
// Pepper is OTP_HASH_SECRET, used to HMAC both destinations (so no raw
// email/phone appears in a Valkey key) and codes (so a Postgres/Valkey dump
// alone can never brute-force a code offline, Pitfall 4).
type Auth struct {
	Pool   *pgxpool.Pool
	RDB    *redis.Client
	Pepper []byte
	Email  notify.Sender
	SMS    notify.Sender
	Issuer *auth.Issuer
	Nudge  func()
}

// RequestOtp normalises raw, enforces the D-03 send-rate rules (60s cooldown,
// 5/hour cap), generates a fresh 6-digit code, stores its HMAC hash in
// Valkey with a 300s TTL, and sends it over the channel matching the
// destination's kind. Once past the rate limits it always returns the same
// nil result whether or not a user exists for destination (Pitfall 3 — no
// account enumeration on the request path; no user lookup happens here).
func (a *Auth) RequestOtp(ctx context.Context, raw string) error {
	dest, err := domain.NormalizeDestination(raw)
	if err != nil {
		return err
	}

	dh := hashDestination(a.Pepper, dest.Value)
	codeKey := otpCodeKey(dh)
	cooldownKey := otpCooldownKey(dh)
	hourlyKey := otpHourlyKey(dh)

	won, err := a.RDB.SetNX(ctx, cooldownKey, 1, otpCooldownTTL).Result()
	if err != nil {
		return fmt.Errorf("app: check otp cooldown: %w", err)
	}
	if !won {
		return domain.ErrResendTooSoon
	}

	count, err := a.RDB.Incr(ctx, hourlyKey).Result()
	if err != nil {
		return fmt.Errorf("app: increment otp hourly count: %w", err)
	}
	if count == 1 {
		if err := a.RDB.Expire(ctx, hourlyKey, otpHourlyTTL).Err(); err != nil {
			return fmt.Errorf("app: set otp hourly ttl: %w", err)
		}
	}
	if count > otpHourlyLimit {
		return domain.ErrRateLimited
	}

	code, err := generateCode()
	if err != nil {
		return fmt.Errorf("app: generate otp code: %w", err)
	}
	codeHash := hashCode(a.Pepper, dh, code)

	if _, err := a.RDB.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
		pipe.HSet(ctx, codeKey, "h", codeHash, "a", 0)
		pipe.Expire(ctx, codeKey, otpCodeTTL)
		return nil
	}); err != nil {
		return fmt.Errorf("app: store otp code: %w", err)
	}

	sender, err := a.senderFor(dest.Kind)
	if err != nil {
		a.RDB.Del(ctx, codeKey, cooldownKey)
		return err
	}

	if err := sender.SendOtp(ctx, dest.Value, code); err != nil {
		a.RDB.Del(ctx, codeKey, cooldownKey)
		return fmt.Errorf("app: send otp: %w", err)
	}

	return nil
}

// VerifyOtp validates code against the stored hash for raw's normalised
// destination, consumes it (single-use — the winner of Valkey DEL), and on
// success returns a signed session: an existing user's stored claims, or a
// freshly auto-created customer (role always "customer", D-04) whose
// identity.UserCreated event is written to the outbox in the same tx as the
// insert.
func (a *Auth) VerifyOtp(ctx context.Context, raw, code string) (Session, error) {
	dest, err := domain.NormalizeDestination(raw)
	if err != nil {
		return Session{}, err
	}
	if !isSixDigits(code) {
		return Session{}, domain.ErrInvalidArgument
	}

	dh := hashDestination(a.Pepper, dest.Value)
	key := otpCodeKey(dh)

	stored, err := a.RDB.HGetAll(ctx, key).Result()
	if err != nil {
		return Session{}, fmt.Errorf("app: get otp code: %w", err)
	}
	if len(stored) == 0 {
		return Session{}, domain.ErrCodeExpired
	}

	wantHash := hashCode(a.Pepper, dh, code)
	if !hmac.Equal([]byte(stored["h"]), []byte(wantHash)) {
		attempts, err := a.RDB.HIncrBy(ctx, key, "a", 1).Result()
		if err != nil {
			return Session{}, fmt.Errorf("app: increment otp attempts: %w", err)
		}
		if attempts >= otpMaxAttempts {
			a.RDB.Del(ctx, key)
			return Session{}, domain.ErrCodeExpired
		}
		return Session{}, &domain.CodeMismatchError{AttemptsLeft: int(otpMaxAttempts - attempts)}
	}

	won, err := a.RDB.Del(ctx, key).Result()
	if err != nil {
		return Session{}, fmt.Errorf("app: delete otp code: %w", err)
	}
	if won == 0 {
		// Another concurrent VerifyOtp call already consumed this code.
		return Session{}, domain.ErrCodeExpired
	}

	var (
		session Session
		created bool
	)
	err = bbpgx.WithTx(ctx, a.Pool, func(tx pgx.Tx) error {
		q := postgres.New(tx)

		row, getErr := getUserRow(ctx, q, dest)
		if errors.Is(getErr, pgx.ErrNoRows) {
			var insErr error
			row, insErr, created = insertCustomerRow(ctx, q, dest)
			if insErr != nil {
				return insErr
			}
			if created {
				getErr = nil
			} else {
				// Lost the create race — re-select the row the winner inserted.
				row, getErr = getUserRow(ctx, q, dest)
			}
		}
		if getErr != nil {
			return fmt.Errorf("app: get or create user: %w", getErr)
		}

		user := userFromRow(row)
		if user.DisabledAt != nil {
			return domain.ErrDisabled
		}

		if created {
			env, evErr := events.New(EventUserCreated, user.ID.String(), &identityv1.UserCreated{
				UserId:     user.ID.String(),
				Role:       user.Role,
				OperatorId: user.OperatorID,
				PierIds:    user.PierIDs,
			})
			if evErr != nil {
				return fmt.Errorf("app: build UserCreated event: %w", evErr)
			}
			if evErr := outbox.Insert(ctx, tx, env); evErr != nil {
				return fmt.Errorf("app: outbox insert: %w", evErr)
			}
		}

		sess, sessErr := issueSession(ctx, tx, a.Issuer, user)
		if sessErr != nil {
			return sessErr
		}
		session = sess
		return nil
	})
	if err != nil {
		return Session{}, err
	}

	if created && a.Nudge != nil {
		a.Nudge()
	}

	return session, nil
}

func getUserRow(ctx context.Context, q *postgres.Queries, dest domain.Destination) (postgres.User, error) {
	switch dest.Kind {
	case domain.KindEmail:
		return q.GetUserByEmail(ctx, pgText(dest.Value))
	case domain.KindPhone:
		return q.GetUserByPhone(ctx, pgText(dest.Value))
	default:
		return postgres.User{}, fmt.Errorf("app: unknown destination kind %q", dest.Kind)
	}
}

// insertCustomerRow attempts to create a new customer row for dest with a
// fresh uuid v7 id. On a unique-constraint race (another VerifyOtp call for
// the same destination won first), InsertCustomerByEmail/Phone's `on
// conflict do nothing` returns pgx.ErrNoRows and created is false — the
// caller re-selects.
func insertCustomerRow(ctx context.Context, q *postgres.Queries, dest domain.Destination) (row postgres.User, err error, created bool) {
	id, err := uuid.NewV7()
	if err != nil {
		return postgres.User{}, fmt.Errorf("app: new user id: %w", err), false
	}

	switch dest.Kind {
	case domain.KindEmail:
		row, err = q.InsertCustomerByEmail(ctx, postgres.InsertCustomerByEmailParams{ID: toPgUUID(id), Email: pgText(dest.Value)})
	case domain.KindPhone:
		row, err = q.InsertCustomerByPhone(ctx, postgres.InsertCustomerByPhoneParams{ID: toPgUUID(id), Phone: pgText(dest.Value)})
	default:
		return postgres.User{}, fmt.Errorf("app: unknown destination kind %q", dest.Kind), false
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return postgres.User{}, nil, false
	}
	if err != nil {
		return postgres.User{}, fmt.Errorf("app: insert customer: %w", err), false
	}
	return row, nil, true
}

func (a *Auth) senderFor(kind string) (notify.Sender, error) {
	switch kind {
	case domain.KindEmail:
		if a.Email == nil {
			return nil, domain.ErrDeliveryUnavailable
		}
		return a.Email, nil
	case domain.KindPhone:
		if a.SMS == nil {
			return nil, domain.ErrDeliveryUnavailable
		}
		return a.SMS, nil
	default:
		return nil, domain.ErrInvalidArgument
	}
}

func otpCodeKey(dh string) string     { return "otp:code:" + dh }
func otpCooldownKey(dh string) string { return "otp:cooldown:" + dh }
func otpHourlyKey(dh string) string   { return "otp:hourly:" + dh }

// hashDestination HMACs a normalised destination so no raw email/phone ever
// appears in a Valkey key.
func hashDestination(pepper []byte, value string) string {
	mac := hmac.New(sha256.New, pepper)
	mac.Write([]byte("dest:" + value))
	return hex.EncodeToString(mac.Sum(nil))
}

// hashCode HMACs a code scoped to its destination hash, so the same 6-digit
// code for two different destinations never hashes to the same value.
func hashCode(pepper []byte, dh, code string) string {
	mac := hmac.New(sha256.New, pepper)
	mac.Write([]byte("code:" + dh + ":" + code))
	return hex.EncodeToString(mac.Sum(nil))
}

// generateCode draws a uniform 6-digit code from crypto/rand — math/rand
// must never be imported in this service (D-03, Pitfall spoofing threat).
func generateCode() (string, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(1_000_000))
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%06d", n.Int64()), nil
}

func isSixDigits(s string) bool {
	if len(s) != 6 {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}
