package httpadapter

import (
	"errors"
	"testing"

	"connectrpc.com/connect"
)

// TestMapAuthErrorRedactsUnmappedErrors proves the WR-09 fix: an error that
// matches none of the domain sentinels must never put its own message on
// the wire. connect-go encodes err.Error() into the response for every
// code, including CodeInternal, and the gateway forwards that body to the
// browser unchanged -- the only safe place to redact is at the source.
func TestMapAuthErrorRedactsUnmappedErrors(t *testing.T) {
	original := errors.New("app: get user: pgx: failed to connect to host=postgres port=5432")
	got := mapAuthError(original)

	var ce *connect.Error
	if !errors.As(got, &ce) {
		t.Fatalf("mapAuthError(%v) = %v, not a *connect.Error", original, got)
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
