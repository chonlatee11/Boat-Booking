package app

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/minio/minio-go/v7"

	"github.com/chonlatee11/boat-booking/services/catalog/internal/domain"
)

// photoContentTypes is the allow-list of upload content types (D-19),
// mapped to the file extension used in the generated object key.
var photoContentTypes = map[string]string{
	"image/jpeg": "jpg",
	"image/png":  "png",
	"image/webp": "webp",
}

// maxPhotoBytes is the upload size ceiling (D-19): 5 MB.
const maxPhotoBytes = 5 << 20

// photoExpiry is how long a presigned upload URL stays valid (D-19).
const photoExpiry = 10 * time.Minute

// Photos issues presigned pier photo upload URLs and builds their public
// browser-usable URL (D-19). A nil Client means object storage isn't
// configured — PresignPierPhoto then returns domain.ErrFailedPrecondition
// instead of panicking, so catalog still starts without storage configured.
type Photos struct {
	Client        *minio.Client
	Bucket        string
	PublicBaseURL string
}

// PresignPierPhoto validates contentType/size against the D-19 allow-list
// and bound, then mints a PUT URL scoped to one server-generated object key
// ("piers/<uuidv7>.<ext>") whose signature covers Content-Type and
// Content-Length — the browser must send both headers exactly as declared
// here for the signature to validate (T-02-08-02, T-02-08-03). Requires
// scope.CanWrite() (pier_admin or super_admin, T-02-08-01); storage not
// configured returns domain.ErrFailedPrecondition so catalog still starts
// without an object-storage endpoint.
func (p Photos) PresignPierPhoto(ctx context.Context, scope Scope, contentType string, size int64) (uploadURL, key string, err error) {
	if !scope.CanWrite() {
		return "", "", domain.ErrPermissionDenied
	}
	ext, ok := photoContentTypes[contentType]
	if !ok {
		return "", "", fmt.Errorf("%w: content_type must be image/jpeg, image/png, or image/webp, got %q", domain.ErrInvalidArgument, contentType)
	}
	if size < 1 || size > maxPhotoBytes {
		return "", "", fmt.Errorf("%w: size_bytes must be 1-%d, got %d", domain.ErrInvalidArgument, int64(maxPhotoBytes), size)
	}
	if p.Client == nil {
		return "", "", fmt.Errorf("%w: object storage not configured", domain.ErrFailedPrecondition)
	}

	id, err := uuid.NewV7()
	if err != nil {
		return "", "", fmt.Errorf("app: new photo id: %w", err)
	}
	key = fmt.Sprintf("piers/%s.%s", id.String(), ext)

	u, err := p.Client.PresignHeader(ctx, http.MethodPut, p.Bucket, key, photoExpiry, nil, http.Header{
		"Content-Type":   {contentType},
		"Content-Length": {strconv.FormatInt(size, 10)},
	})
	if err != nil {
		return "", "", fmt.Errorf("app: presign pier photo: %w", err)
	}
	return u.String(), key, nil
}

// URL returns key's public browser-usable URL, or "" when key or
// PublicBaseURL is empty (no photo set, or no storage configured).
func (p Photos) URL(key string) string {
	if key == "" || p.PublicBaseURL == "" {
		return ""
	}
	return p.PublicBaseURL + "/" + key
}
