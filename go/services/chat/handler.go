/*
(c) 2025 Sharon Aicler (saichler@gmail.com)

Layer 8 Ecosystem is licensed under the Apache License, Version 2.0.
*/

package chat

import (
	"fmt"

	"github.com/saichler/l8agent/go/executor"
	"github.com/saichler/l8agent/go/llm"
	"github.com/saichler/l8agent/go/masking"
	"github.com/saichler/l8agent/go/schema"
	"github.com/saichler/l8agent/go/services/conversations"
	"github.com/saichler/l8agent/go/services/messages"
	"github.com/saichler/l8agent/go/types/l8agent"
	"github.com/saichler/l8srlz/go/serialize/object"
	"github.com/saichler/l8types/go/ifs"
)

// chatHandler implements IServiceHandler for the chat orchestration service.
type chatHandler struct {
	llmClient    *llm.Client
	schema       *schema.Provider
	toolExec     *executor.ToolExecutor
	maskingProxy *masking.Proxy
	sla          *ifs.ServiceLevelAgreement
}

func newChatHandler(llmClient *llm.Client, schemaProvider *schema.Provider, toolExec *executor.ToolExecutor, maskingProxy *masking.Proxy) *chatHandler {
	return &chatHandler{
		llmClient:    llmClient,
		schema:       schemaProvider,
		toolExec:     toolExec,
		maskingProxy: maskingProxy,
	}
}

func (h *chatHandler) Activate(sla *ifs.ServiceLevelAgreement, vnic ifs.IVNic) error {
	h.sla = sla
	return nil
}

func (h *chatHandler) DeActivate() error {
	return nil
}

// Post handles a chat request. The facade L8AgentChatConversation carries the
// conversation metadata plus a Messages slice with the latest user message.
// Returns the assistant's L8AgentChatMessage.
func (h *chatHandler) Post(elem ifs.IElements, vnic ifs.IVNic) ifs.IElements {
	if elem.Element() == nil {
		return object.New(errMsg("no request body"), nil)
	}

	facade, ok := elem.Element().(*l8agent.L8AgentChatConversation)
	if !ok {
		return object.New(errMsg("invalid L8AgentChatConversation type"), nil)
	}

	if len(facade.Messages) == 0 || facade.Messages[len(facade.Messages)-1].Message == "" {
		return object.New(errMsg("message is required"), nil)
	}

	resp, err := orchestrate(h, facade, vnic)
	if err != nil {
		return object.New(err, nil)
	}

	return object.New(nil, resp)
}

func (h *chatHandler) Put(elem ifs.IElements, vnic ifs.IVNic) ifs.IElements {
	return object.New(nil, nil)
}

func (h *chatHandler) Patch(elem ifs.IElements, vnic ifs.IVNic) ifs.IElements {
	return object.New(nil, nil)
}

// Delete removes a conversation and all its messages.
func (h *chatHandler) Delete(elem ifs.IElements, vnic ifs.IVNic) ifs.IElements {
	if elem.Element() == nil {
		return object.New(errMsg("no request body"), nil)
	}

	facade, ok := elem.Element().(*l8agent.L8AgentChatConversation)
	if !ok {
		return object.New(errMsg("invalid type for delete"), nil)
	}

	if facade.ConversationId == "" {
		return object.New(errMsg("ConversationId is required"), nil)
	}

	// Delete messages first, then conversation
	if err := messages.DeleteMessages(facade.ConversationId, vnic); err != nil {
		fmt.Println("[agent] warning: failed to delete messages:", err)
	}
	if err := conversations.DeleteConversation(facade.ConversationId, vnic); err != nil {
		return object.New(err, nil)
	}

	return object.New(nil, nil)
}

// Get reconstructs conversations with their messages from the two ORM services.
func (h *chatHandler) Get(elem ifs.IElements, vnic ifs.IVNic) ifs.IElements {
	// List all conversations
	convoList, err := conversations.ListConversations(vnic)
	if err != nil {
		return object.New(err, nil)
	}

	// Build facade list with messages
	var result []*l8agent.L8AgentChatConversation
	for _, convo := range convoList {
		msgs, err := messages.GetMessages(convo.ConversationId, vnic)
		if err != nil {
			fmt.Println("[agent] warning: failed to load messages for", convo.ConversationId, ":", err)
			msgs = nil
		}
		facade := &l8agent.L8AgentChatConversation{
			ConversationId: convo.ConversationId,
			UserId:         convo.UserId,
			Title:          convo.Title,
			Status:         convo.Status,
			Messages:       msgs,
			CreatedAt:      convo.CreatedAt,
			UpdatedAt:      convo.UpdatedAt,
		}
		result = append(result, facade)
	}

	return object.New(nil, &l8agent.L8AgentChatConversationList{List: result})
}

func (h *chatHandler) Failed(elem ifs.IElements, vnic ifs.IVNic, msg *ifs.Message) ifs.IElements {
	return object.New(nil, nil)
}

func (h *chatHandler) TransactionConfig() ifs.ITransactionConfig {
	return nil
}

func (h *chatHandler) WebService() ifs.IWebService {
	if h.sla != nil {
		return h.sla.WebService()
	}
	return nil
}

func errMsg(msg string) error {
	return &chatError{msg}
}

type chatError struct {
	msg string
}

func (e *chatError) Error() string {
	return e.msg
}
