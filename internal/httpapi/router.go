package httpapi

import (
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"

	"github.com/asmisnik/mobile-api/internal/auth"
	"github.com/asmisnik/mobile-api/internal/db"
)

type Server struct {
	db       *db.DB
	sessions *auth.SessionIssuer
	logger   *zap.Logger

	refreshTokenTTL time.Duration
}

func NewServer(database *db.DB, sessions *auth.SessionIssuer, refreshTokenTTL time.Duration, logger *zap.Logger) *Server {
	return &Server{db: database, sessions: sessions, refreshTokenTTL: refreshTokenTTL, logger: logger}
}

func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("POST /v1/auth/device", withMetrics("auth_device", s.handleDeviceSignIn))
	mux.HandleFunc("POST /v1/auth/refresh", withMetrics("auth_refresh", s.handleRefresh))
	mux.HandleFunc("POST /v1/auth/logout", withMetrics("auth_logout", s.handleLogout))

	mux.HandleFunc("GET /v1/me", withMetrics("me_get", s.withAuth(s.handleGetMe)))
	mux.HandleFunc("DELETE /v1/me", withMetrics("me_delete", s.withAuth(s.handleDeleteMe)))

	mux.HandleFunc("GET /v1/subscriptions", withMetrics("subscriptions_list", s.withAuth(s.handleListSubscriptions)))
	mux.HandleFunc("POST /v1/subscriptions", withMetrics("subscriptions_create", s.withAuth(s.handleCreateSubscription)))
	mux.HandleFunc("GET /v1/subscriptions/{id}", withMetrics("subscriptions_get", s.withAuth(s.handleGetSubscription)))
	mux.HandleFunc("PUT /v1/subscriptions/{id}", withMetrics("subscriptions_update", s.withAuth(s.handleUpdateSubscription)))
	mux.HandleFunc("DELETE /v1/subscriptions/{id}", withMetrics("subscriptions_delete", s.withAuth(s.handleDeleteSubscription)))
	mux.HandleFunc("PATCH /v1/subscriptions/{id}/active", withMetrics("subscriptions_toggle", s.withAuth(s.handleToggleSubscription)))
	mux.HandleFunc("GET /v1/subscriptions/{id}/flats", withMetrics("subscriptions_feed", s.withAuth(s.handleSubscriptionFeed)))

	mux.HandleFunc("GET /v1/flats/{id}", withMetrics("flats_get", s.withAuth(s.handleGetFlat)))

	mux.HandleFunc("GET /v1/regions", withMetrics("meta_regions", s.handleRegions))
	mux.HandleFunc("GET /v1/metro/lines", withMetrics("meta_metro_lines", s.handleMetroLines))
	mux.HandleFunc("GET /v1/metro/okrugs", withMetrics("meta_metro_okrugs", s.handleMetroOkrugs))
	mux.HandleFunc("GET /v1/metro/stations", withMetrics("meta_metro_stations", s.handleMetroStations))

	mux.Handle("/metrics", promhttp.Handler())
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	return mux
}
