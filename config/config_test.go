package config

import (
	"os"
	"testing"
)

func TestParseDefaults(t *testing.T) {
	cfg := Parse()

	if cfg.Domain != "tunnel.example.com" {
		t.Errorf("expected default domain, got %s", cfg.Domain)
	}
	if cfg.HTTPPort != 80 {
		t.Errorf("expected default HTTP port 80, got %d", cfg.HTTPPort)
	}
	if cfg.APIPort != 8080 {
		t.Errorf("expected default API port 8080, got %d", cfg.APIPort)
	}
}

func TestEnvOverrides(t *testing.T) {
	os.Setenv("TUNNEL_DOMAIN", "test.example.com")
	os.Setenv("HTTP_PORT", "9090")
	defer os.Unsetenv("TUNNEL_DOMAIN")
	defer os.Unsetenv("HTTP_PORT")

	cfg := &Config{}
	if env := os.Getenv("TUNNEL_DOMAIN"); env != "" {
		cfg.Domain = env
	}
	if env := os.Getenv("HTTP_PORT"); env != "" {
		cfg.HTTPPort = 9090
	}

	if cfg.Domain != "test.example.com" {
		t.Errorf("expected domain from env, got %s", cfg.Domain)
	}
	if cfg.HTTPPort != 9090 {
		t.Errorf("expected port from env, got %d", cfg.HTTPPort)
	}
}
