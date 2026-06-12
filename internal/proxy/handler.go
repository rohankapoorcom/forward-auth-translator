package proxy

import (
	"encoding/base64"
	"io"
	"net/http"
	"net/http/httputil"
	"net/url"
)

// Config holds runtime settings for the forward-auth translator.
type Config struct {
	UpstreamURL       *url.URL
	ProbeIDHeader     string
	ProbeSecretHeader string
}

// Handler proxies Traefik forwardAuth subrequests to an Authentik outpost,
// translating probe headers into Authorization: Basic when both are present.
type Handler struct {
	cfg    Config
	proxy  *httputil.ReverseProxy
	client *http.Client
}

// NewHandler builds a Handler. upstreamURL must be a fixed outpost base URL.
func NewHandler(cfg Config) (*Handler, error) {
	if cfg.UpstreamURL == nil {
		return nil, errMissingUpstream
	}
	if cfg.ProbeIDHeader == "" {
		cfg.ProbeIDHeader = "Gatus-Probe-Client-Id"
	}
	if cfg.ProbeSecretHeader == "" {
		cfg.ProbeSecretHeader = "Gatus-Probe-Client-Secret"
	}

	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = nil

	h := &Handler{
		cfg: cfg,
		client: &http.Client{
			Transport: transport,
			CheckRedirect: func(*http.Request, []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
	}

	h.proxy = &httputil.ReverseProxy{
		Director:       h.director,
		Transport:      h.client.Transport,
		ModifyResponse: nil,
		ErrorHandler: func(w http.ResponseWriter, _ *http.Request, err error) {
			http.Error(w, "upstream unavailable", http.StatusBadGateway)
			_ = err
		},
	}

	return h, nil
}

var errMissingUpstream = &configError{msg: "AUTHENTIK_OUTPOST_URL is required"}

type configError struct {
	msg string
}

func (e *configError) Error() string {
	return e.msg
}

func (h *Handler) director(req *http.Request) {
	upstream := h.cfg.UpstreamURL
	req.URL.Scheme = upstream.Scheme
	req.URL.Host = upstream.Host
	req.URL.Path = upstream.Path
	req.URL.RawPath = upstream.RawPath
	req.Host = upstream.Host

	id := req.Header.Get(h.cfg.ProbeIDHeader)
	secret := req.Header.Get(h.cfg.ProbeSecretHeader)
	if id != "" && secret != "" {
		cred := base64.StdEncoding.EncodeToString([]byte(id + ":" + secret))
		req.Header.Set("Authorization", "Basic "+cred)
		req.Header.Del(h.cfg.ProbeIDHeader)
		req.Header.Del(h.cfg.ProbeSecretHeader)
	}
}

// ServeAuth handles GET /auth/traefik forwardAuth subrequests.
func (h *Handler) ServeAuth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	h.proxy.ServeHTTP(w, r)
}

// ServeHealthz handles GET /healthz.
func ServeHealthz(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	w.WriteHeader(http.StatusOK)
	_, _ = io.WriteString(w, "ok")
}

// UpstreamHost returns the configured upstream host for tests and guards.
func (h *Handler) UpstreamHost() string {
	return h.cfg.UpstreamURL.Host
}
