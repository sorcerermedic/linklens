package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/portfolio-author/linklens/internal/analytics"
	"github.com/portfolio-author/linklens/internal/ratelimit"
	"github.com/portfolio-author/linklens/internal/shortener"
	"github.com/portfolio-author/linklens/internal/store"
)

type createRequest struct {
	URL       string     `json:"url"`
	Alias     string     `json:"alias,omitempty"`
	ExpiresAt *time.Time `json:"expiresAt,omitempty"`
}

type linkResponse struct {
	Code      string     `json:"code"`
	ShortURL  string     `json:"shortUrl"`
	URL       string     `json:"url"`
	Custom    bool       `json:"custom"`
	CreatedAt time.Time  `json:"createdAt"`
	ExpiresAt *time.Time `json:"expiresAt,omitempty"`
	Expired   bool       `json:"expired"`
	Clicks    int64      `json:"clicks"`
}

func (s *Server) toResponse(l store.Link) linkResponse {
	return linkResponse{
		Code:      l.Code,
		ShortURL:  s.cfg.BaseURL + "/" + l.Code,
		URL:       l.URL,
		Custom:    l.Custom,
		CreatedAt: l.CreatedAt,
		ExpiresAt: l.ExpiresAt,
		Expired:   l.Expired(s.cfg.Clock()),
		Clicks:    l.Clicks,
	}
}

func (s *Server) handleCreate(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	var req createRequest
	if err := dec.Decode(&req); err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			writeError(w, http.StatusRequestEntityTooLarge, "request body too large")
			return
		}
		writeError(w, http.StatusBadRequest, "invalid JSON body: "+err.Error())
		return
	}
	link, err := s.cfg.Service.Create(r.Context(), shortener.CreateRequest{
		URL: req.URL, Alias: req.Alias, ExpiresAt: req.ExpiresAt,
	})
	if err != nil {
		s.writeServiceError(w, err)
		return
	}
	w.Header().Set("Location", "/api/links/"+link.Code+"/stats")
	writeJSON(w, http.StatusCreated, s.toResponse(link))
}

func (s *Server) handleList(w http.ResponseWriter, r *http.Request) {
	links, err := s.cfg.Service.List(r.Context())
	if err != nil {
		s.writeServiceError(w, err)
		return
	}
	out := make([]linkResponse, 0, len(links))
	for _, l := range links {
		out = append(out, s.toResponse(l))
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleStats(w http.ResponseWriter, r *http.Request) {
	st, err := s.cfg.Service.Stats(r.Context(), chi.URLParam(r, "code"))
	if err != nil {
		s.writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, st)
}

func (s *Server) handleDelete(w http.ResponseWriter, r *http.Request) {
	if err := s.cfg.Service.Delete(r.Context(), chi.URLParam(r, "code")); err != nil {
		s.writeServiceError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleRedirect(w http.ResponseWriter, r *http.Request) {
	code := chi.URLParam(r, "code")
	if !shortener.IsValidCode(code) {
		writeError(w, http.StatusNotFound, "link not found")
		return
	}
	link, err := s.cfg.Service.Resolve(r.Context(), code)
	if err != nil {
		s.writeServiceError(w, err)
		return
	}
	s.cfg.Recorder.Record(analytics.Event{
		Code:        link.Code,
		At:          s.cfg.Clock(),
		IP:          ratelimit.ClientIP(s.cfg.TrustProxy)(r),
		UserAgent:   r.UserAgent(),
		Referrer:    r.Referer(),
		CountryHint: r.Header.Get("CF-IPCountry"),
	})
	s.cfg.Metrics.Redirect()

	w.Header().Set("Cache-Control", "private, no-store")
	http.Redirect(w, r, link.URL, http.StatusFound)
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	checks := make(map[string]string, len(s.cfg.HealthChecks))
	status := http.StatusOK
	for name, p := range s.cfg.HealthChecks {
		if err := p.Ping(ctx); err != nil {
			checks[name] = "down: " + err.Error()
			status = http.StatusServiceUnavailable
			continue
		}
		checks[name] = "ok"
	}
	overall := "ok"
	if status != http.StatusOK {
		overall = "degraded"
	}
	writeJSON(w, status, map[string]any{"status": overall, "checks": checks})
}

func (s *Server) writeServiceError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, shortener.ErrNotFound):
		writeError(w, http.StatusNotFound, "link not found")
	case errors.Is(err, shortener.ErrExpired):
		writeError(w, http.StatusGone, err.Error())
	case errors.Is(err, shortener.ErrAliasTaken):
		writeError(w, http.StatusConflict, err.Error())
	case errors.Is(err, shortener.ErrInvalidURL),
		errors.Is(err, shortener.ErrUnsupportedScheme),
		errors.Is(err, shortener.ErrPrivateHost),
		errors.Is(err, shortener.ErrURLTooLong),
		errors.Is(err, shortener.ErrInvalidAlias),
		errors.Is(err, shortener.ErrReservedAlias),
		errors.Is(err, shortener.ErrInvalidExpiry):
		writeError(w, http.StatusBadRequest, err.Error())
	default:
		s.cfg.Logger.Error("request failed", "error", err)
		writeError(w, http.StatusInternalServerError, "internal error")
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
