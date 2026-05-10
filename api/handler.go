package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/codenomad/tunnel-hub/tunnel"
)

type Handler struct {
	hub    *tunnel.Hub
	domain string
}

func NewHandler(hub *tunnel.Hub, domain string) *Handler {
	return &Handler{hub: hub, domain: domain}
}

func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/tunnels", h.createTunnel)
	mux.HandleFunc("GET /api/tunnels", h.listTunnels)
	mux.HandleFunc("GET /api/tunnels/{id}", h.getTunnel)
	mux.HandleFunc("DELETE /api/tunnels/{id}", h.deleteTunnel)
	mux.HandleFunc("GET /api/tunnels/{id}/logs", h.streamLogs)
	mux.HandleFunc("GET /api/health", h.health)
}

type createRequest struct {
	Name       string `json:"name"`
	TargetHost string `json:"targetHost"`
	TargetPort int    `json:"targetPort"`
	Subdomain  string `json:"subdomain"`
}

func (h *Handler) createTunnel(w http.ResponseWriter, r *http.Request) {
	var req createRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}

	t, err := h.hub.Create(req.Name, req.TargetHost, req.TargetPort, req.Subdomain)
	if err != nil {
		status := http.StatusInternalServerError
		switch err {
		case tunnel.ErrInvalidSubdomain:
			status = http.StatusBadRequest
		case tunnel.ErrDuplicate:
			status = http.StatusConflict
		}
		writeJSON(w, status, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusCreated, t)
}

func (h *Handler) listTunnels(w http.ResponseWriter, r *http.Request) {
	tunnels := h.hub.List()
	writeJSON(w, http.StatusOK, tunnels)
}

func (h *Handler) getTunnel(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	t, ok := h.hub.Get(id)
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "tunnel not found"})
		return
	}
	writeJSON(w, http.StatusOK, t)
}

func (h *Handler) deleteTunnel(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !h.hub.Delete(id) {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "tunnel not found"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func (h *Handler) streamLogs(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	_, ok := h.hub.Get(id)
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "tunnel not found"})
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	ctx := r.Context()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			t, exists := h.hub.Get(id)
			if !exists {
				return
			}
			fmt.Fprintf(w, "data: {\"requestCount\": %d, \"status\": %q}\n\n", t.RequestCount, t.Status)
			flusher.Flush()
		}
	}
}

func (h *Handler) health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status": "ok",
		"time":   time.Now().UTC().Format(time.RFC3339),
	})
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
