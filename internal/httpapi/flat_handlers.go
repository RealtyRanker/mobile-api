package httpapi

import (
	"net/http"
	"strconv"

	"go.uber.org/zap"

	"github.com/asmisnik/mobile-api/internal/model"
)

type feedResponse struct {
	Flats      []model.Flat `json:"flats"`
	NextCursor string       `json:"next_cursor,omitempty"`
}

func (s *Server) handleSubscriptionFeed(w http.ResponseWriter, r *http.Request, userID int) {
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

	limit := 0
	if v := r.URL.Query().Get("limit"); v != "" {
		limit, _ = strconv.Atoi(v)
	}
	cursor := r.URL.Query().Get("cursor")

	flats, next, err := s.db.Feed(r.Context(), *sub, cursor, limit)
	if err != nil {
		s.logger.Error("building feed", zap.Error(err))
		writeError(w, http.StatusInternalServerError, "failed to load flats")
		return
	}
	if flats == nil {
		flats = []model.Flat{}
	}

	if cursor == "" {
		if err := s.db.TouchLastViewed(r.Context(), userID, id); err != nil {
			s.logger.Warn("touching last viewed", zap.Error(err))
		}
	}

	writeJSON(w, http.StatusOK, feedResponse{Flats: flats, NextCursor: next})
}

func (s *Server) handleGetFlat(w http.ResponseWriter, r *http.Request, userID int) {
	id, ok := pathInt(r, "id")
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	flat, err := s.db.GetFlat(r.Context(), id)
	if err != nil {
		respondNotFoundOr500(w, s.logger, err, "loading flat")
		return
	}
	writeJSON(w, http.StatusOK, flat)
}
