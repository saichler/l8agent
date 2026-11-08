/*
(c) 2025 Sharon Aicler (saichler@gmail.com)

Layer 8 Ecosystem is licensed under the Apache License, Version 2.0.
*/

// Package lanfirst is an http.RoundTripper wrapper that reaches a service directly on the local network when
// it can, and uses the original (public / tunnel) address only when the direct connection cannot be made.
//
// A route maps a public host (e.g. "tools.test-llm.dev") to a LAN address ("192.168.1.124:8083"). A request
// for that host is first sent to the LAN address (same scheme and path, verified against the LAN CA). If
// that attempt fails before the request was written, the request is replayed unchanged on the public
// address. After a failure the LAN is skipped for a cool-down so later calls do not each pay the dial wait.
package lanfirst

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"net"
	"net/http"
	"net/http/httptrace"
	"strings"
	"sync"
	"time"
)

const (
	dialTimeout = 1500 * time.Millisecond
	// coolDown is how long the LAN is skipped after a failed direct attempt.
	coolDown = 30 * time.Second
)

// Transport tries the LAN route first and falls back to the wrapped transport.
type Transport struct {
	fallback http.RoundTripper
	lan      http.RoundTripper
	routes   map[string]string // lower-case public host -> LAN "host:port"
	now      func() time.Time

	mu        sync.Mutex
	skipUntil time.Time
}

// Wrap returns fallback unchanged when routes is empty. caPEM is the CA that signed the LAN servers'
// certificates (empty = system roots only).
func Wrap(fallback http.RoundTripper, routes map[string]string, caPEM string) (http.RoundTripper, error) {
	if len(routes) == 0 {
		return fallback, nil
	}
	if fallback == nil {
		fallback = http.DefaultTransport
	}
	pool, err := x509.SystemCertPool()
	if err != nil || pool == nil {
		pool = x509.NewCertPool()
	}
	if strings.TrimSpace(caPEM) != "" && !pool.AppendCertsFromPEM([]byte(caPEM)) {
		return nil, errors.New("lanfirst: LAN CA is not valid PEM")
	}
	t := &Transport{
		fallback: fallback,
		routes:   make(map[string]string, len(routes)),
		now:      time.Now,
		lan: &http.Transport{
			DialContext:         (&net.Dialer{Timeout: dialTimeout}).DialContext,
			TLSHandshakeTimeout: dialTimeout,
			TLSClientConfig:     &tls.Config{RootCAs: pool, MinVersion: tls.VersionTLS12},
			MaxIdleConns:        4,
			IdleConnTimeout:     30 * time.Second,
		},
	}
	for host, addr := range routes {
		t.routes[strings.ToLower(host)] = addr
	}
	return t, nil
}

// RoundTrip implements http.RoundTripper.
func (t *Transport) RoundTrip(req *http.Request) (*http.Response, error) {
	addr, ok := t.routes[strings.ToLower(req.URL.Hostname())]
	if !ok || req.URL.Scheme != "https" || t.skipping() {
		return t.fallback.RoundTrip(req)
	}
	if req.Body != nil && req.Body != http.NoBody && req.GetBody == nil {
		return t.fallback.RoundTrip(req) // body could not be replayed after a failed attempt
	}

	wrote := false
	trace := &httptrace.ClientTrace{WroteHeaders: func() { wrote = true }}
	lanReq := req.Clone(httptrace.WithClientTrace(req.Context(), trace))
	lanReq.URL.Host = addr
	lanReq.Host = addr
	resp, err := t.lan.RoundTrip(lanReq)
	if err == nil {
		return resp, nil
	}
	if wrote || req.Context().Err() != nil {
		return nil, err // the request reached the server (or the caller gave up): do not replay it
	}
	t.markDown()
	if req.GetBody != nil {
		body, berr := req.GetBody()
		if berr != nil {
			return nil, berr
		}
		req = req.Clone(req.Context())
		req.Body = body
	}
	return t.fallback.RoundTrip(req)
}

func (t *Transport) skipping() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.now().Before(t.skipUntil)
}

func (t *Transport) markDown() {
	t.mu.Lock()
	t.skipUntil = t.now().Add(coolDown)
	t.mu.Unlock()
}
