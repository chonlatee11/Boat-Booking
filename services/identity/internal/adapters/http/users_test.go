package httpadapter

import (
	"context"
	"errors"
	"testing"

	"connectrpc.com/connect"
)

// TestMapUserErrorRedactsUnmappedErrors proves the WR-09 fix: an error that
// matches none of the domain sentinels must never put its own message on
// the wire. connect-go encodes err.Error() into the response for every
// code, including CodeInternal, and the gateway's admin proxy forwards
// that body to the admin browser unchanged -- the only safe place to
// redact is at the source.
func TestMapUserErrorRedactsUnmappedErrors(t *testing.T) {
	original := errors.New("app: insert staff user: pgx: failed to connect to host=postgres port=5432")
	got := mapUserError(context.Background(), original)

	var ce *connect.Error
	if !errors.As(got, &ce) {
		t.Fatalf("mapUserError(%v) = %v, not a *connect.Error", original, got)
	}
	if ce.Code() != connect.CodeInternal {
		t.Errorf("code = %v, want CodeInternal", ce.Code())
	}
	if ce.Message() == original.Error() {
		t.Errorf("message leaked the original error: %q", ce.Message())
	}
	if ce.Message() != "internal error" {
		t.Errorf("message = %q, want %q", ce.Message(), "internal error")
	}
}
