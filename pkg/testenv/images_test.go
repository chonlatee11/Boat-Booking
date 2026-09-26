package testenv

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestImagesMatchCompose is a drift guard (D-23): the testcontainers image
// tags this package pins must equal the tags deploy/docker-compose.yml
// actually runs, so dev/CI/prod all see the same Postgres/Redpanda behavior.
// No Docker needed — this only reads the compose file as text.
func TestImagesMatchCompose(t *testing.T) {
	path := filepath.Join(RepoRoot(), "deploy", "docker-compose.yml")
	data, err := os.ReadFile(path) //nolint:gosec // fixed repo-relative path, not user input
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	content := string(data)

	for _, image := range []string{PostgresImage, RedpandaImage} {
		want := "image: " + image
		if !strings.Contains(content, want) {
			t.Errorf("deploy/docker-compose.yml does not contain %q (testenv pins %q) — keep testcontainers and Compose on the same tag", want, image)
		}
	}
}
