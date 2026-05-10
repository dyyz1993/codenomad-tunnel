package tunnel

import (
	"encoding/base64"
	"io"
	"log"
	"net/http"
	"strings"
	"time"
)

func HandleProxy(hub *Hub, domain string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		host := r.Host
		subdomain := extractSubdomain(host, domain)
		if subdomain == "" {
			http.Error(w, "invalid host", http.StatusBadRequest)
			return
		}

		t, ok := hub.GetBySubdomain(subdomain)
		if !ok {
			http.Error(w, "tunnel not found", http.StatusNotFound)
			return
		}

		t.mu.RLock()
		status := t.Status
		t.mu.RUnlock()

		if status != StatusConnected {
			http.Error(w, "tunnel not connected", http.StatusBadGateway)
			return
		}

		reqID := GenerateRequestID()

		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "failed to read body", http.StatusInternalServerError)
			return
		}
		r.Body.Close()

		headers := make(map[string]string)
		for k, vals := range r.Header {
			if len(vals) > 0 {
				headers[k] = vals[0]
			}
		}

		relayReq := &RelayRequest{
			Type:    "request",
			ID:      reqID,
			Method:  r.Method,
			Path:    r.URL.Path,
			Query:   r.URL.RawQuery,
			Headers: headers,
			Body:    string(body),
		}

		ch := t.RegisterPending(reqID)
		t.IncrementRequests()

		if err := t.SendRequest(relayReq); err != nil {
			t.pendingMu.Lock()
			delete(t.pending, reqID)
			t.pendingMu.Unlock()
			http.Error(w, "relay send failed", http.StatusBadGateway)
			return
		}

		select {
		case resp, ok := <-ch:
			if !ok {
				http.Error(w, "tunnel closed", http.StatusBadGateway)
				return
			}
			writeResponse(w, resp)
		case <-time.After(requestTimeout):
			t.pendingMu.Lock()
			delete(t.pending, reqID)
			t.pendingMu.Unlock()
			http.Error(w, "gateway timeout", http.StatusGatewayTimeout)
		}
	}
}

func writeResponse(w http.ResponseWriter, resp *RelayResponse) {
	for k, v := range resp.Headers {
		w.Header().Set(k, v)
	}
	w.WriteHeader(resp.Status)
	if resp.BodyBase64 != "" {
		data, err := base64.StdEncoding.DecodeString(resp.BodyBase64)
		if err != nil {
			log.Printf("base64 decode error: %v", err)
			return
		}
		w.Write(data)
	} else {
		w.Write([]byte(resp.Body))
	}
}

func extractSubdomain(host, domain string) string {
	h := host
	if idx := strings.LastIndex(h, ":"); idx != -1 {
		h = h[:idx]
	}
	suffix := "." + domain
	if !strings.HasSuffix(h, suffix) {
		return ""
	}
	sub := strings.TrimSuffix(h, suffix)
	if sub == "" || strings.Contains(sub, ".") {
		return ""
	}
	return sub
}
