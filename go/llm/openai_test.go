package llm

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestOpenAIRoundTrip(t *testing.T) {
	var got oaiRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" || r.Header.Get("Authorization") != "Bearer k" {
			t.Errorf("bad request %s auth=%q", r.URL.Path, r.Header.Get("Authorization"))
		}
		json.NewDecoder(r.Body).Decode(&got)
		w.Write([]byte(`{"id":"1","choices":[{"finish_reason":"tool_calls","message":{"role":"assistant","content":"","tool_calls":[{"id":"c1","type":"function","function":{"name":"get","arguments":"{\"a\":1}"}}]}}],"usage":{"prompt_tokens":3,"completion_tokens":4}}`))
	}))
	defer srv.Close()

	c := NewOpenAIClient(srv.URL+"/v1/", "k", "m")
	msgs := []Message{
		{Role: "user", Content: "hi"},
		{Role: "assistant", Content: []ContentBlock{{Type: "tool_use", ID: "p", Name: "x", Input: json.RawMessage(`{}`)}}},
		{Role: "user", Content: []ToolResultContent{{Type: "tool_result", ToolUseID: "p", Content: "ok"}}},
	}
	resp, err := c.SendMessage("sys", msgs, []Tool{{Name: "get", Description: "d", InputSchema: map[string]string{"type": "object"}}})
	if err != nil {
		t.Fatal(err)
	}
	if resp.StopReason != "tool_use" || len(resp.Content) != 1 || resp.Content[0].Name != "get" || string(resp.Content[0].Input) != `{"a":1}` {
		t.Fatalf("bad response %+v", resp)
	}
	if resp.Usage.InputTokens != 3 || resp.Usage.OutputTokens != 4 {
		t.Fatalf("bad usage %+v", resp.Usage)
	}
	roles := []string{"system", "user", "assistant", "tool"}
	if len(got.Messages) != 4 {
		t.Fatalf("messages %+v", got.Messages)
	}
	for i, r := range roles {
		if got.Messages[i].Role != r {
			t.Errorf("msg %d role %s want %s", i, got.Messages[i].Role, r)
		}
	}
	if got.Messages[3].ToolCallID != "p" || len(got.Messages[2].ToolCalls) != 1 {
		t.Errorf("tool linkage wrong: %+v", got.Messages)
	}
}
