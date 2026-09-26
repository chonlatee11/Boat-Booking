package httpx

import (
	"encoding/json"
	"errors"
	"net/http"

	"connectrpc.com/connect"
)

type errorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// WriteError writes err as a JSON error body with an HTTP status derived from
// connect's error code (D-44). Internal/Unknown codes never leak error detail.
func WriteError(w http.ResponseWriter, err error) {
	code := connect.CodeOf(err)

	status := http.StatusInternalServerError
	switch code {
	case connect.CodeUnauthenticated:
		status = http.StatusUnauthorized
	case connect.CodePermissionDenied:
		status = http.StatusForbidden
	case connect.CodeInvalidArgument:
		status = http.StatusBadRequest
	case connect.CodeNotFound:
		status = http.StatusNotFound
	case connect.CodeAlreadyExists:
		status = http.StatusConflict
	case connect.CodeUnavailable:
		status = http.StatusServiceUnavailable
	}

	message := "internal error"
	if code != connect.CodeInternal && code != connect.CodeUnknown {
		var connErr *connect.Error
		if errors.As(err, &connErr) {
			message = connErr.Message()
		} else {
			message = err.Error()
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(errorBody{Code: code.String(), Message: message})
}
