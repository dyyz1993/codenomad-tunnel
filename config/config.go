package config

import (
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	Domain    string
	APIDomain string
	HTTPPort  int
	APIPort   int
	TLSCert   string
	TLSKey    string
	PublicURL string
}

func normalizeDomain(domain string) string {
	domain = strings.TrimSpace(domain)
	domain = strings.TrimPrefix(domain, "*.")
	domain = strings.TrimPrefix(domain, "*")
	return strings.TrimRight(domain, ".")
}

func Parse() *Config {
	cfg := &Config{}
	flag.StringVar(&cfg.Domain, "domain", "tunnel.example.com", "Base domain for tunnels")
	flag.StringVar(&cfg.APIDomain, "api-domain", "", "Domain for management API (e.g. api.tunnel.example.com). Auto-generated as api.{domain} if empty")
	flag.IntVar(&cfg.HTTPPort, "http-port", 80, "HTTP listener port")
	flag.IntVar(&cfg.APIPort, "api-port", 8080, "Management API port")
	flag.StringVar(&cfg.TLSCert, "tls-cert", "", "TLS certificate path")
	flag.StringVar(&cfg.TLSKey, "tls-key", "", "TLS key path")
	flag.StringVar(&cfg.PublicURL, "public-url", "", "Full public base URL (e.g. https://tunnel.example.com)")
	flag.Parse()

	if env := os.Getenv("TUNNEL_DOMAIN"); env != "" {
		cfg.Domain = env
	}
	if env := os.Getenv("API_DOMAIN"); env != "" {
		cfg.APIDomain = env
	}
	if env := os.Getenv("HTTP_PORT"); env != "" {
		if n, err := strconv.Atoi(env); err == nil && n > 0 {
			cfg.HTTPPort = n
		}
	}
	if env := os.Getenv("API_PORT"); env != "" {
		if n, err := strconv.Atoi(env); err == nil && n > 0 {
			cfg.APIPort = n
		}
	}
	if env := os.Getenv("TLS_CERT"); env != "" {
		cfg.TLSCert = env
	}
	if env := os.Getenv("TLS_KEY"); env != "" {
		cfg.TLSKey = env
	}
	if env := os.Getenv("TUNNEL_PUBLIC_URL"); env != "" {
		cfg.PublicURL = env
	}

	cfg.Domain = normalizeDomain(cfg.Domain)
	cfg.APIDomain = normalizeDomain(cfg.APIDomain)

	if cfg.APIDomain == "" {
		cfg.APIDomain = "api." + cfg.Domain
	}

	return cfg
}

func (c *Config) GetPublicBaseURL() string {
	if c.PublicURL != "" {
		return strings.TrimRight(c.PublicURL, "/")
	}
	scheme := "http"
	if c.TLSCert != "" {
		scheme = "https"
	}
	if (c.HTTPPort == 80 && scheme == "http") || (c.HTTPPort == 443 && scheme == "https") {
		return fmt.Sprintf("%s://%s", scheme, c.Domain)
	}
	return fmt.Sprintf("%s://%s:%d", scheme, c.Domain, c.HTTPPort)
}

func (c *Config) GetRelayBaseURL() string {
	base := c.GetPublicBaseURL()
	if strings.HasPrefix(base, "https://") {
		return "wss://" + strings.TrimPrefix(base, "https://")
	}
	return "ws://" + strings.TrimPrefix(base, "http://")
}
