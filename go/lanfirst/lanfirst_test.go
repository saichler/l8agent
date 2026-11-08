package lanfirst

import (
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// newTransport builds a Transport whose LAN client trusts srv's certificate.
func newTransport(t *testing.T, fallback http.RoundTripper, lanAddr string, lanSrv *httptest.Server) *Transport {
	t.Helper()
	rt, err := Wrap(fallback, map[string]string{"tools.example.test": lanAddr}, "")
	if err != nil {
		t.Fatal(err)
	}
	tr := rt.(*Transport)
	if lanSrv != nil {
		tr.lan = lanSrv.Client().Transport
	}
	return tr
}

func post(t *testing.T, rt http.RoundTripper, url, body string) (int, string) {
	t.Helper()
	req, _ := http.NewRequest(http.MethodPost, url, strings.NewReader(body))
	resp, err := rt.RoundTrip(req)
	if err != nil {
		t.Fatalf("round trip: %v", err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(b)
}

func TestEmptyRoutesReturnsFallback(t *testing.T) {
	fb := http.DefaultTransport
	rt, err := Wrap(fb, nil, "")
	if err != nil || rt != fb {
		t.Fatal("no routes must return the fallback unchanged")
	}
}

func TestLANUsedWhenReachable(t *testing.T) {
	lan := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { io.WriteString(w, "lan") }))
	defer lan.Close()
	pub := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { io.WriteString(w, "public") }))
	defer pub.Close()
	addr := strings.TrimPrefix(lan.URL, "https://")
	tr := newTransport(t, &redirectTo{pub.Listener.Addr().String()}, addr, lan)
	if _, got := post(t, tr, "https://tools.example.test/v1/search", "{}"); got != "lan" {
		t.Fatalf("expected LAN, got %q", got)
	}
}

func TestFallbackWhenLANDown(t *testing.T) {
	ln, _ := net.Listen("tcp", "127.0.0.1:0")
	dead := ln.Addr().String()
	ln.Close() // nothing listens: connection refused
	var hits int
	pub := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		b, _ := io.ReadAll(r.Body)
		io.WriteString(w, "public:"+string(b))
	}))
	defer pub.Close()
	tr := newTransport(t, &redirectTo{pub.Listener.Addr().String()}, dead, nil)
	if _, got := post(t, tr, "https://tools.example.test/v1/search", "payload"); got != "public:payload" {
		t.Fatalf("expected fallback with body intact, got %q", got)
	}
	// Cool-down: the next call goes straight to the fallback.
	start := time.Now()
	post(t, tr, "https://tools.example.test/v1/search", "again")
	if !tr.skipping() || time.Since(start) > time.Second || hits != 2 {
		t.Fatalf("expected cool-down skip, hits=%d", hits)
	}
}

func TestUnroutedHostGoesToFallback(t *testing.T) {
	pub := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { io.WriteString(w, "public") }))
	defer pub.Close()
	tr := newTransport(t, &redirectTo{pub.Listener.Addr().String()}, "127.0.0.1:1", nil)
	if _, got := post(t, tr, "https://other.example.test/x", ""); got != "public" {
		t.Fatalf("got %q", got)
	}
}

// redirectTo is a stand-in "public" transport: it sends every request to a plain-HTTP test server.
type redirectTo struct{ addr string }

func (r *redirectTo) RoundTrip(req *http.Request) (*http.Response, error) {
	c := req.Clone(req.Context())
	c.URL.Scheme, c.URL.Host = "http", r.addr
	return http.DefaultTransport.RoundTrip(c)
}
