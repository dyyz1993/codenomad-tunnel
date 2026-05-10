package main

import (
	"fmt"
	"log"
	"net/http"

	"github.com/codenomad/tunnel-hub/api"
	"github.com/codenomad/tunnel-hub/config"
	"github.com/codenomad/tunnel-hub/tunnel"
)

func main() {
	cfg := config.Parse()

	publicBaseURL := cfg.GetPublicBaseURL()
	relayBaseURL := cfg.GetRelayBaseURL()

	hub := tunnel.NewHub(publicBaseURL, relayBaseURL)
	handler := api.NewHandler(hub, cfg.Domain)

	apiMux := http.NewServeMux()
	handler.RegisterRoutes(apiMux)

	proxyMux := http.NewServeMux()
	proxyMux.HandleFunc("/relay/{id}", func(w http.ResponseWriter, r *http.Request) {
		tunnel.HandleRelay(hub, w, r)
	})
	proxyMux.Handle("/", tunnel.HandleProxy(hub, cfg.Domain, cfg.APIDomain, corsMiddleware(apiMux)))

	go func() {
		addr := fmt.Sprintf(":%d", cfg.APIPort)
		log.Printf("Management API listening on %s", addr)
		if err := http.ListenAndServe(addr, corsMiddleware(apiMux)); err != nil {
			log.Fatalf("API server error: %v", err)
		}
	}()

	addr := fmt.Sprintf(":%d", cfg.HTTPPort)
	log.Printf("Tunnel proxy listening on %s (domain: %s)", addr, cfg.Domain)

	if cfg.TLSCert != "" && cfg.TLSKey != "" {
		log.Fatal(http.ListenAndServeTLS(addr, cfg.TLSCert, cfg.TLSKey, proxyMux))
	} else {
		log.Fatal(http.ListenAndServe(addr, proxyMux))
	}
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		setCorsHeaders(w)
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func setCorsHeaders(w http.ResponseWriter) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, PATCH, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "*")
	w.Header().Set("Access-Control-Allow-Credentials", "true")
	w.Header().Set("Access-Control-Max-Age", "86400")
}
