package httpapi

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/asmisnik/mobile-api/internal/metrics"
)

type ctxKey int

const userIDKey ctxKey = iota

func userIDFromContext(ctx context.Context) (int, bool) {
	id, ok := ctx.Value(userIDKey).(int)
	return id, ok
}

// withAuth requires a valid "Authorization: Bearer <access token>" header
// and injects the resolved user id into the request context.
func (s *Server) withAuth(next func(w http.ResponseWriter, r *http.Request, userID int)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		const prefix = "Bearer "
		if !strings.HasPrefix(authHeader, prefix) {
			writeError(w, http.StatusUnauthorized, "missing bearer token")
			return
		}
		token := strings.TrimPrefix(authHeader, prefix)
		userID, err := s.sessions.VerifyAccessToken(token)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "invalid or expired access token")
			return
		}
		ctx := context.WithValue(r.Context(), userIDKey, userID)
		next(w, r.WithContext(ctx), userID)
	}
}

// withMetrics records a request counter and duration histogram under a
// fixed route label (never the raw URL, to keep cardinality bounded).
func withMetrics(route string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next(rec, r)
		metrics.RequestDuration.WithLabelValues(route).Observe(time.Since(start).Seconds())
		metrics.RequestsTotal.WithLabelValues(route, strconv.Itoa(rec.status)).Inc()
	}
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(status int) {
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}
