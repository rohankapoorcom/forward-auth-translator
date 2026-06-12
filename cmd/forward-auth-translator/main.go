package main

import (
	"errors"
	"log"
	"net/http"
	"net/url"
	"os"

	"github.com/rohankapoorcom/forward-auth-translator/internal/proxy"
)

func main() {
	listenAddr := envOrDefault("LISTEN_ADDR", ":8080")
	upstreamRaw := os.Getenv("AUTHENTIK_OUTPOST_URL")
	if upstreamRaw == "" {
		log.Fatal("AUTHENTIK_OUTPOST_URL is required")
	}

	upstreamURL, err := parseUpstreamURL(upstreamRaw)
	if err != nil {
		log.Fatalf("invalid AUTHENTIK_OUTPOST_URL: %v", err)
	}

	handler, err := proxy.NewHandler(proxy.Config{
		UpstreamURL:       upstreamURL,
		ProbeIDHeader:     envOrDefault("PROBE_ID_HEADER", "Gatus-Probe-Client-Id"),
		ProbeSecretHeader: envOrDefault("PROBE_SECRET_HEADER", "Gatus-Probe-Client-Secret"),
	})
	if err != nil {
		log.Fatalf("handler: %v", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/auth/traefik", handler.ServeAuth)
	mux.HandleFunc("/healthz", proxy.ServeHealthz)

	log.Printf("listening on %s; upstream %s", listenAddr, upstreamURL.String())
	if err := http.ListenAndServe(listenAddr, mux); err != nil {
		log.Fatal(err)
	}
}

func envOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func parseUpstreamURL(raw string) (*url.URL, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return nil, err
	}
	if u.Scheme == "" || u.Host == "" {
		return nil, errors.New("URL must include scheme and host")
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return nil, errors.New("URL scheme must be http or https")
	}
	return u, nil
}
