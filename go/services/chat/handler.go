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


func (h *chatHandler) Activate(sla *ifs.ServiceLevelAgreement, vnic ifs.IVNic) error {
	h.sla = sla
	args := sla.Args()
	fmt.Println("[agent] Activate called, args count:", len(args))
	if len(args) >= 4 {
		if v, ok := args[0].(*llm.Client); ok {
			h.llmClient = v
			fmt.Println("[agent] llmClient set:", v != nil)
		}
		if v, ok := args[1].(*schema.Provider); ok {
			h.schema = v
			fmt.Println("[agent] schema set:", v != nil)
		}
		if v, ok := args[2].(*executor.ToolExecutor); ok {
			h.toolExec = v
			fmt.Println("[agent] toolExec set:", v != nil)
		}
		if v, ok := args[3].(*masking.Proxy); ok {
			h.maskingProxy = v
			fmt.Println("[agent] maskingProxy set:", v != nil)
		}
	} else {
		fmt.Println("[agent] WARNING: not enough args in SLA, handler fields will be nil")
	}

	// Print all models in the introspector
	nodes := vnic.Resources().Introspector().Nodes(true, true)
	fmt.Println("[agent] Introspector models (", len(nodes), "):")
	for _, node := range nodes {
		fmt.Println("[agent]   -", node.TypeName)
	}

	return nil
}

func (h *chatHandler) DeActivate() error {
	return nil
}

// Post handles a chat request. The facade L8AgentChatConversation carries the
// conversation metadata plus a Messages slice with the latest user message.
// Returns the assistant's L8AgentChatMessage.
func (h *chatHandler) Post(elem ifs.IElements, vnic ifs.IVNic) ifs.IElements {
	fmt.Println("[agent] Post called, llmClient:", h.llmClient != nil, "schema:", h.schema != nil, "toolExec:", h.toolExec != nil, "maskingProxy:", h.maskingProxy != nil)
	if elem.Element() == nil {
		fmt.Println("[agent] Post: no request body")
		return object.New(errMsg("no request body"), nil)
	}

	facade, ok := elem.Element().(*l8agent.L8AgentChatConversation)
	if !ok {
		fmt.Println("[agent] Post: invalid type, got:", fmt.Sprintf("%T", elem.Element()))
		return object.New(errMsg("invalid L8AgentChatConversation type"), nil)
	}

	fmt.Println("[agent] Post: conversationId:", facade.ConversationId, "messages:", len(facade.Messages))
	if len(facade.Messages) == 0 || facade.Messages[len(facade.Messages)-1].Message == "" {
		fmt.Println("[agent] Post: message is required")
		return object.New(errMsg("message is required"), nil)
	}

	fmt.Println("[agent] Post: calling orchestrate with message:", facade.Messages[len(facade.Messages)-1].Message)
	resp, err := orchestrate(h, facade, vnic)
	if err != nil {
		fmt.Println("[agent] Post: orchestrate error:", err)
		return object.New(err, nil)
	}

	if resp == nil {
		fmt.Println("[agent] Post: orchestrate returned nil response")
	} else {
		fmt.Println("[agent] Post: orchestrate success, response length:", len(resp.Message))
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
