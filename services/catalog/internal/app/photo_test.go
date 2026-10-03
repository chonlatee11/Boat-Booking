package app

import (
	"context"
	"errors"
	"net/url"
	"regexp"
	"strings"
	"testing"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"

	"github.com/chonlatee11/boat-booking/pkg/auth"
	"github.com/chonlatee11/boat-booking/services/catalog/internal/domain"
)

// testPhotos builds a Photos backed by a minio.Client pointed at a fake
// S3_PUBLIC_ENDPOINT. PresignHeader is a local computation (no network
// call) when Region is set, so this never touches the network (D-19).
func testPhotos(t *testing.T) Photos {
	t.Helper()
	client, err := minio.New("localhost:8333", &minio.Options{
		Creds:  credentials.NewStaticV4("test-access-key", "test-secret-key", ""),
		Secure: false,
		Region: "us-east-1",
	})
	if err != nil {
		t.Fatalf("minio.New: %v", err)
	}
	return Photos{Client: client, Bucket: "pier-photos", PublicBaseURL: "http://localhost:8333/pier-photos"}
}

var photoKeyPattern = regexp.MustCompile(`^piers/[0-9a-f-]{36}\.png$`)

func TestPresignPierPhoto(t *testing.T) {
	p := testPhotos(t)
	scope := Scope{Role: auth.RolePierAdmin}
	ctx := context.Background()

	uploadURL, key, err := p.PresignPierPhoto(ctx, scope, "image/png", 1024)
	if err != nil {
		t.Fatalf("PresignPierPhoto: %v", err)
	}
	if !photoKeyPattern.MatchString(key) {
		t.Errorf("key = %q, want to match piers/<uuid>.png", key)
	}

	parsed, err := url.Parse(uploadURL)
	if err != nil {
		t.Fatalf("parse upload url: %v", err)
	}
	if parsed.Host != "localhost:8333" {
		t.Errorf("host = %q, want localhost:8333", parsed.Host)
	}
	if want := "/pier-photos/" + key; parsed.Path != want {
		t.Errorf("path = %q, want %q", parsed.Path, want)
	}
	if got := parsed.Query().Get("X-Amz-Expires"); got != "600" {
		t.Errorf("X-Amz-Expires = %q, want 600", got)
	}
	signedHeaders := parsed.Query().Get("X-Amz-SignedHeaders")
	for _, want := range []string{"content-length", "content-type", "host"} {
		if !strings.Contains(signedHeaders, want) {
			t.Errorf("X-Amz-SignedHeaders = %q, want to contain %q", signedHeaders, want)
		}
	}
	t.Logf("--- PASS presign shape: url=%s", uploadURL)
}

func TestPresignPierPhotoValidation(t *testing.T) {
	p := testPhotos(t)
	scope := Scope{Role: auth.RolePierAdmin}
	ctx := context.Background()

	cases := []struct {
		name        string
		contentType string
		size        int64
		wantErr     error
	}{
		{"disallowed content type", "image/gif", 1024, domain.ErrInvalidArgument},
		{"zero size", "image/png", 0, domain.ErrInvalidArgument},
		{"over max size", "image/png", 5_242_881, domain.ErrInvalidArgument},
		{"exactly max size ok", "image/png", 5_242_880, nil},
		{"exactly one byte ok", "image/jpeg", 1, nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, _, err := p.PresignPierPhoto(ctx, scope, c.contentType, c.size)
			if c.wantErr == nil {
				if err != nil {
					t.Fatalf("PresignPierPhoto(%q, %d) = %v, want nil", c.contentType, c.size, err)
				}
				return
			}
			if !errors.Is(err, c.wantErr) {
				t.Fatalf("PresignPierPhoto(%q, %d) = %v, want %v", c.contentType, c.size, err, c.wantErr)
			}
		})
	}
}

func TestPresignPierPhotoScopeAndStorage(t *testing.T) {
	ctx := context.Background()

	t.Run("staff scope denied", func(t *testing.T) {
		p := testPhotos(t)
		_, _, err := p.PresignPierPhoto(ctx, Scope{Role: auth.RoleStaff}, "image/png", 1024)
		if !errors.Is(err, domain.ErrPermissionDenied) {
			t.Fatalf("PresignPierPhoto(staff) = %v, want ErrPermissionDenied", err)
		}
	})

	t.Run("storage not configured", func(t *testing.T) {
		p := Photos{} // zero value: nil Client
		_, _, err := p.PresignPierPhoto(ctx, Scope{Role: auth.RoleSuperAdmin}, "image/png", 1024)
		if !errors.Is(err, domain.ErrFailedPrecondition) {
			t.Fatalf("PresignPierPhoto(no storage) = %v, want ErrFailedPrecondition", err)
		}
	})
}

func TestPhotosURL(t *testing.T) {
	p := Photos{PublicBaseURL: "http://localhost:8333/pier-photos"}
	if got := p.URL("piers/x.png"); got != "http://localhost:8333/pier-photos/piers/x.png" {
		t.Errorf("URL = %q, want joined path", got)
	}
	if got := p.URL(""); got != "" {
		t.Errorf("URL(\"\") = %q, want empty", got)
	}
	if got := (Photos{}).URL("piers/x.png"); got != "" {
		t.Errorf("URL with no PublicBaseURL = %q, want empty", got)
	}
}
