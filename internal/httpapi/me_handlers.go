package httpapi

import "net/http"

func (s *Server) handleGetMe(w http.ResponseWriter, r *http.Request, userID int) {
	user, err := s.db.GetUser(r.Context(), userID)
	if err != nil {
		writeError(w, http.StatusNotFound, "user not found")
		return
	}
	writeJSON(w, http.StatusOK, user)
}

// handleDeleteMe permanently deletes the account and everything derived
// from it. Required for App Store review when an app offers Sign in with
// Apple account creation (guideline 5.1.1(v)): the app must let a person
// delete their account, not just sign out.
func (s *Server) handleDeleteMe(w http.ResponseWriter, r *http.Request, userID int) {
	if err := s.db.DeleteUser(r.Context(), userID); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete account")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
