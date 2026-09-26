//go:build integration

package main

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"

	identityv1 "github.com/chonlatee11/boat-booking/gen/go/identity/v1"
	"github.com/chonlatee11/boat-booking/gen/go/identity/v1/identityv1connect"
	"github.com/chonlatee11/boat-booking/pkg/auth"
	bbpgx "github.com/chonlatee11/boat-booking/pkg/pgx"
	"github.com/chonlatee11/boat-booking/pkg/testenv"
)

var (
	adminDSN string
	brokers  []string
	valkeyAddr,
	mailpitSMTPAddr, mailpitAPIURL string
)

func TestMain(m *testing.M) {
	ctx := context.Background()

	dsn, stopPG, err := testenv.StartPostgres(ctx)
	if err != nil {
		panic(err)
	}
	adminDSN = dsn

	bs, stopRP, err := testenv.StartRedpanda(ctx, serviceName)
	if err != nil {
		stopPG()
		panic(err)
	}
	brokers = bs

	vAddr, stopValkey, err := testenv.StartValkey(ctx)
	if err != nil {
		stopRP()
		stopPG()
		panic(err)
	}
	valkeyAddr = vAddr

	smtpAddr, apiURL, stopMailpit, err := testenv.StartMailpit(ctx)
	if err != nil {
		stopValkey()
		stopRP()
		stopPG()
		panic(err)
	}
	mailpitSMTPAddr = smtpAddr
	mailpitAPIURL = apiURL

	code := m.Run()
	stopMailpit()
	stopValkey()
	stopRP()
	stopPG()
	os.Exit(code)
}

// freePort picks a currently-unused 127.0.0.1 port for the service under
// test to bind. Closing the probe listener before the service starts leaves
// a small window for reuse by something else, acceptable for tests.
func freePort(t *testing.T) int {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("freePort: listen: %v", err)
	}
	defer l.Close() //nolint:errcheck // test helper, nothing actionable
	return l.Addr().(*net.TCPAddr).Port
}

// generateKeypairB64 mirrors devtoken's key generation: a fresh RSA keypair,
// base64-std-encoded PKCS8/PKIX PEM.
func generateKeypairB64(t *testing.T) (privB64 string, pub *rsa.PublicKey) {
	t.Helper()
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate rsa key: %v", err)
	}
	privBytes, err := x509.MarshalPKCS8PrivateKey(priv)
	if err != nil {
		t.Fatalf("marshal private key: %v", err)
	}
	privPEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: privBytes})
	return base64.StdEncoding.EncodeToString(privPEM), &priv.PublicKey
}

// setIdentityEnv sets the env vars run() reads, isolated per-test via a
// fresh DB, HTTP port, and RSA keypair. It returns the listen address, the
// verifier for the freshly generated key, and the database's own DSN.
func setIdentityEnv(t *testing.T) (addr string, verifier *auth.Verifier, dsn string) {
	t.Helper()
	dsn = testenv.NewDB(t, adminDSN, "../migrations")
	port := freePort(t)
	addr = fmt.Sprintf("127.0.0.1:%d", port)

	privB64, pub := generateKeypairB64(t)
	const issuer = "boatbooking-test"

	t.Setenv("HTTP_ADDR", addr)
	t.Setenv("DATABASE_URL", dsn)
	t.Setenv("KAFKA_BROKERS", strings.Join(brokers, ","))
	t.Setenv("INTERNAL_TOKEN", "test-token")
	t.Setenv("LOG_FORMAT", "text")
	t.Setenv("VALKEY_ADDR", valkeyAddr)
	t.Setenv("JWT_PRIVATE_KEY_B64", privB64)
	t.Setenv("JWT_ISSUER", issuer)
	t.Setenv("OTP_HASH_SECRET", strings.Repeat("p", 32))
	t.Setenv("SMTP_ADDR", mailpitSMTPAddr)

	return addr, auth.NewVerifier(pub, issuer), dsn
}

func runService(t *testing.T) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)
	go func() { errCh <- run(ctx) }()
	t.Cleanup(func() {
		cancel()
		select {
		case <-errCh:
		case <-time.After(15 * time.Second):
			t.Error("run did not return within 15s of test cleanup cancellation")
		}
	})
}

func waitForFullyReady(t *testing.T, baseURL string) {
	t.Helper()
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		if readyBody, ok := getReadyz(baseURL); ok {
			if readyBody["db"] == "ok" && readyBody["kafka"] == "ok" && readyBody["valkey"] == "ok" {
				return
			}
		}
		time.Sleep(200 * time.Millisecond)
	}
	t.Fatal("service did not report db+kafka+valkey ready in time")
}

func getReadyz(baseURL string) (map[string]string, bool) {
	resp, err := http.Get(baseURL + "/readyz")
	if err != nil {
		return nil, false
	}
	defer resp.Body.Close() //nolint:errcheck // test helper, nothing actionable
	if resp.StatusCode != http.StatusOK {
		return nil, false
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, false
	}
	var body map[string]string
	if err := json.Unmarshal(data, &body); err != nil {
		return nil, false
	}
	return body, true
}

// waitFor polls cond until it returns true or timeout elapses, failing the
// test if it never does.
func waitFor(t testing.TB, timeout time.Duration, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	if !cond() {
		t.Fatalf("condition not met within %s", timeout)
	}
}

var sixDigits = regexp.MustCompile(`[0-9]{6}`)

type mailpitSearchResult struct {
	Messages []struct {
		ID string `json:"ID"`
	} `json:"messages"`
}

type mailpitMessage struct {
	Text string `json:"Text"`
}

// pollMailpitCode polls Mailpit's API for a message delivered to to and
// extracts the single 6-digit OTP code from its plain-text body, failing the
// test if none arrives in time. Never logs the message body verbatim beyond
// what t.Fatalf needs to report the failure — matching D-03's "never log the
// code" rule extends to test debugging, but a Fatal is diagnostic, not
// operational, output.
func pollMailpitCode(t *testing.T, apiURL, to string) string {
	t.Helper()
	q := url.QueryEscape(fmt.Sprintf(`to:%q`, to))

	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		resp, err := http.Get(apiURL + "/api/v1/search?query=" + q)
		if err == nil {
			var search mailpitSearchResult
			if json.NewDecoder(resp.Body).Decode(&search) == nil && len(search.Messages) > 0 {
				resp.Body.Close() //nolint:errcheck // test helper, nothing actionable
				id := search.Messages[0].ID
				msgResp, err := http.Get(apiURL + "/api/v1/message/" + id)
				if err == nil {
					var msg mailpitMessage
					if json.NewDecoder(msgResp.Body).Decode(&msg) == nil {
						msgResp.Body.Close() //nolint:errcheck // test helper, nothing actionable
						if code := sixDigits.FindString(msg.Text); code != "" {
							return code
						}
					}
					msgResp.Body.Close() //nolint:errcheck // test helper, nothing actionable
				}
			} else {
				resp.Body.Close() //nolint:errcheck // test helper, nothing actionable
			}
		}
		time.Sleep(200 * time.Millisecond)
	}
	t.Fatalf("no otp code arrived at Mailpit for %q within 20s", to)
	return ""
}

func newAuthClient(baseURL string, headers map[string]string) identityv1connect.AuthServiceClient {
	return identityv1connect.NewAuthServiceClient(http.DefaultClient, baseURL,
		connect.WithInterceptors(headerInterceptor{headers: headers}))
}

type headerInterceptor struct{ headers map[string]string }

func (i headerInterceptor) WrapUnary(next connect.UnaryFunc) connect.UnaryFunc {
	return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
		for k, v := range i.headers {
			req.Header().Set(k, v)
		}
		return next(ctx, req)
	}
}
func (i headerInterceptor) WrapStreamingClient(next connect.StreamingClientFunc) connect.StreamingClientFunc {
	return next
}
func (i headerInterceptor) WrapStreamingHandler(next connect.StreamingHandlerFunc) connect.StreamingHandlerFunc {
	return next
}

// TestEmailOtpLogin proves the full RequestOtp -> Valkey -> SMTP/Mailpit ->
// VerifyOtp -> customer row + UserCreated outbox -> signed JWT + refresh
// token loop end to end (AUTH-01, D-01 through D-06).
func TestEmailOtpLogin(t *testing.T) {
	addr, verifier, dsn := setIdentityEnv(t)
	baseURL := "http://" + addr
	runService(t)
	waitForFullyReady(t, baseURL)

	ctx := context.Background()
	client := newAuthClient(baseURL, map[string]string{"X-Internal-Token": "test-token"})

	if _, err := client.RequestOtp(ctx, connect.NewRequest(&identityv1.RequestOtpRequest{
		Destination: " E2E@Example.com ",
	})); err != nil {
		t.Fatalf("RequestOtp: %v", err)
	}

	code := pollMailpitCode(t, mailpitAPIURL, "e2e@example.com")

	resp, err := client.VerifyOtp(ctx, connect.NewRequest(&identityv1.VerifyOtpRequest{
		Destination: " E2E@Example.com ",
		Code:        code,
	}))
	if err != nil {
		t.Fatalf("VerifyOtp: %v", err)
	}

	claims, err := verifier.Verify(resp.Msg.AccessToken, auth.KindAccess)
	if err != nil {
		t.Fatalf("verify access token: %v", err)
	}
	if claims.Role != auth.RoleCustomer {
		t.Errorf("claims.Role = %q, want %q", claims.Role, auth.RoleCustomer)
	}
	if len(claims.PierIDs) != 0 {
		t.Errorf("claims.PierIDs = %v, want empty", claims.PierIDs)
	}
	if resp.Msg.RefreshToken == "" {
		t.Error("refresh_token is empty")
	}
	if resp.Msg.User.Role != auth.RoleCustomer {
		t.Errorf("resp.User.Role = %q, want %q", resp.Msg.User.Role, auth.RoleCustomer)
	}

	pool, err := bbpgx.NewPool(ctx, dsn)
	if err != nil {
		t.Fatalf("new verification pool: %v", err)
	}
	defer pool.Close()

	var userCount int
	if err := pool.QueryRow(ctx, `select count(*) from users where email = $1`, "e2e@example.com").Scan(&userCount); err != nil {
		t.Fatalf("count users: %v", err)
	}
	if userCount != 1 {
		t.Fatalf("users count for e2e@example.com = %d, want 1", userCount)
	}

	sum := sha256.Sum256([]byte(resp.Msg.RefreshToken))
	var refreshCount int
	if err := pool.QueryRow(ctx, `select count(*) from refresh_tokens where token_hash = $1`, sum[:]).Scan(&refreshCount); err != nil {
		t.Fatalf("count refresh_tokens: %v", err)
	}
	if refreshCount != 1 {
		t.Fatalf("refresh_tokens count for issued token = %d, want 1", refreshCount)
	}

	waitFor(t, 15*time.Second, func() bool {
		var count int
		err := pool.QueryRow(ctx, `select count(*) from outbox where event_type = $1 and aggregate_id = $2`,
			"identity.UserCreated", resp.Msg.User.UserId,
		).Scan(&count)
		return err == nil && count == 1
	})
}

// TestExistingStaffLoginGetsClaims proves that a pre-existing staff user's
// role/operator_id/pier_ids reach the issued JWT, and that logging in via
// OTP never creates a second row or a second UserCreated event for them
// (D-05, AUTH-02).
func TestExistingStaffLoginGetsClaims(t *testing.T) {
	addr, verifier, dsn := setIdentityEnv(t)
	baseURL := "http://" + addr
	runService(t)
	waitForFullyReady(t, baseURL)

	pool, err := bbpgx.NewPool(context.Background(), dsn)
	if err != nil {
		t.Fatalf("new verification pool: %v", err)
	}
	defer pool.Close()

	userID := uuid.Must(uuid.NewV7())
	operatorID := uuid.New()
	pierA, pierB := uuid.New(), uuid.New()
	const email = "staff@example.com"

	if _, err := pool.Exec(context.Background(), `
		insert into users (id, email, role, operator_id, pier_ids)
		values ($1, $2, 'pier_admin', $3, $4)
	`, userID, email, operatorID, []uuid.UUID{pierA, pierB}); err != nil {
		t.Fatalf("seed staff user: %v", err)
	}

	ctx := context.Background()
	client := newAuthClient(baseURL, map[string]string{"X-Internal-Token": "test-token"})

	if _, err := client.RequestOtp(ctx, connect.NewRequest(&identityv1.RequestOtpRequest{Destination: email})); err != nil {
		t.Fatalf("RequestOtp: %v", err)
	}
	code := pollMailpitCode(t, mailpitAPIURL, email)

	resp, err := client.VerifyOtp(ctx, connect.NewRequest(&identityv1.VerifyOtpRequest{Destination: email, Code: code}))
	if err != nil {
		t.Fatalf("VerifyOtp: %v", err)
	}

	claims, err := verifier.Verify(resp.Msg.AccessToken, auth.KindAccess)
	if err != nil {
		t.Fatalf("verify access token: %v", err)
	}
	if claims.Role != "pier_admin" {
		t.Errorf("claims.Role = %q, want pier_admin", claims.Role)
	}
	if claims.OperatorID != operatorID.String() {
		t.Errorf("claims.OperatorID = %q, want %q", claims.OperatorID, operatorID.String())
	}
	wantPiers := map[string]bool{pierA.String(): true, pierB.String(): true}
	if len(claims.PierIDs) != 2 || !wantPiers[claims.PierIDs[0]] || !wantPiers[claims.PierIDs[1]] {
		t.Errorf("claims.PierIDs = %v, want %v (any order)", claims.PierIDs, wantPiers)
	}

	var userCount int
	if err := pool.QueryRow(ctx, `select count(*) from users where email = $1`, email).Scan(&userCount); err != nil {
		t.Fatalf("count users: %v", err)
	}
	if userCount != 1 {
		t.Fatalf("users count for %s = %d, want 1 (no duplicate created)", email, userCount)
	}

	var eventCount int
	if err := pool.QueryRow(ctx, `select count(*) from outbox where event_type = $1`, "identity.UserCreated").Scan(&eventCount); err != nil {
		t.Fatalf("count outbox: %v", err)
	}
	if eventCount != 0 {
		t.Errorf("outbox UserCreated count = %d, want 0 (existing user login must not publish)", eventCount)
	}
}

// TestRejectsMissingInternalToken proves every identity RPC rejects a caller
// without the internal token (AUTH-03).
func TestRejectsMissingInternalToken(t *testing.T) {
	addr, _, _ := setIdentityEnv(t)
	baseURL := "http://" + addr
	runService(t)
	waitForFullyReady(t, baseURL)

	client := newAuthClient(baseURL, nil)
	_, err := client.RequestOtp(context.Background(), connect.NewRequest(&identityv1.RequestOtpRequest{Destination: "x@example.com"}))
	if err == nil {
		t.Fatal("RequestOtp without internal token: err = nil, want Unauthenticated")
	}
	if connect.CodeOf(err) != connect.CodeUnauthenticated {
		t.Fatalf("RequestOtp without internal token: code = %s, want %s", connect.CodeOf(err), connect.CodeUnauthenticated)
	}
}
