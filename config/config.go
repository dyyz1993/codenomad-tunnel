package config

import (
	"flag"
	"os"
	"strconv"
)

type Config struct {
	Domain   string
	HTTPPort int
	APIPort  int
	TLSCert  string
	TLSKey   string
}

func Parse() *Config {
	cfg := &Config{}
	flag.StringVar(&cfg.Domain, "domain", "tunnel.example.com", "Base domain for tunnels")
	flag.IntVar(&cfg.HTTPPort, "http-port", 80, "HTTP listener port")
	flag.IntVar(&cfg.APIPort, "api-port", 8080, "Management API port")
	flag.StringVar(&cfg.TLSCert, "tls-cert", "", "TLS certificate path")
	flag.StringVar(&cfg.TLSKey, "tls-key", "", "TLS key path")
	flag.Parse()

	if env := os.Getenv("TUNNEL_DOMAIN"); env != "" {
		cfg.Domain = env
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

	return cfg
}
