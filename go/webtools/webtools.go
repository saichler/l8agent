/*
(c) 2025 Sharon Aicler (saichler@gmail.com)

Layer 8 Ecosystem is licensed under the Apache License, Version 2.0.
*/

// Package webtools gives the agent web search and page fetch. It calls an HTTP "web tools" service
// (POST /v1/search and POST /v1/fetch, bearer-key authenticated) and exposes the two calls as LLM tools.
package webtools

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/saichler/l8agent/go/llm"
)

const (
	// ToolSearch and ToolFetch are the names of the tools offered to the model.
	ToolSearch = "web_search"
	ToolFetch  = "fetch_url"

	maxResultChars = 12000
	requestTimeout = 45 * time.Second
	// dialTimeout is short and attempts are retried: a connection that does not open quickly is
	// usually lost at the network edge, and a new attempt succeeds.
	dialTimeout = 6 * time.Second
	attempts    = 4
)

// Client calls the web tools service.
type Client struct {
	baseURL string
	apiKey  string
	http    *http.Client
}

// New returns a client for the service at baseURL (e.g. https://tools.example.com), or nil if baseURL is empty.
func New(baseURL, apiKey string, wrap ...func(http.RoundTripper) http.RoundTripper) *Client {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if baseURL == "" {
		return nil
	}
	transport := &http.Transport{
		Proxy:               http.ProxyFromEnvironment,
		DialContext:         (&net.Dialer{Timeout: dialTimeout}).DialContext,
		TLSHandshakeTimeout: dialTimeout,
		MaxIdleConns:        4,
		IdleConnTimeout:     30 * time.Second,
	}
	var rt http.RoundTripper = transport
	for _, w := range wrap {
		rt = w(rt)
	}
	return &Client{baseURL: baseURL, apiKey: apiKey, http: &http.Client{Timeout: requestTimeout, Transport: rt}}
}

// Definitions returns the tool definitions to pass to the model.
func Definitions() []llm.Tool {
	return []llm.Tool{
		{
			Name:        ToolSearch,
			Description: "Search the web. Returns titles, URLs and snippets. Use it for current or real-world information.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"query":       map[string]interface{}{"type": "string", "description": "The search query"},
					"max_results": map[string]interface{}{"type": "integer", "description": "Number of results (1-10)"},
				},
				"required": []string{"query"},
			},
		},
		{
			Name:        ToolFetch,
			Description: "Fetch a web page and return its text. Use it after web_search to read a result.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"url":       map[string]interface{}{"type": "string", "description": "The http(s) URL to read"},
					"max_chars": map[string]interface{}{"type": "integer", "description": "Maximum characters to return"},
				},
				"required": []string{"url"},
			},
		},
	}
}

// Execute runs one tool call; input is the model's JSON arguments. The result is plain text for the model.
func (c *Client) Execute(name, input string) (string, error) {
	var path string
	var args map[string]interface{}
	if err := json.Unmarshal([]byte(input), &args); err != nil {
		return "", fmt.Errorf("invalid tool arguments: %w", err)
	}
	switch name {
	case ToolSearch:
		path = "/v1/search"
		if _, ok := args["max_results"]; !ok {
			args["max_results"] = 5
		}
	case ToolFetch:
		path = "/v1/fetch"
		if _, ok := args["max_chars"]; !ok {
			args["max_chars"] = 6000
		}
	default:
		return "", fmt.Errorf("unknown web tool %q", name)
	}
	body, _ := json.Marshal(args)
	resp, err := c.post(path, body)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", err
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("web tools returned %d: %s", resp.StatusCode, strings.TrimSpace(string(data)))
	}
	out := string(data)
	if len(out) > maxResultChars {
		out = out[:maxResultChars] + "...[truncated]"
	}
	return out, nil
}

// post sends the request, retrying when the connection cannot be opened.
func (c *Client) post(path string, body []byte) (*http.Response, error) {
	var lastErr error
	for i := 0; i < attempts; i++ {
		req, err := http.NewRequest(http.MethodPost, c.baseURL+path, bytes.NewReader(body))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
		req.Header.Set("Content-Type", "application/json")
		resp, err := c.http.Do(req)
		if err == nil {
			return resp, nil
		}
		lastErr = err
		var nerr net.Error
		if !errors.As(err, &nerr) || !nerr.Timeout() {
			if _, isOp := err.(*url.Error); !isOp {
				return nil, err
			}
		}
	}
	return nil, lastErr
}
