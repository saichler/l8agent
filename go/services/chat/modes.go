/*
(c) 2025 Sharon Aicler (saichler@gmail.com)

Layer 8 Ecosystem is licensed under the Apache License, Version 2.0.
*/

package chat

import (
	"fmt"
	"log"
	"time"

	"github.com/saichler/l8agent/go/llm"
	"github.com/saichler/l8agent/go/masking"
	"github.com/saichler/l8agent/go/types/l8agent"
	"github.com/saichler/l8agent/go/webtools"
)

// answerHelp answers an '@help' prompt from the project's help documentation only (no tools).
func answerHelp(h *chatHandler, question string, history []*l8agent.L8AgentChatMessage, tokenMap *masking.TokenMap) (string, int, error) {
	if h.helpDocs == "" {
		return noHelpDocsReply, 0, nil
	}
	if question == "" {
		question = emptyHelpAsk
	}
	msgs := buildMessages(history)
	msgs[len(msgs)-1].Content = h.maskingProxy.MaskText(question, tokenMap)
	resp, err := h.llmClient.SendMessage(helpPrompt(h.helpDocs), msgs, nil)
	if err != nil {
		return "", 0, err
	}
	return extractText(resp), resp.Usage.InputTokens + resp.Usage.OutputTokens, nil
}

// answerWeb answers an '@web' prompt from general knowledge (no tools), after validating that the question is
// not about the application's data.
func answerWeb(h *chatHandler, question string, history []*l8agent.L8AgentChatMessage, tokenMap *masking.TokenMap) (string, int, error) {
	if question == "" {
		return webEmptyReply, 0, nil
	}
	ok, err := webAllowed(h, question)
	if err != nil {
		return "", 0, err
	}
	if !ok {
		return webRejectReply, 0, nil
	}
	msgs := buildMessages(history)
	msgs[len(msgs)-1].Content = h.maskingProxy.MaskText(question, tokenMap)
	if h.web != nil {
		return webToolLoop(h, msgs, tokenMap)
	}
	resp, err := h.llmClient.SendMessage(webSystemPrompt, msgs, nil)
	if err != nil {
		return "", 0, err
	}
	return extractText(resp), resp.Usage.InputTokens + resp.Usage.OutputTokens, nil
}

// maxWebToolCalls bounds the searches and fetches of one '@web' prompt.
const maxWebToolCalls = 6

// webToolLoop answers an '@web' prompt with live web search: the model asks for web_search / fetch_url, the
// web tools service runs them, and the loop ends when the model answers (or the budget is spent). The
// application's data tools are never offered. Results are public web data, so they are not masked; masked
// tokens in the model's arguments are restored first.
func webToolLoop(h *chatHandler, msgs []llm.Message, tokenMap *masking.TokenMap) (string, int, error) {
	system := fmt.Sprintf(webToolsSystemPrompt, time.Now().Format("Monday, January 2, 2006"))
	defs := webtools.Definitions()
	total := 0
	calls := 0
	for calls <= maxWebToolCalls {
		resp, err := h.llmClient.SendMessage(system, msgs, defs)
		if err != nil {
			return "", total, err
		}
		total += resp.Usage.InputTokens + resp.Usage.OutputTokens
		if resp.StopReason != "tool_use" {
			return extractText(resp), total, nil
		}
		var results []llm.ToolResultContent
		for _, block := range resp.Content {
			if block.Type != "tool_use" {
				continue
			}
			calls++
			r := llm.ToolResultContent{Type: "tool_result", ToolUseID: block.ID}
			if calls > maxWebToolCalls {
				r.Content, r.IsError = "Error: tool call limit reached; answer with what you have.", true
			} else {
				start := time.Now()
				out, err := h.web.Execute(block.Name, tokenMap.Unmask(string(block.Input)))
				if err != nil {
					log.Printf("[agent @web] %s %s failed after %s: %v", block.Name, block.Input, time.Since(start).Round(time.Millisecond), err)
					r.Content, r.IsError = "Error: "+err.Error(), true
				} else {
					log.Printf("[agent @web] %s %s ok in %s (%d chars)", block.Name, block.Input, time.Since(start).Round(time.Millisecond), len(out))
					r.Content = out
				}
			}
			results = append(results, r)
		}
		msgs = append(msgs, llm.Message{Role: "assistant", Content: resp.Content})
		msgs = append(msgs, llm.Message{Role: "user", Content: results})
	}
	// Budget spent: force a text answer from what was gathered.
	resp, err := h.llmClient.SendFinalMessage(system+finalAnswerPrompt, msgs, defs)
	if err != nil {
		return "", total, err
	}
	total += resp.Usage.InputTokens + resp.Usage.OutputTokens
	return extractText(resp), total, nil
}
