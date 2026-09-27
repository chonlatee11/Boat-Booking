package httpadapter

import (
	"errors"
	"testing"

	"connectrpc.com/connect"
)

// TestToConnectErrRedactsUnmappedErrors proves the WR-09 fix: an error that
// matches none of the domain sentinels must never put its own message on
// the wire. connect-go encodes err.Error() into the response for every
// code, including CodeInternal, and the gateway's admin/public proxies
// forward that body to the browser unchanged -- the only safe place to
// redact is at the source.
func TestToConnectErrRedactsUnmappedErrors(t *testing.T) {
	original := errors.New(`app: insert pier: ERROR: new row violates check constraint "piers_lat_check"`)
	got := toConnectErr(original)

	var ce *connect.Error
	if !errors.As(got, &ce) {
		t.Fatalf("toConnectErr(%v) = %v, not a *connect.Error", original, got)
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
