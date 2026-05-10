package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/codenomad/tunnel-hub/tunnel"
)

func setupMux() (*tunnel.Hub, *http.ServeMux) {
	hub := tunnel.NewHub("https://tunnel.example.com", "wss://tunnel.example.com")
	handler := NewHandler(hub, "tunnel.example.com")
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)
	return hub, mux
}

func TestHealthEndpoint(t *testing.T) {
	_, mux := setupMux()

	req := httptest.NewRequest("GET", "/api/health", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	var body map[string]interface{}
	json.NewDecoder(w.Body).Decode(&body)

	if body["status"] != "ok" {
		t.Errorf("expected status ok, got %v", body["status"])
	}
}

func TestCreateTunnel(t *testing.T) {
	_, mux := setupMux()

	body := `{"name":"test","targetHost":"localhost","targetPort":8080}`
	req := httptest.NewRequest("POST", "/api/tunnels", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d, body: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	json.NewDecoder(w.Body).Decode(&resp)

	if resp["id"] == "" {
		t.Error("expected non-empty tunnel ID")
	}
	if resp["subdomain"] == "" {
		t.Error("expected non-empty subdomain")
	}
	if resp["publicUrl"] == "" {
		t.Error("expected non-empty publicUrl")
	}
}

func TestListTunnels(t *testing.T) {
	hub, mux := setupMux()

	hub.Create("a", "localhost", 8080, "")
	hub.Create("b", "localhost", 8081, "")

	req := httptest.NewRequest("GET", "/api/tunnels", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	var list []interface{}
	json.NewDecoder(w.Body).Decode(&list)

	if len(list) != 2 {
		t.Errorf("expected 2 tunnels, got %d", len(list))
	}
}

func TestDeleteTunnel(t *testing.T) {
	hub, mux := setupMux()

	created, _ := hub.Create("test", "localhost", 8080, "")

	req := httptest.NewRequest("DELETE", "/api/tunnels/"+created.ID, nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	_, ok := hub.Get(created.ID)
	if ok {
		t.Error("expected tunnel to be deleted")
	}
}

func TestGetTunnel(t *testing.T) {
	hub, mux := setupMux()

	created, _ := hub.Create("test", "localhost", 8080, "")

	req := httptest.NewRequest("GET", "/api/tunnels/"+created.ID, nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestGetTunnelNotFound(t *testing.T) {
	_, mux := setupMux()

	req := httptest.NewRequest("GET", "/api/tunnels/nonexistent", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestCreateTunnelWithSubdomain(t *testing.T) {
	_, mux := setupMux()

	body := `{"name":"test","targetHost":"localhost","targetPort":8080,"subdomain":"my-app"}`
	req := httptest.NewRequest("POST", "/api/tunnels", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d, body: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	json.NewDecoder(w.Body).Decode(&resp)

	if resp["subdomain"] != "my-app" {
		t.Errorf("expected subdomain my-app, got %v", resp["subdomain"])
	}
}

func TestCreateTunnelWithDuplicateSubdomain(t *testing.T) {
	hub, mux := setupMux()

	hub.Create("first", "localhost", 8080, "taken")

	body := `{"name":"second","targetHost":"localhost","targetPort":8081,"subdomain":"taken"}`
	req := httptest.NewRequest("POST", "/api/tunnels", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d, body: %s", w.Code, w.Body.String())
	}
}

func TestCreateTunnelWithInvalidSubdomain(t *testing.T) {
	_, mux := setupMux()

	body := `{"name":"test","targetHost":"localhost","targetPort":8080,"subdomain":"-bad"}`
	req := httptest.NewRequest("POST", "/api/tunnels", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d, body: %s", w.Code, w.Body.String())
	}
}
