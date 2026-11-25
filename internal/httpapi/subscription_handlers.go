package httpapi

import (
	"errors"
	"net/http"
	"strconv"

	"go.uber.org/zap"

	"github.com/asmisnik/mobile-api/internal/db"
	"github.com/asmisnik/mobile-api/internal/model"
)

func (s *Server) handleListSubscriptions(w http.ResponseWriter, r *http.Request, userID int) {
	subs, err := s.db.ListSubscriptions(r.Context(), userID)
	if err != nil {
		s.logger.Error("listing subscriptions", zap.Error(err))
		writeError(w, http.StatusInternalServerError, "failed to list saved searches")
		return
	}
	if subs == nil {
		subs = []model.Subscription{}
	}
	writeJSON(w, http.StatusOK, subs)
}

func (s *Server) handleCreateSubscription(w http.ResponseWriter, r *http.Request, userID int) {
	var sub model.Subscription
	if err := readJSON(r, &sub); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if msg, ok := validateSubscription(sub); !ok {
		writeError(w, http.StatusBadRequest, msg)
		return
	}

	id, err := s.db.CreateSubscription(r.Context(), userID, sub)
	if err != nil {
		s.logger.Error("creating subscription", zap.Error(err))
		writeError(w, http.StatusInternalServerError, "failed to create saved search")
		return
	}

	created, err := s.db.GetSubscription(r.Context(), userID, id)
	if err != nil {
		s.logger.Error("loading created subscription", zap.Error(err))
		writeError(w, http.StatusInternalServerError, "saved search created but failed to load")
		return
	}
	writeJSON(w, http.StatusCreated, created)
}

func (s *Server) handleGetSubscription(w http.ResponseWriter, r *http.Request, userID int) {
	id, ok := pathInt(r, "id")
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	sub, err := s.db.GetSubscription(r.Context(), userID, id)
	if err != nil {
		respondNotFoundOr500(w, s.logger, err, "loading saved search")
		return
	}
	writeJSON(w, http.StatusOK, sub)
}

func (s *Server) handleUpdateSubscription(w http.ResponseWriter, r *http.Request, userID int) {
	id, ok := pathInt(r, "id")
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	var sub model.Subscription
	if err := readJSON(r, &sub); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if msg, ok := validateSubscription(sub); !ok {
		writeError(w, http.StatusBadRequest, msg)
		return
	}

	if err := s.db.UpdateSubscription(r.Context(), userID, id, sub); err != nil {
		respondNotFoundOr500(w, s.logger, err, "updating saved search")
		return
	}
	updated, err := s.db.GetSubscription(r.Context(), userID, id)
	if err != nil {
		respondNotFoundOr500(w, s.logger, err, "loading updated saved search")
		return
	}
	writeJSON(w, http.StatusOK, updated)
}

func (s *Server) handleDeleteSubscription(w http.ResponseWriter, r *http.Request, userID int) {
	id, ok := pathInt(r, "id")
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	if err := s.db.DeleteSubscription(r.Context(), userID, id); err != nil {
		respondNotFoundOr500(w, s.logger, err, "deleting saved search")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type toggleRequest struct {
	IsActive bool `json:"is_active"`
}

func (s *Server) handleToggleSubscription(w http.ResponseWriter, r *http.Request, userID int) {
	id, ok := pathInt(r, "id")
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	var req toggleRequest
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := s.db.SetSubscriptionActive(r.Context(), userID, id, req.IsActive); err != nil {
		respondNotFoundOr500(w, s.logger, err, "toggling saved search")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func pathInt(r *http.Request, key string) (int, bool) {
	v, err := strconv.Atoi(r.PathValue(key))
	if err != nil {
		return 0, false
	}
	return v, true
}

func respondNotFoundOr500(w http.ResponseWriter, logger *zap.Logger, err error, action string) {
	if errors.Is(err, db.ErrNotFound) {
		writeError(w, http.StatusNotFound, "saved search not found")
		return
	}
	logger.Error(action, zap.Error(err))
	writeError(w, http.StatusInternalServerError, "something went wrong")
}

func validateSubscription(s model.Subscription) (string, bool) {
	if s.DealType != "rent" && s.DealType != "sale" {
		return "deal_type must be 'rent' or 'sale'", false
	}
	if s.Region <= 0 {
		return "region is required", false
	}
	switch s.MinRenovation {
	case "", "cosmetic", "euro", "design":
	default:
		return "min_renovation must be one of '', 'cosmetic', 'euro', 'design'", false
	}
	switch s.BathroomType {
	case "", "separated", "combined":
	default:
		return "bathroom_type must be one of '', 'separated', 'combined'", false
	}
	return "", true
}
