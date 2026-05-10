package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/codenomad/tunnel-hub/client"
)

func main() {
	hubURL := flag.String("hub-url", "", "Tunnel hub API URL (e.g., https://api.tunnel.example.com:8443)")
	local := flag.String("local", "", "Local service URL (e.g., http://127.0.0.1:8888 or tcp://127.0.0.1:3306)")
	subdomain := flag.String("subdomain", "", "Requested subdomain (optional, auto-generated if empty)")
	name := flag.String("name", "", "Tunnel name (optional)")
	apiHost := flag.String("api-host", "", "API host header for hub routing (e.g., api.tunnel.example.com)")
	insecure := flag.Bool("insecure", false, "Skip TLS verification")
	flag.Parse()

	if *hubURL == "" || *local == "" {
		fmt.Fprintln(os.Stderr, "Usage: tunnel-client --hub-url <url> --local <url> [options]")
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "Options:")
		flag.PrintDefaults()
		os.Exit(1)
	}

	cfg := &client.Config{
		HubURL:    *hubURL,
		LocalURL:  *local,
		Subdomain: *subdomain,
		Name:      *name,
		APIHost:   *apiHost,
		Insecure:  *insecure,
	}

	c, err := client.New(cfg)
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigCh
		log.Println("Shutting down...")
		c.Shutdown()
	}()

	if err := c.Run(); err != nil {
		log.Fatalf("Client error: %v", err)
	}
}
