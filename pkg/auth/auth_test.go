package auth

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"net/http"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func mustGenerateKey(t *testing.T) *rsa.PrivateKey {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	return key
}

func TestIssueVerifyRoundTrip(t *testing.T) {
	priv := mustGenerateKey(t)
	issuer := NewIssuer(priv, "test-issuer")
	verifier := NewVerifier(&priv.PublicKey, "test-issuer")

	now := time.Now()
	want := Claims{UserID: "u1", OperatorID: "op1", Role: "pier_admin", Kind: KindAccess}
	tok, err := issuer.Issue(want, now)
	if err != nil {
		t.Fatalf("issue: %v", err)
	}

	got, err := verifier.Verify(tok, KindAccess)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if got != want {
		t.Fatalf("claims mismatch: got %+v, want %+v", got, want)
	}
}

func TestAccessTokenTTL(t *testing.T) {
	priv := mustGenerateKey(t)
	issuer := NewIssuer(priv, "test-issuer")
	now := time.Now()
	tok, err := issuer.Issue(Claims{UserID: "u1", Kind: KindAccess}, now)
	if err != nil {
		t.Fatalf("issue: %v", err)
	}
	iat, exp := decodeIatExp(t, tok)
	if diff := exp - iat; diff != 900 {
		t.Fatalf("access token exp-iat = %d, want 900", diff)
	}
}

func TestRefreshTokenTTL(t *testing.T) {
	priv := mustGenerateKey(t)
	issuer := NewIssuer(priv, "test-issuer")
	now := time.Now()
	tok, err := issuer.Issue(Claims{UserID: "u1", Kind: KindRefresh}, now)
	if err != nil {
		t.Fatalf("issue: %v", err)
	}
	iat, exp := decodeIatExp(t, tok)
	if diff := exp - iat; diff != 2592000 {
		t.Fatalf("refresh token exp-iat = %d, want 2592000", diff)
	}
}

func decodeIatExp(t *testing.T, tok string) (iat, exp int64) {
	t.Helper()
	parser := jwt.NewParser()
	var claims jwtClaims
	if _, _, err := parser.ParseUnverified(tok, &claims); err != nil {
		t.Fatalf("parse unverified: %v", err)
	}
	return claims.IssuedAt.Unix(), claims.ExpiresAt.Unix()
}

func TestVerifyRejectsForeignKey(t *testing.T) {
	priv := mustGenerateKey(t)
	foreign := mustGenerateKey(t)
	issuer := NewIssuer(foreign, "test-issuer")
	verifier := NewVerifier(&priv.PublicKey, "test-issuer")

	now := time.Now()
	tok, err := issuer.Issue(Claims{UserID: "u1", Kind: KindAccess}, now)
	if err != nil {
		t.Fatalf("issue: %v", err)
	}
	if _, err := verifier.Verify(tok, KindAccess); err == nil {
		t.Fatal("expected error for token signed by a foreign key")
	}
}

func TestVerifyRejectsAlgConfusion(t *testing.T) {
	priv := mustGenerateKey(t)
	verifier := NewVerifier(&priv.PublicKey, "test-issuer")

	pubBytes, err := x509.MarshalPKIXPublicKey(&priv.PublicKey)
	if err != nil {
		t.Fatalf("marshal public key: %v", err)
	}
	pubPEM := pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: pubBytes})

	claims := jwtClaims{
		Sub:  "u1",
		Kind: KindAccess,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    "test-issuer",
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(AccessTTL)),
		},
	}
	hsToken := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := hsToken.SignedString(pubPEM)
	if err != nil {
		t.Fatalf("sign HS256 alg-confusion token: %v", err)
	}

	if _, err := verifier.Verify(signed, KindAccess); err == nil {
		t.Fatal("expected error for HS256 alg-confusion token, got nil")
	}
}

func TestVerifyRejectsExpired(t *testing.T) {
	priv := mustGenerateKey(t)
	issuer := NewIssuer(priv, "test-issuer")
	verifier := NewVerifier(&priv.PublicKey, "test-issuer")

	past := time.Now().Add(-2 * time.Hour)
	tok, err := issuer.Issue(Claims{UserID: "u1", Kind: KindAccess}, past)
	if err != nil {
		t.Fatalf("issue: %v", err)
	}
	if _, err := verifier.Verify(tok, KindAccess); err == nil {
		t.Fatal("expected error for expired token")
	}
}

func TestVerifyRejectsWrongIssuer(t *testing.T) {
	priv := mustGenerateKey(t)
	issuer := NewIssuer(priv, "issuer-a")
	verifier := NewVerifier(&priv.PublicKey, "issuer-b")

	now := time.Now()
	tok, err := issuer.Issue(Claims{UserID: "u1", Kind: KindAccess}, now)
	if err != nil {
		t.Fatalf("issue: %v", err)
	}
	if _, err := verifier.Verify(tok, KindAccess); err == nil {
		t.Fatal("expected error for wrong issuer")
	}
}

func TestVerifyRejectsWrongKind(t *testing.T) {
	priv := mustGenerateKey(t)
	issuer := NewIssuer(priv, "test-issuer")
	verifier := NewVerifier(&priv.PublicKey, "test-issuer")

	now := time.Now()
	tok, err := issuer.Issue(Claims{UserID: "u1", Kind: KindRefresh}, now)
	if err != nil {
		t.Fatalf("issue: %v", err)
	}
	if _, err := verifier.Verify(tok, KindAccess); err == nil {
		t.Fatal("expected error verifying a refresh token as access")
	}
}

func TestCookieAttributes(t *testing.T) {
	c := Cookie(KindAccess, "tok")
	if c.Name != AccessCookie {
		t.Errorf("Name = %q, want %q", c.Name, AccessCookie)
	}
	if !c.HttpOnly {
		t.Error("HttpOnly = false, want true")
	}
	if !c.Secure {
		t.Error("Secure = false, want true")
	}
	if c.SameSite != http.SameSiteLaxMode {
		t.Errorf("SameSite = %v, want %v", c.SameSite, http.SameSiteLaxMode)
	}
	if c.Path != "/" {
		t.Errorf("Path = %q, want /", c.Path)
	}
	if c.MaxAge != 900 {
		t.Errorf("MaxAge = %d, want 900", c.MaxAge)
	}
}
