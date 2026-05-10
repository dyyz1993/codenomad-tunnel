package client

import (
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"strings"
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

func errorResponse(id string, status int, msg string) *RelayResponse {
	return &RelayResponse{
		Type:    "response",
		ID:      id,
		Status:  status,
		Headers: map[string]string{"Content-Type": "text/plain"},
		Body:    fmt.Sprintf("tunnel-client: %s", msg),
	}
}
