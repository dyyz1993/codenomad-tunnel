package tunnel

import (
	"testing"
)

func TestGenerateSubdomain(t *testing.T) {
	sub := GenerateSubdomain()

	if len(sub) != subdomLen {
		t.Errorf("expected subdomain length %d, got %d", subdomLen, len(sub))
	}
}

func TestGenerateSubdomainUniqueness(t *testing.T) {
	seen := make(map[string]bool)

	for i := 0; i < 100; i++ {
		sub := GenerateSubdomain()
		if seen[sub] {
			t.Errorf("duplicate subdomain generated: %s", sub)
		}
		seen[sub] = true
	}
}

func TestGenerateRequestID(t *testing.T) {
	id := GenerateRequestID()

	expectedLen := len("r_") + reqIDLen
	if len(id) != expectedLen {
		t.Errorf("expected request ID length %d, got %d", expectedLen, len(id))
	}
}

func TestGenerateTunnelID(t *testing.T) {
	id := GenerateTunnelID()

	expectedLen := len(tunnelPfx) + subdomLen
	if len(id) != expectedLen {
		t.Errorf("expected tunnel ID length %d, got %d", expectedLen, len(id))
	}
}

func TestNewTunnel(t *testing.T) {
	tunnel := NewTunnel("t_abc123", "xyz789", "https://*.tunnel.example.com", "wss://tunnel.example.com", "myapp", "localhost", 3000)

	if tunnel.ID != "t_abc123" {
		t.Errorf("expected ID t_abc123, got %s", tunnel.ID)
	}
	if tunnel.Subdomain != "xyz789" {
		t.Errorf("expected subdomain xyz789, got %s", tunnel.Subdomain)
	}
	if tunnel.PublicURL != "https://xyz789.tunnel.example.com" {
		t.Errorf("unexpected publicUrl: %s", tunnel.PublicURL)
	}
	if tunnel.RelayURL != "wss://tunnel.example.com/relay/t_abc123" {
		t.Errorf("unexpected relayUrl: %s", tunnel.RelayURL)
	}
	if tunnel.Status != StatusWaiting {
		t.Errorf("expected status waiting, got %s", tunnel.Status)
	}
}

func TestNewTunnelNoWildcard(t *testing.T) {
	tunnel := NewTunnel("t_abc123", "xyz789", "https://tunnel.example.com", "wss://tunnel.example.com", "myapp", "localhost", 3000)

	if tunnel.PublicURL != "https://tunnel.example.com/xyz789" {
		t.Errorf("unexpected publicUrl without wildcard: %s", tunnel.PublicURL)
	}
}

func TestValidateSubdomain(t *testing.T) {
	tests := []struct {
		input string
		valid bool
	}{
		{"my-app", true},
		{"app123", true},
		{"a", true},
		{"-app", false},
		{"app-", false},
		{"APP", false},
		{"my app", false},
		{"", false},
		{"a.b", false},
	}
	for _, tt := range tests {
		if got := ValidateSubdomain(tt.input); got != tt.valid {
			t.Errorf("ValidateSubdomain(%q) = %v, want %v", tt.input, got, tt.valid)
		}
	}
}
