package client

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gorilla/websocket"
)

type Config struct {
	HubURL    string
	LocalURL  string
	Subdomain string
	Name      string
	APIHost   string
	Insecure  bool
}

type RelayRequest struct {
	Type    string            `json:"type"`
	ID      string            `json:"id"`
	Method  string            `json:"method"`
	Path    string            `json:"path"`
	Query   string            `json:"query"`
	Headers map[string]string `json:"headers"`
	Body    string            `json:"body"`
}

type RelayResponse struct {
	Type       string            `json:"type"`
	ID         string            `json:"id"`
	Status     int               `json:"status"`
	Headers    map[string]string `json:"headers"`
	Body       string            `json:"body,omitempty"`
	BodyBase64 string            `json:"bodyBase64,omitempty"`
}

type tunnelInfo struct {
	ID        string `json:"id"`
	Subdomain string `json:"subdomain"`
	PublicURL string `json:"publicUrl"`
	RelayURL  string `json:"relayUrl"`
}

type Client struct {
	cfg         *Config
	localHost   string
	localPort   int
	httpBase    string
	tunnel      *tunnelInfo
	done        chan struct{}
	httpClient  *http.Client
	probeResult *ProbeResult
	writeMu     chan struct{}
}

func New(cfg *Config) (*Client, error) {
	host, port := parseHostPort(cfg.LocalURL)

	httpBase := fmt.Sprintf("http://%s:%d", host, port)
	if strings.HasPrefix(cfg.LocalURL, "http://") || strings.HasPrefix(cfg.LocalURL, "https://") {
		httpBase = strings.TrimRight(cfg.LocalURL, "/")
	}

	transport := &http.Transport{}
	if cfg.Insecure {
		transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	}

	return &Client{
		cfg:        cfg,
		localHost:  host,
		localPort:  port,
		httpBase:   httpBase,
		done:       make(chan struct{}),
		httpClient: &http.Client{Transport: transport, Timeout: 30 * time.Second},
		writeMu:    make(chan struct{}, 1),
	}, nil
}

func (c *Client) Shutdown() {
	select {
	case <-c.done:
		return
	default:
		close(c.done)
	}
}

func (c *Client) Run() error {
	if err := c.register(); err != nil {
		return fmt.Errorf("register: %w", err)
	}

	c.probe()

	fmt.Printf("Public URL:  %s\n", c.tunnel.PublicURL)
	fmt.Printf("Relay URL:   %s\n", c.tunnel.RelayURL)
	fmt.Printf("Protocol:    %s\n", c.probeResult.Type)

	return c.relayLoop()
}

func (c *Client) register() error {
	reqBody := map[string]interface{}{
		"subdomain":  c.cfg.Subdomain,
		"targetHost": c.localHost,
		"targetPort": c.localPort,
	}
	if c.cfg.Name != "" {
		reqBody["name"] = c.cfg.Name
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return err
	}

	apiURL := strings.TrimRight(c.cfg.HubURL, "/") + "/api/tunnels"
	httpReq, err := http.NewRequest(http.MethodPost, apiURL, strings.NewReader(string(body)))
	if err != nil {
		return err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if c.cfg.APIHost != "" {
		httpReq.Host = c.cfg.APIHost
	}

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("POST %s: %w", apiURL, err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("register failed (%d): %s", resp.StatusCode, string(respBody))
	}

	var ti tunnelInfo
	if err := json.Unmarshal(respBody, &ti); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}

	c.tunnel = &ti
	log.Printf("Tunnel registered: %s (id: %s)", ti.Subdomain, ti.ID)
	return nil
}

func (c *Client) probe() {
	result := ProbeLocal(c.cfg.LocalURL, c.httpClient)
	c.probeResult = result
	log.Printf("Detected local service: %s (%s)", result.Type, result.Detail)
}

func (c *Client) relayLoop() error {
	for {
		select {
		case <-c.done:
			return nil
		default:
		}

		if err := c.connectAndRelay(); err != nil {
			log.Printf("Relay error: %v", err)
		}

		select {
		case <-c.done:
			return nil
		case <-time.After(3 * time.Second):
			log.Println("Reconnecting...")
		}
	}
}

func (c *Client) connectAndRelay() error {
	dialer := websocket.DefaultDialer
	if c.cfg.Insecure {
		dialer = &websocket.Dialer{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		}
	}

	conn, _, err := dialer.Dial(c.tunnel.RelayURL, nil)
	if err != nil {
		return fmt.Errorf("dial: %w", err)
	}

	stop := make(chan struct{})
	defer func() {
		close(stop)
		conn.Close()
	}()

	go func() {
		select {
		case <-c.done:
			conn.Close()
		case <-stop:
		}
	}()

	log.Println("Connected to relay")

	conn.SetPongHandler(func(string) error {
		conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	pingDone := make(chan struct{})
	go c.pingLoop(conn, pingDone)
	defer close(pingDone)

	conn.SetReadDeadline(time.Now().Add(60 * time.Second))

	for {
		_, msg, err := conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseNormalClosure) {
				return fmt.Errorf("read: %w", err)
			}
			return nil
		}

		var req RelayRequest
		if err := json.Unmarshal(msg, &req); err != nil {
			log.Printf("Invalid message: %v", err)
			continue
		}

		if req.Type == "request" {
			go c.handleRequest(conn, &req)
		}
	}
}

func (c *Client) handleRequest(conn *websocket.Conn, req *RelayRequest) {
	var resp *RelayResponse
	if c.probeResult.Type == ServiceTypeTCP {
		log.Printf("[TCP mode] Falling back to HTTP for request %s %s", req.Method, req.Path)
		resp = ForwardHTTP(c.httpBase, c.httpClient, req)
	} else {
		resp = ForwardHTTP(c.httpBase, c.httpClient, req)
	}

	c.writeMu <- struct{}{}
	conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
	if err := conn.WriteJSON(resp); err != nil {
		log.Printf("Send response %s: %v", req.ID, err)
	}
	<-c.writeMu
}

func (c *Client) pingLoop(conn *websocket.Conn, done chan struct{}) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-done:
			return
		case <-c.done:
			return
		case <-ticker.C:
			c.writeMu <- struct{}{}
			conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			err := conn.WriteMessage(websocket.PingMessage, nil)
			<-c.writeMu
			if err != nil {
				return
			}
		}
	}
}

func parseHostPort(rawURL string) (string, int) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return "127.0.0.1", 80
	}

	host := u.Hostname()
	portStr := u.Port()

	var port int
	if portStr != "" {
		fmt.Sscanf(portStr, "%d", &port)
	} else {
		switch u.Scheme {
		case "https", "wss":
			port = 443
		default:
			port = 80
		}
	}

	return host, port
}
