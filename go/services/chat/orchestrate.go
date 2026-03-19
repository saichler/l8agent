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
	"github.com/saichler/l8agent/go/services/messages"
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

// orchestrate runs the full chat flow using the facade pattern:
// 1. Load or create conversation metadata (Service 1: AgntConvo)
// 2. Load history messages (Service 2: AgntMsg)
// 3. Save user message (Service 2)
// 4. Call LLM with tool loop
// 5. Save assistant message (Service 2)
// 6. Update conversation metadata (Service 1)
// 7. Return the assistant L8AgentChatMessage
func orchestrate(h *chatHandler, facade *l8agent.L8AgentChatConversation, vnic ifs.IVNic) (*l8agent.L8AgentChatMessage, error) {
	if h.llmClient == nil {
		return nil, fmt.Errorf("LLM client not configured. Set ANTHROPIC_API_KEY environment variable")
	}

	// Extract the user message from the facade
	userMsg := facade.Messages[len(facade.Messages)-1]

	// Step 1: Load or create conversation metadata
	convo, isNew, err := loadOrCreateConversation(facade, userMsg.Message, vnic)
	if err != nil {
		return nil, fmt.Errorf("conversation error: %w", err)
	}

	// Step 2: Load existing messages from Service 2
	var historyMsgs []*l8agent.L8AgentChatMessage
	if !isNew {
		historyMsgs, err = messages.GetMessages(convo.ConversationId, vnic)
		if err != nil {
			fmt.Println("[agent] warning: failed to load messages:", err)
		}
	}

	// Step 3: Determine sequence and save user message
	nextSeq := int32(len(historyMsgs) + 1)
	userChatMsg := &l8agent.L8AgentChatMessage{
		ConversationId: convo.ConversationId,
		Sequence:       nextSeq,
		IsLlm:          false,
		Message:        userMsg.Message,
		AllowedModules: userMsg.AllowedModules,
		Timestamp:      time.Now().Unix(),
	}
	if err := messages.SaveMessage(userChatMsg, vnic); err != nil {
		fmt.Println("[agent] warning: failed to save user message:", err)
	}

	// Step 4: Build LLM context
	allMsgs := append(historyMsgs, userChatMsg)
	tokenMap := masking.NewTokenMap()
	systemPrompt := baseSystemPrompt + "\n\n" + h.schema.GetTier1Schema()
	llmMessages := buildMessages(allMsgs)
	toolDefs := tools.GetToolDefinitions()

	// Step 5: Call LLM with tool loop
	response, toolCallCount, totalTokens, err := toolLoop(h, systemPrompt, llmMessages, toolDefs, tokenMap, vnic)
	if err != nil {
		return nil, fmt.Errorf("LLM error: %w", err)
	}

	// Step 6: Unmask the response
	response = tokenMap.Unmask(response)

	// Step 7: Save assistant message
	assistantMsg := &l8agent.L8AgentChatMessage{
		ConversationId: convo.ConversationId,
		Sequence:       nextSeq + 1,
		IsLlm:          true,
		Message:        response,
		Timestamp:      time.Now().Unix(),
		TokenCount:     int32(totalTokens),
	}
	if err := messages.SaveMessage(assistantMsg, vnic); err != nil {
		fmt.Println("[agent] warning: failed to save assistant message:", err)
	}

	// Step 8: Update conversation metadata
	convo.UpdatedAt = time.Now().Unix()
	if err := conversations.SaveConversation(convo, isNew, vnic); err != nil {
		fmt.Println("[agent] warning: failed to save conversation:", err)
	}

	_ = toolCallCount
	return assistantMsg, nil
}

func loadOrCreateConversation(facade *l8agent.L8AgentChatConversation, userMessage string, vnic ifs.IVNic) (*l8agent.L8AgentConversation, bool, error) {
	if facade.ConversationId != "" {
		convo, err := conversations.Conversation(facade.ConversationId, vnic)
		if err != nil {
			return nil, false, err
		}
		if convo != nil {
			return convo, false, nil
		}
	}

	convo := &l8agent.L8AgentConversation{
		ConversationId: ifs.NewUuid(),
		UserId:         facade.UserId,
		Title:          truncateTitle(userMessage),
		Status:         int32(l8agent.L8AgentConvoStatus_L8_AGENT_CONVO_STATUS_ACTIVE),
		CreatedAt:      time.Now().Unix(),
		UpdatedAt:      time.Now().Unix(),
	}
	if convo.UserId == "" {
		convo.UserId = "system"
	}
	return convo, true, nil
}

func buildMessages(msgs []*l8agent.L8AgentChatMessage) []llm.Message {
	maxHistory := getMaxHistory()
	if len(msgs) > maxHistory {
		msgs = msgs[len(msgs)-maxHistory:]
	}

	result := make([]llm.Message, 0, len(msgs))
	for _, msg := range msgs {
		role := "user"
		if msg.IsLlm {
			role = "assistant"
		}
		result = append(result, llm.Message{Role: role, Content: msg.Message})
	}
	return result
}

// toolLoop calls the LLM and executes tool calls until the LLM returns a text response.
func toolLoop(h *chatHandler, systemPrompt string, msgs []llm.Message, toolDefs []llm.Tool, tokenMap *masking.TokenMap, vnic ifs.IVNic) (string, int, int, error) {
	maxToolCalls := getMaxToolCalls()
	toolCallCount := 0
	totalTokens := 0

	for i := 0; i < maxToolCalls; i++ {
		resp, err := h.llmClient.SendMessage(systemPrompt, msgs, toolDefs)
		if err != nil {
			return "", toolCallCount, totalTokens, err
		}

		totalTokens += resp.Usage.InputTokens + resp.Usage.OutputTokens

		if resp.StopReason == "end_turn" {
			text := extractText(resp)
			return text, toolCallCount, totalTokens, nil
		}

		if resp.StopReason == "tool_use" {
			toolResults := executeToolCalls(h, resp, tokenMap, vnic)
			toolCallCount += len(toolResults)

			assistantContent := marshalJSON(resp.Content)
			msgs = append(msgs, llm.Message{Role: "assistant", Content: assistantContent})

			toolResultContent := marshalJSON(toolResults)
			msgs = append(msgs, llm.Message{Role: "user", Content: toolResultContent})
			continue
		}

		text := extractText(resp)
		return text, toolCallCount, totalTokens, nil
	}

	return "I've reached the maximum number of tool calls for this request. Please try a more specific question.", toolCallCount, totalTokens, nil
}

func executeToolCalls(h *chatHandler, resp *llm.Response, tokenMap *masking.TokenMap, vnic ifs.IVNic) []llm.ToolResultContent {
	var toolResults []llm.ToolResultContent

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
		}

		toolResults = append(toolResults, toolResult)
	}

	return toolResults
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
