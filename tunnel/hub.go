package tunnel

import (
	"sync"
	"time"
)

const cleanupInterval = 5 * time.Minute

type Hub struct {
	mu       sync.RWMutex
	tunnels  map[string]*Tunnel
	bySubdom map[string]*Tunnel
	domain   string
}

func NewHub(domain string) *Hub {
	h := &Hub{
		tunnels:  make(map[string]*Tunnel),
		bySubdom: make(map[string]*Tunnel),
		domain:   domain,
	}
	go h.cleanupLoop()
	return h
}

func (h *Hub) Create(name, targetHost string, targetPort int) *Tunnel {
	h.mu.Lock()
	defer h.mu.Unlock()

	var subdomain string
	for i := 0; i < 10; i++ {
		subdomain = GenerateSubdomain()
		if _, exists := h.bySubdom[subdomain]; !exists {
			break
		}
		subdomain = ""
	}
	if subdomain == "" {
		subdomain = GenerateSubdomain()
	}

	id := GenerateTunnelID()
	t := NewTunnel(id, subdomain, h.domain, name, targetHost, targetPort)
	h.tunnels[id] = t
	h.bySubdom[subdomain] = t
	return t
}

func (h *Hub) Get(id string) (*Tunnel, bool) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	t, ok := h.tunnels[id]
	return t, ok
}

func (h *Hub) GetBySubdomain(sub string) (*Tunnel, bool) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	t, ok := h.bySubdom[sub]
	return t, ok
}

func (h *Hub) List() []*Tunnel {
	h.mu.RLock()
	defer h.mu.RUnlock()
	result := make([]*Tunnel, 0, len(h.tunnels))
	for _, t := range h.tunnels {
		result = append(result, t)
	}
	return result
}

func (h *Hub) Delete(id string) bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	t, ok := h.tunnels[id]
	if !ok {
		return false
	}
	delete(h.tunnels, id)
	delete(h.bySubdom, t.Subdomain)
	t.Stop()
	return true
}

func (h *Hub) cleanupLoop() {
	ticker := time.NewTicker(cleanupInterval)
	defer ticker.Stop()
	for range ticker.C {
		h.mu.Lock()
		now := time.Now()
		for id, t := range h.tunnels {
			t.mu.RLock()
			status := t.Status
			t.mu.RUnlock()
			if status == StatusDisconnected || status == StatusWaiting {
				if now.Sub(t.CreatedAt) > cleanupInterval {
					delete(h.tunnels, id)
					delete(h.bySubdom, t.Subdomain)
					t.Stop()
				}
			}
		}
		h.mu.Unlock()
	}
}
