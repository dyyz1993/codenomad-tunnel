package client

import (
	"fmt"
	"net"
	"net/http"
	"time"
)

type ServiceType string

const (
	ServiceTypeHTTP    ServiceType = "http"
	ServiceTypeTCP     ServiceType = "tcp"
	ServiceTypeUnknown ServiceType = "unknown"
)

type ProbeResult struct {
	Type   ServiceType
	Detail string
}

func ProbeLocal(localURL string, httpClient *http.Client) *ProbeResult {
	if result := probeHTTP(localURL, httpClient); result != nil {
		return result
	}

	if result := probeTCP(localURL); result != nil {
		return result
	}

	return &ProbeResult{
		Type:   ServiceTypeUnknown,
		Detail: "Could not determine service type, defaulting to HTTP",
	}
}

func probeHTTP(localURL string, httpClient *http.Client) *ProbeResult {
	probeClient := *httpClient
	probeClient.Timeout = 3 * time.Second

	resp, err := probeClient.Get(localURL)
	if err != nil {
		return nil
	}
	resp.Body.Close()

	return &ProbeResult{
		Type:   ServiceTypeHTTP,
		Detail: fmt.Sprintf("HTTP %d", resp.StatusCode),
	}
}

func probeTCP(localURL string) *ProbeResult {
	host, port := parseHostPort(localURL)
	addr := fmt.Sprintf("%s:%d", host, port)

	conn, err := net.DialTimeout("tcp", addr, 3*time.Second)
	if err != nil {
		return nil
	}
	conn.Close()

	return &ProbeResult{
		Type:   ServiceTypeTCP,
		Detail: "TCP connection established",
	}
}
