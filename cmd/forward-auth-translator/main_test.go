package main

import (
	"strings"
	"testing"
)

func TestParseUpstreamURLValid(t *testing.T) {
	u, err := parseUpstreamURL("http://127.0.0.1:9000/outpost.goauthentik.io/auth/traefik")
	if err != nil {
		t.Fatal(err)
	}
	if u.Host != "127.0.0.1:9000" {
		t.Fatalf("host = %q", u.Host)
	}
}

func TestParseUpstreamURLRejectsMissingScheme(t *testing.T) {
	_, err := parseUpstreamURL("authentik-outpost/outpost.goauthentik.io/auth/traefik")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestParseUpstreamURLRejectsUnsupportedScheme(t *testing.T) {
	_, err := parseUpstreamURL("ftp://auth.example.com/outpost")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestParseUpstreamURLRejectsRootPath(t *testing.T) {
	_, err := parseUpstreamURL("http://auth.example.com")
	if err == nil {
		t.Fatal("expected error")
	}
	_, err = parseUpstreamURL("https://auth.example.com/")
	if err == nil {
		t.Fatal("expected error for root path")
	}
}

func TestRunRequiresUpstreamURL(t *testing.T) {
	t.Setenv("AUTHENTIK_OUTPOST_URL", "")
	err := run()
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "AUTHENTIK_OUTPOST_URL is required") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestEnvOrDefault(t *testing.T) {
	t.Setenv("TEST_FORWARD_AUTH_VAR", "custom")
	if got := envOrDefault("TEST_FORWARD_AUTH_VAR", "fallback"); got != "custom" {
		t.Fatalf("got %q", got)
	}
	if got := envOrDefault("TEST_FORWARD_AUTH_MISSING", "fallback"); got != "fallback" {
		t.Fatalf("got %q", got)
	}
}
