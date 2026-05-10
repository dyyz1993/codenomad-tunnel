package tunnel

import (
	"crypto/rand"
	"math/big"
	"regexp"
	"strings"
	"sync"
	"time"
)

const (
	charset     = "abcdefghijklmnopqrstuvwxyz0123456789"
	subdomLen   = 6
	reqIDLen    = 8
	tunnelPfx   = "t_"
)

type Status string

const (
	StatusWaiting    Status = "waiting"
	StatusConnected  Status = "connected"
	StatusDisconnected Status = "disconnected"
)

type Tunnel struct {
	mu            sync.RWMutex
	ID            string    `json:"id"`
	Subdomain     string    `json:"subdomain"`
	PublicURL     string    `json:"publicUrl"`
	RelayURL      string    `json:"relayUrl"`
	Name          string    `json:"name"`
	TargetHost    string    `json:"targetHost,omitempty"`
	TargetPort    int       `json:"targetPort,omitempty"`
	Status        Status    `json:"status"`
	CreatedAt     time.Time `json:"createdAt"`
	RequestCount  int64     `json:"requestCount"`
	LastRequestAt *time.Time `json:"lastRequestAt,omitempty"`

	relay     RelayConn
	pending   map[string]chan *RelayResponse
	pendingMu sync.Mutex
	cleanupCh chan struct{}
}

type RelayConn interface {
	SendJSON(v interface{}) error
	Close() error
}

type RelayRequest struct {
	Type       string            `json:"type"`
	ID         string            `json:"id"`
	Method     string            `json:"method"`
	Path       string            `json:"path"`
	Query      string            `json:"query"`
	Headers    map[string]string `json:"headers"`
	Body       string            `json:"body"`
	BodyBase64 string            `json:"bodyBase64,omitempty"`
}

type RelayResponse struct {
	Type      string            `json:"type"`
	ID        string            `json:"id"`
	Status    int               `json:"status"`
	Headers   map[string]string `json:"headers"`
	Body      string            `json:"body,omitempty"`
	BodyBase64 string           `json:"bodyBase64,omitempty"`
}

func GenerateSubdomain() string {
	return randomString(subdomLen)
}

func GenerateRequestID() string {
	return "r_" + randomString(reqIDLen)
}

func GenerateTunnelID() string {
	return tunnelPfx + randomString(subdomLen)
}

func randomString(length int) string {
	b := make([]byte, length)
	for i := range b {
		n, _ := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
		b[i] = charset[n.Int64()]
	}
	return string(b)
}

var subdomainRe = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]*[a-z0-9])?$`)

func ValidateSubdomain(sub string) bool {
	return len(sub) >= 1 && len(sub) <= 63 && subdomainRe.MatchString(sub)
}

func NewTunnel(id, subdomain, publicBaseURL, relayBaseURL, name, targetHost string, targetPort int) *Tunnel {
	var publicURL string
	if idx := strings.Index(publicBaseURL, "*"); idx >= 0 {
		publicURL = publicBaseURL[:idx] + subdomain + publicBaseURL[idx+1:]
	} else {
		publicURL = publicBaseURL + "/" + subdomain
	}
	relayURL := relayBaseURL
	if idx := strings.Index(relayBaseURL, "*"); idx >= 0 {
		relayURL = relayBaseURL[:idx] + "api" + relayBaseURL[idx+1:]
	}

	return &Tunnel{
		ID:         id,
		Subdomain:  subdomain,
		PublicURL:  publicURL,
		RelayURL:   relayURL + "/relay/" + id,
		Name:       name,
		TargetHost: targetHost,
		TargetPort: targetPort,
		Status:     StatusWaiting,
		CreatedAt:  time.Now().UTC(),
		pending:    make(map[string]chan *RelayResponse),
		cleanupCh:  make(chan struct{}),
	}
}

func (t *Tunnel) SetRelay(conn RelayConn) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.relay = conn
	t.Status = StatusConnected
}

func (t *Tunnel) ClearRelay() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.relay = nil
	t.Status = StatusDisconnected
}

func (t *Tunnel) SendRequest(req *RelayRequest) error {
	t.mu.RLock()
	defer t.mu.RUnlock()
	if t.relay == nil {
		return ErrNoRelay
	}
	return t.relay.SendJSON(req)
}

func (t *Tunnel) RegisterPending(id string) chan *RelayResponse {
	ch := make(chan *RelayResponse, 1)
	t.pendingMu.Lock()
	t.pending[id] = ch
	t.pendingMu.Unlock()
	return ch
}

func (t *Tunnel) DeliverResponse(resp *RelayResponse) bool {
	t.pendingMu.Lock()
	ch, ok := t.pending[resp.ID]
	if ok {
		delete(t.pending, resp.ID)
	}
	t.pendingMu.Unlock()
	if ok {
		ch <- resp
		return true
	}
	return false
}

func (t *Tunnel) IncrementRequests() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.RequestCount++
	now := time.Now().UTC()
	t.LastRequestAt = &now
}

func (t *Tunnel) Stop() {
	t.mu.Lock()
	if t.relay != nil {
		t.relay.Close()
		t.relay = nil
	}
	t.mu.Unlock()
	t.pendingMu.Lock()
	for id, ch := range t.pending {
		close(ch)
		delete(t.pending, id)
	}
	t.pendingMu.Unlock()
	close(t.cleanupCh)
}

func (t *Tunnel) CleanupCh() <-chan struct{} {
	return t.cleanupCh
}
