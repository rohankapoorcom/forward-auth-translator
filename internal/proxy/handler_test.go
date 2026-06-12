package proxy_test

import (
	"encoding/base64"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/rohankapoorcom/forward-auth-translator/internal/proxy"
)

func newTestHandler(t *testing.T, upstream *httptest.Server) *proxy.Handler {
	t.Helper()
	u, err := url.Parse(upstream.URL)
	if err != nil {
		t.Fatal(err)
	}
	h, err := proxy.NewHandler(proxy.Config{UpstreamURL: u})
	if err != nil {
		t.Fatal(err)
	}
	return h
}

func TestBothProbeHeadersSetAuthorization(t *testing.T) {
	var gotAuth string
	var gotProbeID, gotProbeSecret string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotProbeID = r.Header.Get("Gatus-Probe-Client-Id")
		gotProbeSecret = r.Header.Get("Gatus-Probe-Client-Secret")
		w.WriteHeader(http.StatusOK)
	}))
	defer upstream.Close()

	handler := newTestHandler(t, upstream)
	translator := httptest.NewServer(http.HandlerFunc(handler.ServeAuth))
	defer translator.Close()

	req, _ := http.NewRequest(http.MethodGet, translator.URL+"/auth/traefik", nil)
	req.Header.Set("Gatus-Probe-Client-Id", "monitoring-probe")
	req.Header.Set("Gatus-Probe-Client-Secret", "example-app-password")
	req.Header.Set("X-Forwarded-Uri", "https://app.example.internal/")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	want := "Basic " + base64.StdEncoding.EncodeToString([]byte("monitoring-probe:example-app-password"))
	if gotAuth != want {
		t.Fatalf("Authorization = %q, want %q", gotAuth, want)
	}
	if gotProbeID != "" || gotProbeSecret != "" {
		t.Fatalf("probe headers forwarded: id=%q secret=%q", gotProbeID, gotProbeSecret)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
}

func TestOnlyProbeIDNoAuthorization(t *testing.T) {
	var gotAuth string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
	}))
	defer upstream.Close()

	handler := newTestHandler(t, upstream)
	translator := httptest.NewServer(http.HandlerFunc(handler.ServeAuth))
	defer translator.Close()

	req, _ := http.NewRequest(http.MethodGet, translator.URL+"/auth/traefik", nil)
	req.Header.Set("Gatus-Probe-Client-Id", "monitoring-probe")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if gotAuth != "" {
		t.Fatalf("Authorization = %q, want empty", gotAuth)
	}
}

func TestOnlyProbeSecretNoAuthorization(t *testing.T) {
	var gotAuth string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
	}))
	defer upstream.Close()

	handler := newTestHandler(t, upstream)
	translator := httptest.NewServer(http.HandlerFunc(handler.ServeAuth))
	defer translator.Close()

	req, _ := http.NewRequest(http.MethodGet, translator.URL+"/auth/traefik", nil)
	req.Header.Set("Gatus-Probe-Client-Secret", "example-app-password")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if gotAuth != "" {
		t.Fatalf("Authorization = %q, want empty", gotAuth)
	}
}

func TestBrowserPathForwardsCookie(t *testing.T) {
	var gotCookie, gotAuth, gotForwarded string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotCookie = r.Header.Get("Cookie")
		gotAuth = r.Header.Get("Authorization")
		gotForwarded = r.Header.Get("X-Forwarded-Uri")
		w.WriteHeader(http.StatusOK)
	}))
	defer upstream.Close()

	handler := newTestHandler(t, upstream)
	translator := httptest.NewServer(http.HandlerFunc(handler.ServeAuth))
	defer translator.Close()

	req, _ := http.NewRequest(http.MethodGet, translator.URL+"/auth/traefik", nil)
	req.Header.Set("Cookie", "ak_session=abc123")
	req.Header.Set("X-Forwarded-Uri", "https://app.example.internal/dashboard")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if gotCookie != "ak_session=abc123" {
		t.Fatalf("Cookie = %q", gotCookie)
	}
	if gotAuth != "" {
		t.Fatalf("Authorization = %q, want empty", gotAuth)
	}
	if gotForwarded != "https://app.example.internal/dashboard" {
		t.Fatalf("X-Forwarded-Uri = %q", gotForwarded)
	}
}

func TestUpstream401PassedThrough(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer upstream.Close()

	handler := newTestHandler(t, upstream)
	translator := httptest.NewServer(http.HandlerFunc(handler.ServeAuth))
	defer translator.Close()

	req, _ := http.NewRequest(http.MethodGet, translator.URL+"/auth/traefik", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", resp.StatusCode)
	}
}

func TestUpstream302PassedThrough(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "https://auth.example.com/login", http.StatusFound)
	}))
	defer upstream.Close()

	handler := newTestHandler(t, upstream)
	translator := httptest.NewServer(http.HandlerFunc(handler.ServeAuth))
	defer translator.Close()

	req, _ := http.NewRequest(http.MethodGet, translator.URL+"/auth/traefik", nil)
	client := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusFound {
		t.Fatalf("status = %d, want 302", resp.StatusCode)
	}
	if loc := resp.Header.Get("Location"); !strings.Contains(loc, "auth.example.com") {
		t.Fatalf("Location = %q", loc)
	}
}

func TestHealthzReturns200(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rr := httptest.NewRecorder()
	proxy.ServeHealthz(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	if body := rr.Body.String(); body != "ok" {
		t.Fatalf("body = %q, want ok", body)
	}
}

func TestNewHandlerRequiresUpstream(t *testing.T) {
	_, err := proxy.NewHandler(proxy.Config{})
	if err == nil {
		t.Fatal("expected error for nil upstream")
	}
}

func TestSSRFGuardUsesConfiguredHost(t *testing.T) {
	var gotHost string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotHost = r.Host
		w.WriteHeader(http.StatusOK)
	}))
	defer upstream.Close()

	handler := newTestHandler(t, upstream)
	translator := httptest.NewServer(http.HandlerFunc(handler.ServeAuth))
	defer translator.Close()

	req, _ := http.NewRequest(http.MethodGet, translator.URL+"/auth/traefik", nil)
	req.Host = "evil.example.com"
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	wantHost := handler.UpstreamHost()
	if gotHost != wantHost {
		t.Fatalf("upstream Host = %q, want configured %q", gotHost, wantHost)
	}
}

func TestServeAuthRejectsPost(t *testing.T) {
	u, _ := url.Parse("http://127.0.0.1:1")
	h, err := proxy.NewHandler(proxy.Config{UpstreamURL: u})
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/auth/traefik", nil)
	rr := httptest.NewRecorder()
	h.ServeAuth(rr, req)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want 405", rr.Code)
	}
}

func TestServeHealthzRejectsPost(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/healthz", nil)
	rr := httptest.NewRecorder()
	proxy.ServeHealthz(rr, req)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want 405", rr.Code)
	}
}

func TestHeadMethodSupported(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer upstream.Close()

	handler := newTestHandler(t, upstream)
	translator := httptest.NewServer(http.HandlerFunc(handler.ServeAuth))
	defer translator.Close()

	req, _ := http.NewRequest(http.MethodHead, translator.URL+"/auth/traefik", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
}

func TestNoLocal200SynthesisOnUpstreamError(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = io.WriteString(w, "denied")
	}))
	defer upstream.Close()

	handler := newTestHandler(t, upstream)
	translator := httptest.NewServer(http.HandlerFunc(handler.ServeAuth))
	defer translator.Close()

	req, _ := http.NewRequest(http.MethodGet, translator.URL+"/auth/traefik", nil)
	req.Header.Set("Gatus-Probe-Client-Id", "monitoring-probe")
	req.Header.Set("Gatus-Probe-Client-Secret", "wrong")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("status = %d, want 403 from upstream", resp.StatusCode)
	}
}
