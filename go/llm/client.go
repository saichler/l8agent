/*
© 2025 Sharon Aicler (saichler@gmail.com)

Layer 8 Ecosystem is licensed under the Apache License, Version 2.0.
*/

// Package llm provides the HTTP client for calling the Claude API.
package llm

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"time"
)

const (
	defaultAPIURL    = "https://api.anthropic.com/v1/messages"
	defaultModel     = "claude-sonnet-4-6"
	defaultMaxTokens = 4096
	requestTimeout   = 60 * time.Second
	apiVersion       = "2023-06-01"
)

// Client is the Claude API HTTP client.
type Client struct {
	apiKey    string
	apiURL    string
	model     string
	maxTokens int
	client    *http.Client
}

// Message represents a chat message sent to/from the LLM.
type Message struct {
	Role    string      `json:"role"`
	Content interface{} `json:"content"`
}

// Tool represents an LLM tool definition.
type Tool struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	InputSchema interface{} `json:"input_schema"`
}

// Request is the Claude API request body.
type Request struct {
	Model     string    `json:"model"`
	MaxTokens int       `json:"max_tokens"`
	System    string    `json:"system,omitempty"`
	Messages  []Message `json:"messages"`
	Tools     []Tool    `json:"tools,omitempty"`
}

// Response is the Claude API response body.
type Response struct {
	ID           string         `json:"id"`
	Type         string         `json:"type"`
	Role         string         `json:"role"`
	Content      []ContentBlock `json:"content"`
	StopReason   string         `json:"stop_reason"`
	Usage        Usage          `json:"usage"`
}

// ContentBlock represents a content block in the response.
type ContentBlock struct {
	Type  string `json:"type"`
	Text  string `json:"text,omitempty"`
	ID    string `json:"id,omitempty"`
	Name  string `json:"name,omitempty"`
	Input json.RawMessage `json:"input,omitempty"`
}

// Usage tracks token usage in the response.
type Usage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

// ToolResultContent is used to send tool results back to the LLM.
type ToolResultContent struct {
	Type      string `json:"type"`
	ToolUseID string `json:"tool_use_id"`
	Content   string `json:"content"`
	IsError   bool   `json:"is_error,omitempty"`
}

// NewClient creates a new LLM client with the provided API key.
func NewClient(apiKey string) *Client {
	return &Client{
		apiKey:    apiKey,
		apiURL:    defaultAPIURL,
		model:     defaultModel,
		maxTokens: defaultMaxTokens,
		client:    &http.Client{Timeout: requestTimeout},
	}
}

// SendMessage sends a message to the Claude API and returns the response.
func (c *Client) SendMessage(systemPrompt string, messages []Message, tools []Tool) (*Response, error) {
	req := &Request{
		Model:     c.model,
		MaxTokens: c.maxTokens,
		System:    systemPrompt,
		Messages:  messages,
		Tools:     tools,
	}

	body, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequest("POST", c.apiURL, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", c.apiKey)
	httpReq.Header.Set("anthropic-version", apiVersion)

	httpResp, err := c.client.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer httpResp.Body.Close()

	respBody, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return nil, err
	}

	if httpResp.StatusCode != http.StatusOK {
		return nil, errors.New("Claude API error (" + httpResp.Status + "): " + string(respBody))
	}

	resp := &Response{}
	if err := json.Unmarshal(respBody, resp); err != nil {
		return nil, err
	}
	return resp, nil
}
