package client

import (
	"encoding/base64"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"
)

const (
	tcpReadTimeout   = 5 * time.Second
	tcpIdleInterval  = 100 * time.Millisecond
	tcpMaxResponse   = 1 << 20
)

func ForwardHTTP(localBase string, httpClient *http.Client, req *RelayRequest) *RelayResponse {
	targetURL := localBase + req.Path
	if req.Query != "" {
		targetURL += "?" + req.Query
	}

	var body io.Reader
	if req.Body != "" {
		body = strings.NewReader(req.Body)
	}

	httpReq, err := http.NewRequest(req.Method, targetURL, body)
	if err != nil {
		return errorResponse(req.ID, http.StatusBadGateway, "failed to create request: "+err.Error())
	}

	for k, v := range req.Headers {
		httpReq.Header.Set(k, v)
	}

	resp, err := httpClient.Do(httpReq)
	if err != nil {
		return errorResponse(req.ID, http.StatusBadGateway, err.Error())
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return errorResponse(req.ID, http.StatusBadGateway, "failed to read response body")
	}

	headers := make(map[string]string)
	for k, vals := range resp.Header {
		if len(vals) > 0 {
			headers[k] = vals[0]
		}
	}

	relayResp := &RelayResponse{
		Type:    "response",
		ID:      req.ID,
		Status:  resp.StatusCode,
		Headers: headers,
	}

	if isBinaryContent(resp) {
		relayResp.BodyBase64 = base64.StdEncoding.EncodeToString(respBody)
	} else {
		relayResp.Body = string(respBody)
	}

	return relayResp
}

func isBinaryContent(resp *http.Response) bool {
	ct := resp.Header.Get("Content-Type")
	binaryPrefixes := []string{
		"image/",
		"video/",
		"audio/",
		"application/octet-stream",
		"application/pdf",
		"application/zip",
		"application/gzip",
		"application/x-tar",
	}
	for _, prefix := range binaryPrefixes {
		if strings.HasPrefix(ct, prefix) {
			return true
		}
	}
	return false
}

func ForwardTCP(host string, port int, req *RelayRequest) *RelayResponse {
	addr := fmt.Sprintf("%s:%d", host, port)

	conn, err := net.DialTimeout("tcp", addr, tcpReadTimeout)
	if err != nil {
		return errorResponse(req.ID, http.StatusBadGateway, "tcp connect: "+err.Error())
	}
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(tcpReadTimeout))

	if req.Body != "" || req.BodyBase64 != "" {
		var sendBytes []byte
		if req.BodyBase64 != "" {
			sendBytes, err = base64.StdEncoding.DecodeString(req.BodyBase64)
			if err != nil {
				return errorResponse(req.ID, http.StatusBadRequest, "base64 decode: "+err.Error())
			}
		} else {
			sendBytes = []byte(req.Body)
		}

		if _, err := conn.Write(sendBytes); err != nil {
			return errorResponse(req.ID, http.StatusBadGateway, "tcp write: "+err.Error())
		}
	}

	respBytes, err := readTCPResponse(conn)
	if err != nil {
		return errorResponse(req.ID, http.StatusBadGateway, "tcp read: "+err.Error())
	}

	headers := map[string]string{
		"Content-Type": "application/octet-stream",
	}

	relayResp := &RelayResponse{
		Type:       "response",
		ID:         req.ID,
		Status:     http.StatusOK,
		Headers:    headers,
		BodyBase64: base64.StdEncoding.EncodeToString(respBytes),
	}

	return relayResp
}

func readTCPResponse(conn net.Conn) ([]byte, error) {
	buf := make([]byte, 4096)
	var result []byte

	for {
		n, err := conn.Read(buf)
		if n > 0 {
			result = append(result, buf[:n]...)
			if len(result) >= tcpMaxResponse {
				break
			}
		}
		if err != nil {
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() && len(result) > 0 {
				break
			}
			if len(result) > 0 {
				break
			}
			return nil, err
		}
		if n == 0 {
			break
		}

		conn.SetReadDeadline(time.Now().Add(tcpIdleInterval))
	}

	return result, nil
}

func errorResponse(id string, status int, msg string) *RelayResponse {
	return &RelayResponse{
		Type:    "response",
		ID:      id,
		Status:  status,
		Headers: map[string]string{"Content-Type": "text/plain"},
		Body:    fmt.Sprintf("tunnel-client: %s", msg),
	}
}
