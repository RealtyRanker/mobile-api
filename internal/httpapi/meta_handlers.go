package httpapi

import (
	"net/http"

	"github.com/asmisnik/mobile-api/internal/metro"
	"github.com/asmisnik/mobile-api/internal/model"
	"github.com/asmisnik/mobile-api/internal/region"
)

func (s *Server) handleRegions(w http.ResponseWriter, r *http.Request) {
	entries := region.All()
	out := make([]model.Region, len(entries))
	for i, e := range entries {
		out[i] = model.Region{ID: e.ID, Name: e.Name}
	}
	writeJSON(w, http.StatusOK, out)
}

// handleMetroLines and handleMetroOkrugs serve Moscow-only data (metro
// filtering is only meaningful for region=1), matching the two grouping
// modes the Telegram bot already offers subscribers: by line or by okrug.

func (s *Server) handleMetroLines(w http.ResponseWriter, r *http.Request) {
	out := make([]model.MetroLine, len(metro.Lines))
	for i, l := range metro.Lines {
		out[i] = model.MetroLine{Number: l.Number, Name: l.Name, Emoji: l.Emoji, Stations: l.Stations}
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleMetroOkrugs(w http.ResponseWriter, r *http.Request) {
	out := make([]model.Okrug, len(metro.Okrugs))
	for i, o := range metro.Okrugs {
		out[i] = model.Okrug{Name: o.Name, Stations: o.Stations}
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleMetroStations(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, metro.AllStations())
}
