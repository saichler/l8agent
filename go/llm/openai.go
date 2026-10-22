/*
© 2025 Sharon Aicler (saichler@gmail.com)

Layer 8 Ecosystem is licensed under the Apache License, Version 2.0.
*/

package llm

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"
)

const openAIRequestTimeout = 10 * time.Minute

// NewOpenAIClient creates a client for an OpenAI-compatible chat completions endpoint.
// baseURL is the API root, e.g. "https://host/v1". The Anthropic-shaped Message, Tool
// and Response types are translated to and from the OpenAI wire format.
func NewOpenAIClient(baseURL, apiKey, model string) *Client {
	return &Client{
		apiKey:    apiKey,
		apiURL:    strings.TrimRight(baseURL, "/") + "/chat/completions",
		model:     model,
		maxTokens: defaultMaxTokens,
		client:    &http.Client{Timeout: openAIRequestTimeout},
		openAI:    true,
	}
}

type oaiToolCall struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

type oaiMessage struct {
	Role       string        `json:"role"`
	Content    *string       `json:"content"`
	ToolCalls  []oaiToolCall `json:"tool_calls,omitempty"`
	ToolCallID string        `json:"tool_call_id,omitempty"`
}

type oaiFunction struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	Parameters  interface{} `json:"parameters"`
}

type oaiTool struct {
	Type     string      `json:"type"`
	Function oaiFunction `json:"function"`
}

type oaiRequest struct {
	Model     string       `json:"model"`
	MaxTokens int          `json:"max_tokens"`
	Messages  []oaiMessage `json:"messages"`
	Tools     []oaiTool    `json:"tools,omitempty"`
}

type oaiResponse struct {
	ID      string `json:"id"`
	Choices []struct {
		Message      oaiMessage `json:"message"`
		FinishReason string     `json:"finish_reason"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
	} `json:"usage"`
}

func strPtr(s string) *string { return &s }

func toOpenAIMessages(system string, messages []Message) []oaiMessage {
	out := make([]oaiMessage, 0, len(messages)+1)
	if system != "" {
		out = append(out, oaiMessage{Role: "system", Content: strPtr(system)})
	}
	for _, m := range messages {
		switch c := m.Content.(type) {
		case string:
			out = append(out, oaiMessage{Role: m.Role, Content: strPtr(c)})
		case []ContentBlock:
			msg := oaiMessage{Role: m.Role}
			var text string
			for _, b := range c {
				switch b.Type {
				case "text":
					text += b.Text
				case "tool_use":
					tc := oaiToolCall{ID: b.ID, Type: "function"}
					tc.Function.Name = b.Name
					tc.Function.Arguments = string(b.Input)
					if tc.Function.Arguments == "" {
						tc.Function.Arguments = "{}"
					}
					msg.ToolCalls = append(msg.ToolCalls, tc)
				}
			}
			if text != "" || len(msg.ToolCalls) == 0 {
				msg.Content = strPtr(text)
			}
			out = append(out, msg)
		case []ToolResultContent:
			for _, r := range c {
				out = append(out, oaiMessage{Role: "tool", ToolCallID: r.ToolUseID, Content: strPtr(r.Content)})
			}
		}
	}
	return out
}

func toOpenAITools(tools []Tool) []oaiTool {
	out := make([]oaiTool, 0, len(tools))
	for _, t := range tools {
		out = append(out, oaiTool{Type: "function", Function: oaiFunction{
			Name: t.Name, Description: t.Description, Parameters: t.InputSchema}})
	}
	return out
}

func (c *Client) sendOpenAI(systemPrompt string, messages []Message, tools []Tool) (*Response, error) {
	req := &oaiRequest{
		Model:     c.model,
		MaxTokens: c.maxTokens,
		Messages:  toOpenAIMessages(systemPrompt, messages),
		Tools:     toOpenAITools(tools),
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
	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)

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
		return nil, errors.New("LLM API error (" + httpResp.Status + "): " + string(respBody))
	}
	oai := &oaiResponse{}
	if err := json.Unmarshal(respBody, oai); err != nil {
		return nil, err
	}
	if len(oai.Choices) == 0 {
		return nil, errors.New("LLM API returned no choices")
	}
	return fromOpenAI(oai), nil
}

func fromOpenAI(oai *oaiResponse) *Response {
	choice := oai.Choices[0]
	resp := &Response{
		ID:   oai.ID,
		Type: "message",
		Role: "assistant",
		Usage: Usage{
			InputTokens:  oai.Usage.PromptTokens,
			OutputTokens: oai.Usage.CompletionTokens,
		},
	}
	if choice.Message.Content != nil && *choice.Message.Content != "" {
		resp.Content = append(resp.Content, ContentBlock{Type: "text", Text: *choice.Message.Content})
	}
	for _, tc := range choice.Message.ToolCalls {
		args := tc.Function.Arguments
		if args == "" {
			args = "{}"
		}
		resp.Content = append(resp.Content, ContentBlock{
			Type: "tool_use", ID: tc.ID, Name: tc.Function.Name, Input: json.RawMessage(args)})
	}
	switch {
	case len(choice.Message.ToolCalls) > 0:
		resp.StopReason = "tool_use"
	case choice.FinishReason == "length":
		resp.StopReason = "max_tokens"
	default:
		resp.StopReason = "end_turn"
	}
	return resp
}
