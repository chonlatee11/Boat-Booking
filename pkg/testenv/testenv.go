// Package testenv provides the shared testcontainers helpers (Postgres 17 +
// Redpanda) every service's `-tags=integration` suite uses (D-23). Only test
// code imports this package.
package testenv

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"math/rand"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib" // registers the "pgx" database/sql driver, used by goose
	"github.com/pressly/goose/v3"
	testcontainers "github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/modules/redpanda"
	"github.com/testcontainers/testcontainers-go/wait"
)

// PostgresImage, RedpandaImage, ValkeyImage and MailpitImage are pinned to
// the same tags as deploy/docker-compose.yml (D-23) so dev, CI, and prod see
// identical behavior.
const (
	PostgresImage = "postgres:17.9-alpine"
	RedpandaImage = "redpandadata/redpanda:v26.2.3"
	ValkeyImage   = "valkey/valkey:9.0.6-alpine"
	MailpitImage  = "axllent/mailpit:v1.31.2"
)

// RepoRoot returns the absolute path to the repository root, derived from
// this file's own location so it works regardless of the test binary's
// working directory.
func RepoRoot() string {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		panic("testenv: could not determine caller for RepoRoot")
	}
	// this file lives at <root>/pkg/testenv/testenv.go
	return filepath.Join(filepath.Dir(file), "..", "..")
}

// PlatformMigrations returns the shared platform migrations directory that
// every service copies from services/_template (D-05).
func PlatformMigrations() string {
	return filepath.Join(RepoRoot(), "services", "_template", "migrations")
}

// StartPostgres starts a Postgres testcontainer and returns an admin DSN
// (database "postgres") plus a stop function. Dynamic port allocation only
// (Pitfall 12).
func StartPostgres(ctx context.Context) (adminDSN string, stop func(), err error) {
	container, err := postgres.Run(ctx, PostgresImage,
		postgres.WithDatabase("postgres"),
		postgres.WithUsername("postgres"),
		postgres.WithPassword("postgres"),
		postgres.BasicWaitStrategies(),
	)
	if err != nil {
		return "", nil, fmt.Errorf("testenv: start postgres: %w", err)
	}

	dsn, err := container.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		_ = container.Terminate(ctx)
		return "", nil, fmt.Errorf("testenv: postgres connection string: %w", err)
	}

	return dsn, func() { _ = container.Terminate(ctx) }, nil
}

// NewDB creates a fresh, randomly-named database on the server at adminDSN,
// applies every migrationDir with goose, and returns that database's DSN.
// Called once per test for isolation; the container itself is torn down by
// the caller's stop func from StartPostgres.
func NewDB(t testing.TB, adminDSN string, migrationDirs ...string) string {
	t.Helper()
	ctx := context.Background()

	dbName := fmt.Sprintf("t_%x", rand.Uint64()) //nolint:gosec // test-only database name, not security-sensitive

	adminPool, err := pgxpool.New(ctx, adminDSN) //nolint:forbidigo // pkg/testenv is a sanctioned constructor (D-23)
	if err != nil {
		t.Fatalf("testenv: connect admin pool: %v", err)
	}
	defer adminPool.Close()

	if _, err := adminPool.Exec(ctx, "create database "+pgx.Identifier{dbName}.Sanitize()); err != nil {
		t.Fatalf("testenv: create database %s: %v", dbName, err)
	}

	u, err := url.Parse(adminDSN)
	if err != nil {
		t.Fatalf("testenv: parse admin dsn: %v", err)
	}
	u.Path = "/" + dbName
	dsn := u.String()

	db, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatalf("testenv: open %s: %v", dbName, err)
	}
	defer func() { _ = db.Close() }()

	for _, dir := range migrationDirs {
		provider, err := goose.NewProvider(goose.DialectPostgres, db, os.DirFS(dir))
		if err != nil {
			t.Fatalf("testenv: goose provider for %s: %v", dir, err)
		}
		if _, err := provider.Up(ctx); err != nil {
			t.Fatalf("testenv: goose up for %s: %v", dir, err)
		}
	}

	return dsn
}

// StartRedpanda starts a Redpanda testcontainer, provisions <svc>.events /
// <svc>.events.dlq topics for every given service name by copying
// deploy/redpanda/topics.sh into the container and executing it (D-24 reuse),
// and returns the seed broker address(es).
func StartRedpanda(ctx context.Context, services ...string) (brokers []string, stop func(), err error) {
	container, err := redpanda.Run(ctx, RedpandaImage)
	if err != nil {
		return nil, nil, fmt.Errorf("testenv: start redpanda: %w", err)
	}

	seedBroker, err := container.KafkaSeedBroker(ctx)
	if err != nil {
		_ = container.Terminate(ctx)
		return nil, nil, fmt.Errorf("testenv: redpanda seed broker: %w", err)
	}

	scriptPath := filepath.Join(RepoRoot(), "deploy", "redpanda", "topics.sh")
	script, err := os.ReadFile(scriptPath) //nolint:gosec // fixed repo-relative path, not user input
	if err != nil {
		_ = container.Terminate(ctx)
		return nil, nil, fmt.Errorf("testenv: read topics.sh: %w", err)
	}

	const containerScriptPath = "/tmp/topics.sh"
	if err := container.CopyToContainer(ctx, script, containerScriptPath, 0o755); err != nil {
		_ = container.Terminate(ctx)
		return nil, nil, fmt.Errorf("testenv: copy topics.sh: %w", err)
	}

	args := append([]string{"sh", containerScriptPath}, services...)
	exitCode, reader, err := container.Exec(ctx, args)
	if err != nil {
		_ = container.Terminate(ctx)
		return nil, nil, fmt.Errorf("testenv: exec topics.sh: %w", err)
	}
	if exitCode != 0 {
		output, _ := io.ReadAll(reader)
		_ = container.Terminate(ctx)
		return nil, nil, fmt.Errorf("testenv: topics.sh exited %d: %s", exitCode, output)
	}

	return []string{seedBroker}, func() { _ = container.Terminate(ctx) }, nil
}

// StartValkey starts a Valkey testcontainer and returns its host:port
// address plus a stop function.
func StartValkey(ctx context.Context) (addr string, stop func(), err error) {
	const port = "6379/tcp"
	container, err := testcontainers.Run(ctx, ValkeyImage,
		testcontainers.WithExposedPorts(port),
		testcontainers.WithWaitStrategy(wait.ForListeningPort(port)),
	)
	if err != nil {
		return "", nil, fmt.Errorf("testenv: start valkey: %w", err)
	}

	addr, err = container.PortEndpoint(ctx, port, "")
	if err != nil {
		_ = container.Terminate(ctx)
		return "", nil, fmt.Errorf("testenv: valkey port endpoint: %w", err)
	}

	return addr, func() { _ = container.Terminate(ctx) }, nil
}

// StartMailpit starts a Mailpit testcontainer and returns its SMTP address
// (host:port) and API base URL (http://host:port) plus a stop function.
func StartMailpit(ctx context.Context) (smtpAddr, apiURL string, stop func(), err error) {
	const smtpPort = "1025/tcp"
	const apiPort = "8025/tcp"
	container, err := testcontainers.Run(ctx, MailpitImage,
		testcontainers.WithExposedPorts(smtpPort, apiPort),
		testcontainers.WithWaitStrategy(wait.ForListeningPort(apiPort)),
	)
	if err != nil {
		return "", "", nil, fmt.Errorf("testenv: start mailpit: %w", err)
	}

	smtpAddr, err = container.PortEndpoint(ctx, smtpPort, "")
	if err != nil {
		_ = container.Terminate(ctx)
		return "", "", nil, fmt.Errorf("testenv: mailpit smtp endpoint: %w", err)
	}
	apiURL, err = container.PortEndpoint(ctx, apiPort, "http")
	if err != nil {
		_ = container.Terminate(ctx)
		return "", "", nil, fmt.Errorf("testenv: mailpit api endpoint: %w", err)
	}

	return smtpAddr, apiURL, func() { _ = container.Terminate(ctx) }, nil
}
