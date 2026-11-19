package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/kelvins-io/agent-market/internal/a2a"
	"github.com/kelvins-io/agent-market/internal/discovery"
	"github.com/kelvins-io/agent-market/internal/geo"
	"github.com/kelvins-io/agent-market/internal/hostagent"
	"github.com/kelvins-io/agent-market/internal/models"
	"github.com/kelvins-io/agent-market/internal/registry"
)

// Server exposes the Market Gateway HTTP API.
type Server struct {
	Store      *registry.Store
	Discovery  *discovery.Service
	Host       *hostagent.Host
	StaticDir  string
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/v1/agents/register", s.handleRegister)
	mux.HandleFunc("DELETE /api/v1/agents/{id}", s.handleDelete)
	mux.HandleFunc("GET /api/v1/agents", s.handleList)
	mux.HandleFunc("GET /api/v1/agents/discover", s.handleDiscover)
	mux.HandleFunc("GET /api/v1/agents/{id}", s.handleGet)
	mux.HandleFunc("GET /api/v1/agents/{id}/card", s.handleCard)
	mux.HandleFunc("POST /api/v1/chat", s.handleChat)
	mux.HandleFunc("POST /api/v1/a2a/{id}/message:send", s.handleA2ASend)
	mux.HandleFunc("GET /api/v1/geo/reverse", s.handleGeoReverse)
	mux.HandleFunc("GET /api/v1/health", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	if s.StaticDir != "" {
		fs := http.FileServer(http.Dir(s.StaticDir))
		mux.Handle("/", spaHandler(s.StaticDir, fs))
	}
	return withCORS(mux)
}

func (s *Server) handleGeoReverse(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	lat, err1 := strconv.ParseFloat(q.Get("lat"), 64)
	lng, err2 := strconv.ParseFloat(q.Get("lng"), 64)
	if err1 != nil || err2 != nil {
		writeErr(w, http.StatusBadRequest, "lat and lng are required")
		return
	}
	writeJSON(w, http.StatusOK, geo.ReversePlace(lat, lng))
}

func (s *Server) handleRegister(w http.ResponseWriter, r *http.Request) {
	var req models.RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	rec, err := s.Store.Upsert(r.Context(), req)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, rec)
}

func (s *Server) handleDelete(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := s.Store.Delete(r.Context(), id); err != nil {
		writeErr(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"deleted": id})
}

func (s *Server) handleList(w http.ResponseWriter, r *http.Request) {
	onlineOnly := r.URL.Query().Get("online") != "false"
	list, err := s.Store.List(r.Context(), onlineOnly)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	if list == nil {
		list = []models.AgentRecord{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"agents": list})
}

func (s *Server) handleGet(w http.ResponseWriter, r *http.Request) {
	rec, err := s.Store.Get(r.Context(), r.PathValue("id"))
	if err != nil {
		writeErr(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, rec)
}

func (s *Server) handleDiscover(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	lat, _ := strconv.ParseFloat(q.Get("lat"), 64)
	lng, _ := strconv.ParseFloat(q.Get("lng"), 64)
	radius, _ := strconv.ParseFloat(q.Get("radius_km"), 64)
	if radius <= 0 {
		radius = 50
	}
	var tags []string
	if t := q.Get("tags"); t != "" {
		for _, p := range strings.Split(t, ",") {
			p = strings.TrimSpace(p)
			if p != "" {
				tags = append(tags, p)
			}
		}
	}
	results, err := s.Discovery.Discover(r.Context(), models.DiscoverQuery{
		Lat:      lat,
		Lng:      lng,
		RadiusKm: radius,
		Category: q.Get("category"),
		Tags:     tags,
		Q:        q.Get("q"),
	})
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	if results == nil {
		results = []models.AgentWithDistance{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"agents": results})
}

func (s *Server) handleCard(w http.ResponseWriter, r *http.Request) {
	rec, err := s.Store.Get(r.Context(), r.PathValue("id"))
	if err != nil {
		writeErr(w, http.StatusNotFound, err.Error())
		return
	}
	if len(rec.AgentCard) > 0 {
		writeJSON(w, http.StatusOK, rec.AgentCard)
		return
	}
	card, err := a2a.FetchAgentCardJSON(r.Context(), rec.A2AURL, rec.HandlerPath)
	if err != nil {
		writeErr(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, card)
}

func (s *Server) handleChat(w http.ResponseWriter, r *http.Request) {
	var req models.ChatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	if strings.TrimSpace(req.Message) == "" {
		writeErr(w, http.StatusBadRequest, "message is required")
		return
	}
	if s.Host == nil {
		writeErr(w, http.StatusServiceUnavailable, "host agent unavailable: set OPENAI_API_KEY")
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeErr(w, http.StatusInternalServerError, "streaming unsupported")
		return
	}

	lat, lng := 39.9042, 116.4074 // Beijing default
	if req.Lat != nil {
		lat = *req.Lat
	}
	if req.Lng != nil {
		lng = *req.Lng
	}

	err := s.Host.ChatSSE(r.Context(), req.Message, lat, lng, func(event string, data any) {
		payload, _ := json.Marshal(map[string]any{"event": event, "data": data})
		_, _ = w.Write([]byte("data: " + string(payload) + "\n\n"))
		flusher.Flush()
	})
	if err != nil {
		payload, _ := json.Marshal(map[string]any{"event": "error", "data": err.Error()})
		_, _ = w.Write([]byte("data: " + string(payload) + "\n\n"))
		flusher.Flush()
	}
	_, _ = w.Write([]byte("data: " + `{"event":"done"}` + "\n\n"))
	flusher.Flush()
}

func (s *Server) handleA2ASend(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var req models.A2ASendRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	rec, err := s.Store.Get(r.Context(), id)
	if err != nil {
		writeErr(w, http.StatusNotFound, err.Error())
		return
	}
	reply, err := a2a.SendText(r.Context(), a2a.SendOptions{
		BaseURL:     rec.A2AURL,
		HandlerPath: rec.HandlerPath,
		Message:     req.Message,
		AgentCard:   rec.AgentCard,
	})
	if err != nil {
		writeErr(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{
		"agent_id": id,
		"reply":    reply,
	})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func spaHandler(dir string, fs http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") {
			http.NotFound(w, r)
			return
		}
		fs.ServeHTTP(w, r)
	})
}
