/*
(c) 2025 Sharon Aicler (saichler@gmail.com)

Layer 8 Ecosystem is licensed under the Apache License, Version 2.0.
*/

package chat

import (
	"github.com/saichler/l8agent/go/masking"
	"github.com/saichler/l8agent/go/types/l8agent"
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
	resp, err := h.llmClient.SendMessage(webSystemPrompt, msgs, nil)
	if err != nil {
		return "", 0, err
	}
	return extractText(resp), resp.Usage.InputTokens + resp.Usage.OutputTokens, nil
}
