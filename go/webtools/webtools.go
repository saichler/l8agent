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
	"fmt"
	"io"
	"net/http"
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
)

// Client calls the web tools service.
type Client struct {
	baseURL string
	apiKey  string
	http    *http.Client
}

// New returns a client for the service at baseURL (e.g. https://tools.example.com), or nil if baseURL is empty.
func New(baseURL, apiKey string) *Client {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if baseURL == "" {
		return nil
	}
	return &Client{baseURL: baseURL, apiKey: apiKey, http: &http.Client{Timeout: requestTimeout}}
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
	req, err := http.NewRequest(http.MethodPost, c.baseURL+path, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
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
