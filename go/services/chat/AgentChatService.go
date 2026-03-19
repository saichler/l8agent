/*
(c) 2025 Sharon Aicler (saichler@gmail.com)

Layer 8 Ecosystem is licensed under the Apache License, Version 2.0.
*/

// Package chat provides the AgentChat orchestration service.
// This service does NOT persist data — it coordinates the conversation
// between the user, the LLM, and the Layer 8 service endpoints.
// Persistence is handled by the AgntConvo and AgntMsg ORM services.
package chat

import (
	"github.com/saichler/l8agent/go/executor"
	"github.com/saichler/l8agent/go/llm"
	"github.com/saichler/l8agent/go/masking"
	"github.com/saichler/l8agent/go/schema"
	"github.com/saichler/l8agent/go/types/l8agent"
	"github.com/saichler/l8types/go/ifs"
	"github.com/saichler/l8types/go/types/l8api"
	"github.com/saichler/l8utils/go/utils/web"
)

const (
	ServiceName = "AgntChat"
)

func Activate(area byte, vnic ifs.IVNic, llmClient *llm.Client, schemaProvider *schema.Provider, toolExec *executor.ToolExecutor, maskingProxy *masking.Proxy) {
	handler := newChatHandler(llmClient, schemaProvider, toolExec, maskingProxy)

	sla := ifs.NewServiceLevelAgreement(handler, ServiceName, area, false, newChatCallback(handler))
	sla.SetServiceItem(&l8agent.L8AgentChatConversation{})
	sla.SetServiceItemList(&l8agent.L8AgentChatConversationList{})
	sla.SetPrimaryKeys("ConversationId")

	ws := web.New(ServiceName, area, 0)
	ws.AddEndpoint(&l8agent.L8AgentChatConversation{}, ifs.POST, &l8agent.L8AgentChatMessage{})
	ws.AddEndpoint(&l8api.L8Query{}, ifs.GET, &l8agent.L8AgentChatConversationList{})
	ws.AddEndpoint(&l8api.L8Query{}, ifs.DELETE, &l8agent.L8AgentChatConversationList{})
	sla.SetWebService(ws)

	vnic.Resources().Services().Activate(sla, vnic)
}
