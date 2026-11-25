package admin

import (
	"embed"
	"encoding/json"
	"errors"
	"io/fs"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/asmisnik/mobile-api/internal/db"
)

//go:embed web
var files embed.FS

type server struct {
	catalog *db.AdminDB
}

func Handler(catalog *db.AdminDB) http.Handler {
	s := server{catalog: catalog}
	web, err := fs.Sub(files, "web")
	if err != nil {
		panic(err)
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/flats", s.flats)
	mux.HandleFunc("GET /api/flats/{id}", s.detail)
	mux.HandleFunc("GET /api/options", s.options)
	mux.HandleFunc("GET /api/analytics", s.analytics)
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) { w.Write([]byte("ok\n")) })
	mux.Handle("GET /", http.FileServerFS(web))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !loopbackHost(r.Host) {
			http.Error(w, "invalid host", http.StatusForbidden)
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; img-src 'self' https: http:; base-uri 'none'; frame-ancestors 'none'; form-action 'self'")
		mux.ServeHTTP(w, r)
	})
}

func loopbackHost(host string) bool {
	if name, _, err := net.SplitHostPort(host); err == nil {
		host = name
	}
	if host == "localhost" || host == "[::1]" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func (s server) flats(w http.ResponseWriter, r *http.Request) {
	filter, err := readFilter(r.URL.Query())
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	result, err := s.catalog.List(r.Context(), filter)
	s.respond(w, result, err)
}

func (s server) detail(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil || id < 1 {
		s.respond(w, nil, db.ErrNotFound)
		return
	}
	result, err := s.catalog.Detail(r.Context(), id)
	s.respond(w, result, err)
}

func (s server) options(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, locationOptions())
}

func (s server) analytics(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	dealType := q.Get("deal_type")
	if dealType == "" {
		dealType = "rent"
	}
	var days, region int
	if err := integer(q, "days", &days); err != nil || days > 366 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Некорректный период"})
		return
	}
	if err := integer(q, "region", &region); err != nil || (dealType != "rent" && dealType != "sale") {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Некорректный регион или тип сделки"})
		return
	}
	result, err := s.catalog.Metrics(r.Context(), dealType, region, days, time.Now())
	s.respond(w, result, err)
}

func (s server) respond(w http.ResponseWriter, result any, err error) {
	switch {
	case err == nil:
		writeJSON(w, http.StatusOK, result)
	case errors.Is(err, db.ErrNotFound):
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "Объявление не найдено"})
	default:
		slog.Error("admin catalog request", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Не удалось прочитать данные"})
	}
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		slog.Warn("writing admin response", "error", err)
	}
}
