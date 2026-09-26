// Package httpadapter wires services/_template's HTTP routes. The ping
// slice below is the template's executable example: it exercises the full
// domain -> app -> postgres -> outbox loop over one real endpoint. Real
// services delete this route and add their own — plan 10's CLAUDE.md lists
// the exact files to remove.
package httpadapter

import (
	"encoding/json"
	"errors"
	"net/http"

	"connectrpc.com/connect"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/chonlatee11/boat-booking/pkg/httpx"
	bbpgx "github.com/chonlatee11/boat-booking/pkg/pgx"
	"github.com/chonlatee11/boat-booking/services/__NAME__/internal/app"
	"github.com/chonlatee11/boat-booking/services/__NAME__/internal/domain"
)

// maxPingBodyBytes bounds POST /v1/pings request bodies (T-09-03).
const maxPingBodyBytes = 4 * 1024

// Routes mounts this service's protected routes onto r. pool is the
// service's Postgres pool; nudge wakes the outbox relay immediately after a
// write instead of waiting for its next poll tick (a no-op when the relay is
// disabled).
func Routes(r chi.Router, pool *pgxpool.Pool, nudge func()) {
	r.With(httpx.RequireClaims).Post("/v1/pings", handleCreatePing(pool, nudge))
}

type createPingRequest struct {
	Note string `json:"note"`
}

type createPingResponse struct {
	ID string `json:"id"`
}

func handleCreatePing(pool *pgxpool.Pool, nudge func()) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims, _ := httpx.FromContext(r.Context())
		operatorID, err := uuid.Parse(claims.OperatorID)
		if err != nil {
			httpx.WriteError(w, connect.NewError(connect.CodeInvalidArgument, errors.New("invalid or missing operator id")))
			return
		}

		var req createPingRequest
		r.Body = http.MaxBytesReader(w, r.Body, maxPingBodyBytes)
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			httpx.WriteError(w, connect.NewError(connect.CodeInvalidArgument, errors.New("invalid request body")))
			return
		}

		var ping domain.Ping
		err = bbpgx.WithTx(r.Context(), pool, func(tx pgx.Tx) error {
			var txErr error
			ping, txErr = app.RecordPing(r.Context(), tx, operatorID, req.Note)
			return txErr
		})
		if err != nil {
			if errors.Is(err, domain.ErrInvalidArgument) {
				httpx.WriteError(w, connect.NewError(connect.CodeInvalidArgument, err))
				return
			}
			httpx.WriteError(w, connect.NewError(connect.CodeInternal, err))
			return
		}
		nudge()

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(createPingResponse{ID: ping.ID.String()})
	}
}
