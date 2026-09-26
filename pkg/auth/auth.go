// Package auth issues and verifies RS256 JWTs for the platform (D-27, D-31).
package auth

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// Kind distinguishes access from refresh tokens (D-31).
type Kind string

const (
	KindAccess  Kind = "access"
	KindRefresh Kind = "refresh"

	AccessTTL  = 15 * time.Minute
	RefreshTTL = 720 * time.Hour // 30 days

	AccessCookie  = "access_token"
	RefreshCookie = "refresh_token"
)

// Role names, defined once here so every service imports these instead of
// repeating string literals (D-06).
const (
	RoleCustomer   = "customer"
	RoleStaff      = "staff"
	RolePierAdmin  = "pier_admin"
	RoleSuperAdmin = "super_admin"
)

// Claims is the identity carried in a token (D-27). PierIDs narrows a
// pier_admin/staff user's scope to specific piers within OperatorID (D-06,
// D-07) — it never widens scope, and super_admin bypasses it entirely.
type Claims struct {
	UserID     string
	OperatorID string
	Role       string
	PierIDs    []string
	Kind       Kind
}

type jwtClaims struct {
	Sub        string   `json:"sub"`
	OperatorID string   `json:"operator_id"`
	Role       string   `json:"role"`
	PierIDs    []string `json:"pier_ids,omitempty"`
	Kind       Kind     `json:"kind"`
	jwt.RegisteredClaims
}

// Issuer mints RS256 tokens with a private key.
type Issuer struct {
	priv   *rsa.PrivateKey
	issuer string
}

func NewIssuer(priv *rsa.PrivateKey, issuer string) *Issuer {
	return &Issuer{priv: priv, issuer: issuer}
}

// Issue signs a token for c, using now as the clock (callers pass the time — never called internally).
func (i *Issuer) Issue(c Claims, now time.Time) (string, error) {
	ttl := AccessTTL
	if c.Kind == KindRefresh {
		ttl = RefreshTTL
	}
	claims := jwtClaims{
		Sub:        c.UserID,
		OperatorID: c.OperatorID,
		Role:       c.Role,
		PierIDs:    c.PierIDs,
		Kind:       c.Kind,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    i.issuer,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
		},
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	return tok.SignedString(i.priv)
}

// Verifier verifies RS256 tokens with a public key.
type Verifier struct {
	pub    *rsa.PublicKey
	issuer string
}

func NewVerifier(pub *rsa.PublicKey, issuer string) *Verifier {
	return &Verifier{pub: pub, issuer: issuer}
}

var (
	ErrWrongKind = errors.New("auth: wrong token kind")
)

// Verify parses and validates token, requiring RS256, the configured issuer,
// a required expiration, and the given kind.
func (v *Verifier) Verify(token string, want Kind) (Claims, error) {
	var claims jwtClaims
	_, err := jwt.ParseWithClaims(token, &claims, func(t *jwt.Token) (interface{}, error) {
		return v.pub, nil
	},
		jwt.WithValidMethods([]string{"RS256"}),
		jwt.WithIssuer(v.issuer),
		jwt.WithExpirationRequired(),
	)
	if err != nil {
		return Claims{}, fmt.Errorf("auth: verify: %w", err)
	}
	if claims.Kind != want {
		return Claims{}, ErrWrongKind
	}
	pierIDs := claims.PierIDs
	if pierIDs == nil {
		pierIDs = []string{}
	}
	return Claims{
		UserID:     claims.Sub,
		OperatorID: claims.OperatorID,
		Role:       claims.Role,
		PierIDs:    pierIDs,
		Kind:       claims.Kind,
	}, nil
}

// ParsePrivateKeyB64 decodes a base64-std PEM PKCS#8 RSA private key.
func ParsePrivateKeyB64(s string) (*rsa.PrivateKey, error) {
	raw, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		return nil, fmt.Errorf("auth: decode private key b64: %w", err)
	}
	block, _ := pem.Decode(raw)
	if block == nil {
		return nil, errors.New("auth: no PEM block in private key")
	}
	key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("auth: parse PKCS8 private key: %w", err)
	}
	rsaKey, ok := key.(*rsa.PrivateKey)
	if !ok {
		return nil, errors.New("auth: private key is not RSA")
	}
	return rsaKey, nil
}

// ParsePublicKeyB64 decodes a base64-std PEM PKIX RSA public key.
func ParsePublicKeyB64(s string) (*rsa.PublicKey, error) {
	raw, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		return nil, fmt.Errorf("auth: decode public key b64: %w", err)
	}
	block, _ := pem.Decode(raw)
	if block == nil {
		return nil, errors.New("auth: no PEM block in public key")
	}
	key, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("auth: parse PKIX public key: %w", err)
	}
	rsaKey, ok := key.(*rsa.PublicKey)
	if !ok {
		return nil, errors.New("auth: public key is not RSA")
	}
	return rsaKey, nil
}

// Cookie builds the httpOnly cookie carrying token for the given kind.
func Cookie(kind Kind, token string) *http.Cookie {
	name := AccessCookie
	ttl := AccessTTL
	if kind == KindRefresh {
		name = RefreshCookie
		ttl = RefreshTTL
	}
	return &http.Cookie{
		Name:     name,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int(ttl.Seconds()),
	}
}
