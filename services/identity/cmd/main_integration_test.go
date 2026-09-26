//go:build integration

package main

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"

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
	t.Setenv("SUPER_ADMIN_EMAIL", "super-admin-default@example.com")

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
		if code, ok := fetchMailpitCode(apiURL, q); ok {
			return code
		}
		time.Sleep(200 * time.Millisecond)
	}
	t.Fatalf("no otp code arrived at Mailpit for %q within 20s", to)
	return ""
}

// fetchMailpitCode makes one attempt to find a delivered message matching q
// and extract its 6-digit OTP code.
func fetchMailpitCode(apiURL, q string) (code string, ok bool) {
	resp, err := http.Get(apiURL + "/api/v1/search?query=" + q)
	if err != nil {
		return "", false
	}
	defer func() { _ = resp.Body.Close() }()

	var search mailpitSearchResult
	if json.NewDecoder(resp.Body).Decode(&search) != nil || len(search.Messages) == 0 {
		return "", false
	}

	msgResp, err := http.Get(apiURL + "/api/v1/message/" + search.Messages[0].ID)
	if err != nil {
		return "", false
	}
	defer func() { _ = msgResp.Body.Close() }()

	var msg mailpitMessage
	if json.NewDecoder(msgResp.Body).Decode(&msg) != nil {
		return "", false
	}
	code = sixDigits.FindString(msg.Text)
	return code, code != ""
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

	// Scoped to this user's own aggregate_id, not a global count: since D-09
	// every test's identity instance also bootstraps SUPER_ADMIN_EMAIL at
	// startup, which legitimately publishes its own identity.UserCreated.
	var eventCount int
	if err := pool.QueryRow(ctx, `select count(*) from outbox where event_type = $1 and aggregate_id = $2`,
		"identity.UserCreated", userID.String()).Scan(&eventCount); err != nil {
		t.Fatalf("count outbox: %v", err)
	}
	if eventCount != 0 {
		t.Errorf("outbox UserCreated count for %s = %d, want 0 (existing user login must not publish)", userID, eventCount)
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

// testDestHash recomputes app.hashDestination's HMAC (unexported, different
// package) so tests can reach directly into Valkey by key — the same
// OTP_HASH_SECRET every setIdentityEnv test sets.
func testDestHash(dest string) string {
	mac := hmac.New(sha256.New, []byte(strings.Repeat("p", 32)))
	mac.Write([]byte("dest:" + dest))
	return hex.EncodeToString(mac.Sum(nil))
}

func assertConnectCode(t *testing.T, err error, want connect.Code, context string) {
	t.Helper()
	if err == nil {
		t.Fatalf("%s: err = nil, want %s", context, want)
	}
	if got := connect.CodeOf(err); got != want {
		t.Fatalf("%s: code = %s, want %s (err: %v)", context, got, want, err)
	}
}

// TestOtpCooldownAndHourlyLimit proves the D-03 send-rate rules: a resend
// within 60s is rejected, and a destination cannot receive more than 5 codes
// per hour. The 60s cooldown is unblocked by deleting the Valkey cooldown
// key directly rather than sleeping in the test.
func TestOtpCooldownAndHourlyLimit(t *testing.T) {
	addr, _, _ := setIdentityEnv(t)
	baseURL := "http://" + addr
	runService(t)
	waitForFullyReady(t, baseURL)

	rdb := redis.NewClient(&redis.Options{Addr: valkeyAddr})
	defer rdb.Close() //nolint:errcheck // test helper, nothing actionable

	dest := "cooldown@example.com"
	cooldownKey := "otp:cooldown:" + testDestHash(dest)

	ctx := context.Background()
	client := newAuthClient(baseURL, map[string]string{"X-Internal-Token": "test-token"})

	if _, err := client.RequestOtp(ctx, connect.NewRequest(&identityv1.RequestOtpRequest{Destination: dest})); err != nil {
		t.Fatalf("send 1: %v", err)
	}

	_, err := client.RequestOtp(ctx, connect.NewRequest(&identityv1.RequestOtpRequest{Destination: dest}))
	assertConnectCode(t, err, connect.CodeResourceExhausted, "immediate resend")

	for i := 2; i <= 6; i++ {
		if err := rdb.Del(ctx, cooldownKey).Err(); err != nil {
			t.Fatalf("delete cooldown key: %v", err)
		}
		_, err := client.RequestOtp(ctx, connect.NewRequest(&identityv1.RequestOtpRequest{Destination: dest}))
		if i < 6 {
			if err != nil {
				t.Fatalf("send %d: %v", i, err)
			}
			continue
		}
		assertConnectCode(t, err, connect.CodeResourceExhausted, "6th send in the hour")
	}
}

// TestOtpAttemptsLockout proves the D-03 lockout rule: 4 wrong codes report
// a decreasing Attempts-Left header, the 5th locks the code out entirely,
// and even the correct code is rejected afterwards.
func TestOtpAttemptsLockout(t *testing.T) {
	addr, _, _ := setIdentityEnv(t)
	baseURL := "http://" + addr
	runService(t)
	waitForFullyReady(t, baseURL)

	ctx := context.Background()
	client := newAuthClient(baseURL, map[string]string{"X-Internal-Token": "test-token"})
	dest := "lockout@example.com"

	if _, err := client.RequestOtp(ctx, connect.NewRequest(&identityv1.RequestOtpRequest{Destination: dest})); err != nil {
		t.Fatalf("RequestOtp: %v", err)
	}
	code := pollMailpitCode(t, mailpitAPIURL, dest)
	wrongCode := "000000"
	if wrongCode == code {
		wrongCode = "111111"
	}

	for attempt := 1; attempt <= 4; attempt++ {
		_, err := client.VerifyOtp(ctx, connect.NewRequest(&identityv1.VerifyOtpRequest{Destination: dest, Code: wrongCode}))
		assertConnectCode(t, err, connect.CodeInvalidArgument, fmt.Sprintf("wrong code attempt %d", attempt))
		var connErr *connect.Error
		if !errors.As(err, &connErr) {
			t.Fatalf("wrong code attempt %d: not a connect.Error: %v", attempt, err)
		}
		want := strconv.Itoa(5 - attempt)
		if got := connErr.Meta().Get("Attempts-Left"); got != want {
			t.Errorf("wrong code attempt %d: Attempts-Left = %q, want %q", attempt, got, want)
		}
	}

	_, err := client.VerifyOtp(ctx, connect.NewRequest(&identityv1.VerifyOtpRequest{Destination: dest, Code: wrongCode}))
	assertConnectCode(t, err, connect.CodeFailedPrecondition, "5th wrong code")

	_, err = client.VerifyOtp(ctx, connect.NewRequest(&identityv1.VerifyOtpRequest{Destination: dest, Code: code}))
	assertConnectCode(t, err, connect.CodeFailedPrecondition, "correct code after lockout")
}

// TestOtpSingleUseConcurrent proves the atomic-DEL single-use rule holds
// under concurrency: exactly one of many simultaneous VerifyOtp calls with
// the same correct code succeeds, and exactly one customer row is created.
func TestOtpSingleUseConcurrent(t *testing.T) {
	addr, _, dsn := setIdentityEnv(t)
	baseURL := "http://" + addr
	runService(t)
	waitForFullyReady(t, baseURL)

	ctx := context.Background()
	client := newAuthClient(baseURL, map[string]string{"X-Internal-Token": "test-token"})
	dest := "concurrent@example.com"

	if _, err := client.RequestOtp(ctx, connect.NewRequest(&identityv1.RequestOtpRequest{Destination: dest})); err != nil {
		t.Fatalf("RequestOtp: %v", err)
	}
	code := pollMailpitCode(t, mailpitAPIURL, dest)

	const n = 10
	var wg sync.WaitGroup
	var successCount, failedPreconditionCount int64
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := client.VerifyOtp(ctx, connect.NewRequest(&identityv1.VerifyOtpRequest{Destination: dest, Code: code}))
			switch {
			case err == nil:
				atomic.AddInt64(&successCount, 1)
			case connect.CodeOf(err) == connect.CodeFailedPrecondition:
				atomic.AddInt64(&failedPreconditionCount, 1)
			default:
				t.Errorf("unexpected error: %v", err)
			}
		}()
	}
	wg.Wait()

	if successCount != 1 {
		t.Errorf("successCount = %d, want 1", successCount)
	}
	if failedPreconditionCount != n-1 {
		t.Errorf("failedPreconditionCount = %d, want %d", failedPreconditionCount, n-1)
	}

	pool, err := bbpgx.NewPool(ctx, dsn)
	if err != nil {
		t.Fatalf("new verification pool: %v", err)
	}
	defer pool.Close()
	var count int
	if err := pool.QueryRow(ctx, `select count(*) from users where email = $1`, dest).Scan(&count); err != nil {
		t.Fatalf("count users: %v", err)
	}
	if count != 1 {
		t.Errorf("users count = %d, want 1", count)
	}
}

// TestPhoneOtpViaDevSms proves the dev SMS channel: a local Thai phone
// number is normalised to E.164, its code is delivered to Mailpit as
// <digits>@sms.local, and VerifyOtp creates a customer with that phone.
func TestPhoneOtpViaDevSms(t *testing.T) {
	addr, verifier, dsn := setIdentityEnv(t)
	baseURL := "http://" + addr
	runService(t)
	waitForFullyReady(t, baseURL)

	ctx := context.Background()
	client := newAuthClient(baseURL, map[string]string{"X-Internal-Token": "test-token"})

	if _, err := client.RequestOtp(ctx, connect.NewRequest(&identityv1.RequestOtpRequest{Destination: "0812345678"})); err != nil {
		t.Fatalf("RequestOtp: %v", err)
	}
	code := pollMailpitCode(t, mailpitAPIURL, "66812345678@sms.local")

	resp, err := client.VerifyOtp(ctx, connect.NewRequest(&identityv1.VerifyOtpRequest{Destination: "0812345678", Code: code}))
	if err != nil {
		t.Fatalf("VerifyOtp: %v", err)
	}
	if _, err := verifier.Verify(resp.Msg.AccessToken, auth.KindAccess); err != nil {
		t.Fatalf("verify access token: %v", err)
	}

	pool, err := bbpgx.NewPool(ctx, dsn)
	if err != nil {
		t.Fatalf("new verification pool: %v", err)
	}
	defer pool.Close()
	var count int
	if err := pool.QueryRow(ctx, `select count(*) from users where phone = $1`, "+66812345678").Scan(&count); err != nil {
		t.Fatalf("count users by phone: %v", err)
	}
	if count != 1 {
		t.Errorf("users count for +66812345678 = %d, want 1", count)
	}
}

// TestOtpNeverLogged proves no OTP code ever reaches stdout, including on
// the wrong-code mismatch path. httpx.NewLogger builds its own *slog.Logger
// writing JSON/text to os.Stdout rather than routing through
// slog.SetDefault, so this test captures process stdout directly instead of
// installing a slog.Handler (recorded per plan Task 2 action item 6).
func TestOtpNeverLogged(t *testing.T) {
	addr, _, _ := setIdentityEnv(t)
	baseURL := "http://" + addr

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	origStdout := os.Stdout
	os.Stdout = w

	var buf bytes.Buffer
	copyDone := make(chan struct{})
	go func() {
		_, _ = io.Copy(&buf, r)
		close(copyDone)
	}()

	runService(t)
	waitForFullyReady(t, baseURL)

	ctx := context.Background()
	client := newAuthClient(baseURL, map[string]string{"X-Internal-Token": "test-token"})
	dest := "logcheck@example.com"

	if _, err := client.RequestOtp(ctx, connect.NewRequest(&identityv1.RequestOtpRequest{Destination: dest})); err != nil {
		os.Stdout = origStdout
		t.Fatalf("RequestOtp: %v", err)
	}
	code := pollMailpitCode(t, mailpitAPIURL, dest)
	wrongCode := "000000"
	if wrongCode == code {
		wrongCode = "111111"
	}

	_, _ = client.VerifyOtp(ctx, connect.NewRequest(&identityv1.VerifyOtpRequest{Destination: dest, Code: wrongCode}))
	if _, err := client.VerifyOtp(ctx, connect.NewRequest(&identityv1.VerifyOtpRequest{Destination: dest, Code: code})); err != nil {
		os.Stdout = origStdout
		t.Fatalf("VerifyOtp: %v", err)
	}

	os.Stdout = origStdout
	_ = w.Close()
	<-copyDone

	if strings.Contains(buf.String(), code) {
		t.Errorf("captured stdout contains the issued OTP code")
	}
}
