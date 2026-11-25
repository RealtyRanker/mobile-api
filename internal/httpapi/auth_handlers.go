package httpapi

import (
	"net/http"
	"time"

	"go.uber.org/zap"

	"github.com/asmisnik/mobile-api/internal/auth"
	"github.com/asmisnik/mobile-api/internal/metrics"
)

type deviceSignInRequest struct {
	// DeviceID is a random identifier the app generates once and persists
	// in the Keychain — there's no external identity provider behind it.
	// This is a deliberate simplification for a single-user personal app:
	// Sign in with Apple requires a paid Apple Developer Program
	// membership (a free personal team can't enable that capability), which
	// isn't worth it just to run the app on your own iPhone.
	DeviceID    string `json:"device_id"`
	DisplayName string `json:"display_name"`
}

type sessionResponse struct {
	AccessToken     string    `json:"access_token"`
	AccessExpiresAt time.Time `json:"access_expires_at"`
	RefreshToken    string    `json:"refresh_token"`
	User            userDTO   `json:"user"`
}

type userDTO struct {
	ID          int       `json:"id"`
	Email       string    `json:"email"`
	DisplayName string    `json:"display_name"`
	CreatedAt   time.Time `json:"created_at"`
}

func (s *Server) handleDeviceSignIn(w http.ResponseWriter, r *http.Request) {
	var req deviceSignInRequest
	if err := readJSON(r, &req); err != nil || req.DeviceID == "" {
		metrics.SignInsTotal.WithLabelValues("bad_request").Inc()
		writeError(w, http.StatusBadRequest, "device_id is required")
		return
	}

	userID, err := s.db.UpsertDeviceUser(r.Context(), req.DeviceID, req.DisplayName)
	if err != nil {
		metrics.SignInsTotal.WithLabelValues("db_error").Inc()
		s.logger.Error("upserting device user", zap.Error(err))
		writeError(w, http.StatusInternalServerError, "sign-in failed")
		return
	}

	resp, err := s.issueSession(r, userID)
	if err != nil {
		metrics.SignInsTotal.WithLabelValues("session_error").Inc()
		s.logger.Error("issuing session", zap.Error(err))
		writeError(w, http.StatusInternalServerError, "sign-in failed")
		return
	}

	metrics.SignInsTotal.WithLabelValues("ok").Inc()
	writeJSON(w, http.StatusOK, resp)
}

type refreshRequest struct {
	RefreshToken string `json:"refresh_token"`
}

func (s *Server) handleRefresh(w http.ResponseWriter, r *http.Request) {
	var req refreshRequest
	if err := readJSON(r, &req); err != nil || req.RefreshToken == "" {
		writeError(w, http.StatusBadRequest, "refresh_token is required")
		return
	}

	hash := auth.HashRefreshToken(req.RefreshToken)
	userID, err := s.db.UserIDForRefreshToken(r.Context(), hash)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "invalid or expired refresh token")
		return
	}

	// Rotate: the old refresh token is single-use.
	if err := s.db.RevokeRefreshToken(r.Context(), hash); err != nil {
		s.logger.Error("revoking rotated refresh token", zap.Error(err))
	}

	resp, err := s.issueSession(r, userID)
	if err != nil {
		s.logger.Error("issuing session", zap.Error(err))
		writeError(w, http.StatusInternalServerError, "refresh failed")
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	var req refreshRequest
	if err := readJSON(r, &req); err == nil && req.RefreshToken != "" {
		_ = s.db.RevokeRefreshToken(r.Context(), auth.HashRefreshToken(req.RefreshToken))
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) issueSession(r *http.Request, userID int) (sessionResponse, error) {
	accessToken, expiresAt, err := s.sessions.NewAccessToken(userID)
	if err != nil {
		return sessionResponse{}, err
	}
	rawRefresh, refreshHash, err := auth.NewRefreshToken()
	if err != nil {
		return sessionResponse{}, err
	}
	if err := s.db.CreateRefreshToken(r.Context(), userID, refreshHash, time.Now().Add(s.refreshTokenTTL)); err != nil {
		return sessionResponse{}, err
	}
	user, err := s.db.GetUser(r.Context(), userID)
	if err != nil {
		return sessionResponse{}, err
	}
	return sessionResponse{
		AccessToken:     accessToken,
		AccessExpiresAt: expiresAt,
		RefreshToken:    rawRefresh,
		User:            userDTO{ID: user.ID, Email: user.Email, DisplayName: user.DisplayName, CreatedAt: user.CreatedAt},
	}, nil
}
