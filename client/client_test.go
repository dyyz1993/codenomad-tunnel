package client

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestParseHostPort(t *testing.T) {
	tests := []struct {
		input    string
		wantHost string
		wantPort int
	}{
		{"http://127.0.0.1:8888", "127.0.0.1", 8888},
		{"https://localhost:3000", "localhost", 3000},
		{"tcp://192.168.1.1:3306", "192.168.1.1", 3306},
		{"http://example.com", "example.com", 80},
		{"https://example.com", "example.com", 443},
		{"ws://localhost:9000/path", "localhost", 9000},
	}

	for _, tt := range tests {
		host, port := parseHostPort(tt.input)
		if host != tt.wantHost || port != tt.wantPort {
			t.Errorf("parseHostPort(%q) = (%s, %d), want (%s, %d)", tt.input, host, port, tt.wantHost, tt.wantPort)
		}
	}
}

func TestProbeLocalHTTP(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	}))
	defer srv.Close()

	httpClient := &http.Client{Timeout: 3 * time.Second}
	result := ProbeLocal(srv.URL, httpClient)

	if result.Type != ServiceTypeHTTP {
		t.Errorf("expected HTTP, got %s", result.Type)
	}
}

func TestProbeLocalTCP(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	addr := strings.TrimPrefix(srv.URL, "http://")
	result := probeTCP("http://" + addr)
	if result == nil || result.Type != ServiceTypeTCP {
		t.Errorf("expected TCP probe to succeed for %s", addr)
	}
}

func TestProbeLocalUnreachable(t *testing.T) {
	httpClient := &http.Client{Timeout: 1 * time.Second}
	result := ProbeLocal("http://127.0.0.1:1", httpClient)

	if result.Type != ServiceTypeUnknown {
		t.Errorf("expected unknown for unreachable service, got %s", result.Type)
	}
}

func TestForwardHTTP(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/api/test" {
			t.Errorf("expected path /api/test, got %s", r.URL.Path)
		}
		if r.URL.RawQuery != "foo=bar" {
			t.Errorf("expected query foo=bar, got %s", r.URL.RawQuery)
		}
		if r.Header.Get("X-Custom") != "value" {
			t.Errorf("expected X-Custom=value header")
		}

		body, _ := io.ReadAll(r.Body)
		if string(body) != `{"hello":"world"}` {
			t.Errorf("unexpected body: %s", string(body))
		}

		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Response", "ok")
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte(`{"status":"ok"}`))
	}))
	defer srv.Close()

	httpClient := &http.Client{Timeout: 5 * time.Second}
	req := &RelayRequest{
		Type:    "request",
		ID:      "r_test123",
		Method:  "POST",
		Path:    "/api/test",
		Query:   "foo=bar",
		Headers: map[string]string{"X-Custom": "value"},
		Body:    `{"hello":"world"}`,
	}

	resp := ForwardHTTP(srv.URL, httpClient, req)

	if resp.Type != "response" {
		t.Errorf("expected type response, got %s", resp.Type)
	}
	if resp.ID != "r_test123" {
		t.Errorf("expected ID r_test123, got %s", resp.ID)
	}
	if resp.Status != http.StatusCreated {
		t.Errorf("expected status %d, got %d", http.StatusCreated, resp.Status)
	}
	if resp.Headers["X-Response"] != "ok" {
		t.Errorf("expected X-Response=ok, got %s", resp.Headers["X-Response"])
	}
	if resp.Body != `{"status":"ok"}` {
		t.Errorf("unexpected body: %s", resp.Body)
	}
}

func TestForwardHTTPBinary(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		w.Write([]byte{0x89, 0x50, 0x4E, 0x47})
	}))
	defer srv.Close()

	httpClient := &http.Client{Timeout: 5 * time.Second}
	req := &RelayRequest{
		Type:   "request",
		ID:     "r_bin1",
		Method: "GET",
		Path:   "/img.png",
	}

	resp := ForwardHTTP(srv.URL, httpClient, req)

	if resp.BodyBase64 == "" {
		t.Error("expected bodyBase64 for binary content")
	}
	if resp.Body != "" {
		t.Error("expected empty body for binary content")
	}
}

func TestForwardHTTPError(t *testing.T) {
	httpClient := &http.Client{Timeout: 1 * time.Second}
	req := &RelayRequest{
		Type:   "request",
		ID:     "r_err1",
		Method: "GET",
		Path:   "/",
	}

	resp := ForwardHTTP("http://127.0.0.1:1", httpClient, req)

	if resp.Status != http.StatusBadGateway {
		t.Errorf("expected 502 for unreachable, got %d", resp.Status)
	}
	if resp.Type != "response" {
		t.Errorf("expected type response, got %s", resp.Type)
	}
}

func TestRelayRequestJSON(t *testing.T) {
	req := &RelayRequest{
		Type:    "request",
		ID:      "r_abc",
		Method:  "GET",
		Path:    "/",
		Query:   "",
		Headers: map[string]string{"Host": "example.com"},
		Body:    "",
	}

	data, err := json.Marshal(req)
	if err != nil {
		t.Fatal(err)
	}

	var decoded RelayRequest
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}

	if decoded.Type != "request" || decoded.ID != "r_abc" || decoded.Method != "GET" {
		t.Errorf("roundtrip mismatch: %+v", decoded)
	}
}

func TestRelayResponseJSON(t *testing.T) {
	resp := &RelayResponse{
		Type:    "response",
		ID:      "r_abc",
		Status:  200,
		Headers: map[string]string{"Content-Type": "text/html"},
		Body:    "<h1>Hello</h1>",
	}

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatal(err)
	}

	var decoded RelayResponse
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}

	if decoded.Type != "response" || decoded.Status != 200 || decoded.Body != "<h1>Hello</h1>" {
		t.Errorf("roundtrip mismatch: %+v", decoded)
	}
}

func TestClientNew(t *testing.T) {
	cfg := &Config{
		HubURL:   "https://api.example.com:8443",
		LocalURL: "http://127.0.0.1:8888",
		Insecure: true,
	}

	c, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}

	if c.localHost != "127.0.0.1" {
		t.Errorf("expected host 127.0.0.1, got %s", c.localHost)
	}
	if c.localPort != 8888 {
		t.Errorf("expected port 8888, got %d", c.localPort)
	}
	if c.httpBase != "http://127.0.0.1:8888" {
		t.Errorf("expected httpBase http://127.0.0.1:8888, got %s", c.httpBase)
	}
}

func TestClientNewTCP(t *testing.T) {
	cfg := &Config{
		HubURL:   "https://api.example.com:8443",
		LocalURL: "tcp://127.0.0.1:3306",
		Insecure: true,
	}

	c, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}

	if c.httpBase != "http://127.0.0.1:3306" {
		t.Errorf("expected httpBase http://127.0.0.1:3306, got %s", c.httpBase)
	}
}

func TestClientShutdown(t *testing.T) {
	cfg := &Config{
		HubURL:   "https://api.example.com:8443",
		LocalURL: "http://127.0.0.1:8888",
	}

	c, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}

	c.Shutdown()

	select {
	case <-c.done:
	default:
		t.Error("expected done channel to be closed after shutdown")
	}

	c.Shutdown()
}
