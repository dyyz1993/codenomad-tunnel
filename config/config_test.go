package config

import (
	"testing"
)

func TestGetPublicBaseURL_Default(t *testing.T) {
	cfg := &Config{Domain: "tunnel.example.com", HTTPPort: 80}
	got := cfg.GetPublicBaseURL()
	want := "http://tunnel.example.com"
	if got != want {
		t.Errorf("expected %s, got %s", want, got)
	}
}

func TestGetPublicBaseURL_CustomPort(t *testing.T) {
	cfg := &Config{Domain: "tunnel.example.com", HTTPPort: 8080}
	got := cfg.GetPublicBaseURL()
	want := "http://tunnel.example.com:8080"
	if got != want {
		t.Errorf("expected %s, got %s", want, got)
	}
}

func TestGetPublicBaseURL_TLS(t *testing.T) {
	cfg := &Config{Domain: "tunnel.example.com", HTTPPort: 443, TLSCert: "/cert.pem", TLSKey: "/key.pem"}
	got := cfg.GetPublicBaseURL()
	want := "https://tunnel.example.com"
	if got != want {
		t.Errorf("expected %s, got %s", want, got)
	}
}

func TestGetPublicBaseURL_TLSNonStandardPort(t *testing.T) {
	cfg := &Config{Domain: "tunnel.example.com", HTTPPort: 8443, TLSCert: "/cert.pem", TLSKey: "/key.pem"}
	got := cfg.GetPublicBaseURL()
	want := "https://tunnel.example.com:8443"
	if got != want {
		t.Errorf("expected %s, got %s", want, got)
	}
}

func TestGetPublicBaseURL_PublicURLOverride(t *testing.T) {
	cfg := &Config{Domain: "tunnel.example.com", HTTPPort: 8080, PublicURL: "https://tunnel.example.com"}
	got := cfg.GetPublicBaseURL()
	want := "https://tunnel.example.com"
	if got != want {
		t.Errorf("expected %s, got %s", want, got)
	}
}

func TestGetPublicBaseURL_PublicURLTrailingSlash(t *testing.T) {
	cfg := &Config{Domain: "tunnel.example.com", HTTPPort: 80, PublicURL: "https://tunnel.example.com/"}
	got := cfg.GetPublicBaseURL()
	want := "https://tunnel.example.com"
	if got != want {
		t.Errorf("expected %s, got %s", want, got)
	}
}

func TestGetRelayBaseURL_HTTPS(t *testing.T) {
	cfg := &Config{Domain: "tunnel.example.com", HTTPPort: 443, TLSCert: "/cert.pem", TLSKey: "/key.pem"}
	got := cfg.GetRelayBaseURL()
	want := "wss://tunnel.example.com"
	if got != want {
		t.Errorf("expected %s, got %s", want, got)
	}
}

func TestGetRelayBaseURL_HTTP(t *testing.T) {
	cfg := &Config{Domain: "tunnel.example.com", HTTPPort: 8080}
	got := cfg.GetRelayBaseURL()
	want := "ws://tunnel.example.com:8080"
	if got != want {
		t.Errorf("expected %s, got %s", want, got)
	}
}

func TestGetRelayBaseURL_PublicURLOverride(t *testing.T) {
	cfg := &Config{Domain: "tunnel.example.com", HTTPPort: 8080, PublicURL: "https://tunnel.example.com"}
	got := cfg.GetRelayBaseURL()
	want := "wss://tunnel.example.com"
	if got != want {
		t.Errorf("expected %s, got %s", want, got)
	}
}
