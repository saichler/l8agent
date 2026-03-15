/*
(c) 2025 Sharon Aicler (saichler@gmail.com)

Layer 8 Ecosystem is licensed under the Apache License, Version 2.0.
*/

package chat

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/saichler/l8agent/go/llm"
	"github.com/saichler/l8agent/go/masking"
	"github.com/saichler/l8agent/go/services/conversations"
	"github.com/saichler/l8agent/go/tools"
	"github.com/saichler/l8agent/go/types/l8agent"
	"github.com/saichler/l8types/go/ifs"
)

const (
	defaultMaxToolCalls = 10
	defaultMaxHistory   = 20
	baseSystemPrompt    = `You are an AI assistant for a Layer 8 application. You help users query data, create records, and analyze information using the provided tools.

Rules:
- Only use the provided tools to query and modify data.
- Never execute DELETE operations without explicit user confirmation.
- Never fabricate data. If you don't know something, say so.
- When querying data, use L8Query syntax.
- Use describe_model to learn field names before constructing queries.
- Use list_modules to discover available services and models.`
)

// orchestrate runs the full chat flow: load/create conversation, call LLM, execute tools, unmask, save.
func orchestrate(h *chatHandler, req *l8agent.L8AgentChatRequest, vnic ifs.IVNic) (*l8agent.L8AgentChatResponse, error) {
	if h.llmClient == nil {
		return nil, fmt.Errorf("LLM client not configured. Set ANTHROPIC_API_KEY environment variable")
	}

	// Step 1: Load or create conversation
	convo, isNew, err := loadOrCreateConversation(req, vnic)
	if err != nil {
		return nil, fmt.Errorf("conversation error: %w", err)
	}

	// Step 2: Add user message to conversation
	userMsg := &l8agent.L8AgentMessage{
		MessageId: ifs.NewUuid(),
		Role:      int32(l8agent.L8AgentMessageRole_L8_AGENT_MESSAGE_ROLE_USER),
		Content:   req.Message,
		Timestamp: time.Now().Unix(),
	}
	convo.Messages = append(convo.Messages, userMsg)

	// Step 3: Create per-request token map for masking
	tokenMap := masking.NewTokenMap()

	// Step 4: Build system prompt
	systemPrompt := baseSystemPrompt + "\n\n" + h.schema.GetTier1Schema()

	// Step 5: Build LLM messages from conversation history
	messages := buildMessages(convo)

	// Step 6: Get tool definitions
	toolDefs := tools.GetToolDefinitions()

	// Step 7: Call LLM with tool loop
	response, dataResults, toolCallCount, totalTokens, err := toolLoop(h, systemPrompt, messages, toolDefs, tokenMap, vnic)
	if err != nil {
		return nil, fmt.Errorf("LLM error: %w", err)
	}

	// Step 8: Unmask the response
	response = tokenMap.Unmask(response)

	// Step 9: Add assistant message to conversation
	assistantMsg := &l8agent.L8AgentMessage{
		MessageId:  ifs.NewUuid(),
		Role:       int32(l8agent.L8AgentMessageRole_L8_AGENT_MESSAGE_ROLE_ASSISTANT),
		Content:    response,
		Timestamp:  time.Now().Unix(),
		TokenCount: int32(totalTokens),
	}
	convo.Messages = append(convo.Messages, assistantMsg)
	convo.UpdatedAt = time.Now().Unix()

	// Step 10: Save conversation
	if err := conversations.SaveConversation(convo, isNew, vnic); err != nil {
		fmt.Println("[agent] warning: failed to save conversation:", err)
	}

	return &l8agent.L8AgentChatResponse{
		ConversationId: convo.ConversationId,
		Response:       response,
		DataResults:    dataResults,
		ToolCallsMade:  int32(toolCallCount),
		TotalTokens:    int32(totalTokens),
	}, nil
}

func loadOrCreateConversation(req *l8agent.L8AgentChatRequest, vnic ifs.IVNic) (*l8agent.L8AgentConversation, bool, error) {
	if req.ConversationId != "" {
		convo, err := conversations.Conversation(req.ConversationId, vnic)
		if err != nil {
			return nil, false, err
		}
		if convo != nil {
			return convo, false, nil
		}
	}

	convo := &l8agent.L8AgentConversation{
		ConversationId: ifs.NewUuid(),
		UserId:         "system",
		Title:          truncateTitle(req.Message),
		Status:         int32(l8agent.L8AgentConvoStatus_L8_AGENT_CONVO_STATUS_ACTIVE),
		CreatedAt:      time.Now().Unix(),
		UpdatedAt:      time.Now().Unix(),
	}
	return convo, true, nil
}

func buildMessages(convo *l8agent.L8AgentConversation) []llm.Message {
	maxHistory := getMaxHistory()
	msgs := convo.Messages
	if len(msgs) > maxHistory {
		msgs = msgs[len(msgs)-maxHistory:]
	}

	result := make([]llm.Message, 0, len(msgs))
	for _, msg := range msgs {
		role := "user"
		if msg.Role == int32(l8agent.L8AgentMessageRole_L8_AGENT_MESSAGE_ROLE_ASSISTANT) {
			role = "assistant"
		}
		result = append(result, llm.Message{Role: role, Content: msg.Content})
	}
	return result
}

// toolLoop calls the LLM and executes tool calls until the LLM returns a text response.
func toolLoop(h *chatHandler, systemPrompt string, messages []llm.Message, toolDefs []llm.Tool, tokenMap *masking.TokenMap, vnic ifs.IVNic) (string, []*l8agent.L8AgentDataResult, int, int, error) {
	maxToolCalls := getMaxToolCalls()
	toolCallCount := 0
	totalTokens := 0
	var dataResults []*l8agent.L8AgentDataResult

	for i := 0; i < maxToolCalls; i++ {
		resp, err := h.llmClient.SendMessage(systemPrompt, messages, toolDefs)
		if err != nil {
			return "", nil, toolCallCount, totalTokens, err
		}

		totalTokens += resp.Usage.InputTokens + resp.Usage.OutputTokens

		if resp.StopReason == "end_turn" {
			text := extractText(resp)
			return text, dataResults, toolCallCount, totalTokens, nil
		}

		if resp.StopReason == "tool_use" {
			toolResults, results := executeToolCalls(h, resp, tokenMap, vnic)
			toolCallCount += len(toolResults)
			dataResults = append(dataResults, results...)

			assistantContent := marshalJSON(resp.Content)
			messages = append(messages, llm.Message{Role: "assistant", Content: assistantContent})

			toolResultContent := marshalJSON(toolResults)
			messages = append(messages, llm.Message{Role: "user", Content: toolResultContent})
			continue
		}

		text := extractText(resp)
		return text, dataResults, toolCallCount, totalTokens, nil
	}

	return "I've reached the maximum number of tool calls for this request. Please try a more specific question.", dataResults, toolCallCount, totalTokens, nil
}

func executeToolCalls(h *chatHandler, resp *llm.Response, tokenMap *masking.TokenMap, vnic ifs.IVNic) ([]llm.ToolResultContent, []*l8agent.L8AgentDataResult) {
	var toolResults []llm.ToolResultContent
	var dataResults []*l8agent.L8AgentDataResult

	bearerToken := ""

	for _, block := range resp.Content {
		if block.Type != "tool_use" {
			continue
		}

		inputJSON, _ := json.Marshal(block.Input)
		result, err := h.toolExec.Execute(block.Name, string(inputJSON), bearerToken)

		var toolResult llm.ToolResultContent
		toolResult.Type = "tool_result"
		toolResult.ToolUseID = block.ID

		if err != nil {
			toolResult.Content = "Error: " + err.Error()
			toolResult.IsError = true
		} else {
			masked := h.maskingProxy.MaskJSON(result, block.Name, tokenMap)
			toolResult.Content = masked

			if block.Name == "l8query" {
				var input map[string]interface{}
				json.Unmarshal(inputJSON, &input)
				query, _ := input["query"].(string)
				dataResults = append(dataResults, &l8agent.L8AgentDataResult{
					ModelName: block.Name,
					Query:     query,
					DataJson:  result,
				})
			}
		}

		toolResults = append(toolResults, toolResult)
	}

	return toolResults, dataResults
}

func extractText(resp *llm.Response) string {
	for _, block := range resp.Content {
		if block.Type == "text" {
			return block.Text
		}
	}
	return ""
}

func marshalJSON(v interface{}) string {
	data, _ := json.Marshal(v)
	return string(data)
}

func truncateTitle(msg string) string {
	if len(msg) > 50 {
		return msg[:47] + "..."
	}
	return msg
}

func getMaxToolCalls() int {
	if v := os.Getenv("L8AGENT_MAX_TOOL_CALLS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return defaultMaxToolCalls
}

func getMaxHistory() int {
	if v := os.Getenv("L8AGENT_MAX_HISTORY"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return defaultMaxHistory
}
