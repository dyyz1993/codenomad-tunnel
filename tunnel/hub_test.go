package tunnel

import (
	"strings"
	"testing"
)

func TestHubCreateTunnel(t *testing.T) {
	hub := NewHub("https://tunnel.example.com", "wss://tunnel.example.com")

	tunnel := hub.Create("test-crawler", "localhost", 8080)

	if tunnel.ID == "" {
		t.Error("expected non-empty tunnel ID")
	}
	if tunnel.Subdomain == "" {
		t.Error("expected non-empty subdomain")
	}
	if tunnel.TargetHost != "localhost" {
		t.Errorf("expected targetHost localhost, got %s", tunnel.TargetHost)
	}
	if tunnel.TargetPort != 8080 {
		t.Errorf("expected targetPort 8080, got %d", tunnel.TargetPort)
	}
	if tunnel.Status != StatusWaiting {
		t.Errorf("expected status waiting, got %s", tunnel.Status)
	}
	if tunnel.PublicURL == "" {
		t.Error("expected non-empty publicUrl")
	}
	if tunnel.RelayURL == "" {
		t.Error("expected non-empty relayUrl")
	}
	if !strings.Contains(tunnel.PublicURL, "https://tunnel.example.com/") {
		t.Errorf("expected publicUrl to contain base URL, got %s", tunnel.PublicURL)
	}
	if !strings.Contains(tunnel.RelayURL, "wss://tunnel.example.com/relay/") {
		t.Errorf("expected relayUrl to contain relay base URL, got %s", tunnel.RelayURL)
	}
}

func TestHubCreateUniqueSubdomains(t *testing.T) {
	hub := NewHub("https://tunnel.example.com", "wss://tunnel.example.com")

	t1 := hub.Create("a", "localhost", 8080)
	t2 := hub.Create("b", "localhost", 8081)

	if t1.Subdomain == t2.Subdomain {
		t.Error("expected unique subdomains")
	}
	if t1.ID == t2.ID {
		t.Error("expected unique IDs")
	}
}

func TestHubGetTunnel(t *testing.T) {
	hub := NewHub("https://tunnel.example.com", "wss://tunnel.example.com")

	created := hub.Create("test", "localhost", 3000)
	found, ok := hub.Get(created.ID)

	if !ok {
		t.Fatal("expected to find tunnel")
	}
	if found.ID != created.ID {
		t.Errorf("expected ID %s, got %s", created.ID, found.ID)
	}
}

func TestHubGetTunnelNotFound(t *testing.T) {
	hub := NewHub("https://tunnel.example.com", "wss://tunnel.example.com")

	_, ok := hub.Get("nonexistent")
	if ok {
		t.Error("expected false for nonexistent tunnel")
	}
}

func TestHubListTunnels(t *testing.T) {
	hub := NewHub("https://tunnel.example.com", "wss://tunnel.example.com")

	hub.Create("a", "localhost", 8080)
	hub.Create("b", "localhost", 8081)
	hub.Create("c", "localhost", 8082)

	list := hub.List()
	if len(list) != 3 {
		t.Errorf("expected 3 tunnels, got %d", len(list))
	}
}

func TestHubDeleteTunnel(t *testing.T) {
	hub := NewHub("https://tunnel.example.com", "wss://tunnel.example.com")

	created := hub.Create("test", "localhost", 8080)
	deleted := hub.Delete(created.ID)

	if !deleted {
		t.Error("expected delete to return true")
	}

	_, ok := hub.Get(created.ID)
	if ok {
		t.Error("expected tunnel to be deleted")
	}
}

func TestHubDeleteNonexistent(t *testing.T) {
	hub := NewHub("https://tunnel.example.com", "wss://tunnel.example.com")

	deleted := hub.Delete("nonexistent")
	if deleted {
		t.Error("expected delete to return false for nonexistent")
	}
}

func TestHubGetBySubdomain(t *testing.T) {
	hub := NewHub("https://tunnel.example.com", "wss://tunnel.example.com")

	created := hub.Create("test", "localhost", 8080)
	found, ok := hub.GetBySubdomain(created.Subdomain)

	if !ok {
		t.Fatal("expected to find tunnel by subdomain")
	}
	if found.ID != created.ID {
		t.Error("expected same tunnel")
	}
}

func TestHubGetBySubdomainNotFound(t *testing.T) {
	hub := NewHub("https://tunnel.example.com", "wss://tunnel.example.com")

	_, ok := hub.GetBySubdomain("nonexistent")
	if ok {
		t.Error("expected false for nonexistent subdomain")
	}
}
