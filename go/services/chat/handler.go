/*
(c) 2025 Sharon Aicler (saichler@gmail.com)

Layer 8 Ecosystem is licensed under the Apache License, Version 2.0.
*/

package chat

import (
	"github.com/saichler/l8agent/go/executor"
	"github.com/saichler/l8agent/go/llm"
	"github.com/saichler/l8agent/go/masking"
	"github.com/saichler/l8agent/go/schema"
	"github.com/saichler/l8agent/go/types/l8agent"
	"github.com/saichler/l8srlz/go/serialize/object"
	"github.com/saichler/l8types/go/ifs"
)

// chatHandler implements IServiceHandler for the chat orchestration service.
// Only POST is meaningful (send a message). All other operations return empty.
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

func (h *chatHandler) Post(elem ifs.IElements, vnic ifs.IVNic) ifs.IElements {
	if elem.Element() == nil {
		return object.New(errMsg("no request body"))
	}

	req, ok := elem.Element().(*l8agent.L8AgentChatRequest)
	if !ok {
		return object.New(errMsg("invalid L8AgentChatRequest type"))
	}

	if req.Message == "" {
		return object.New(errMsg("message is required"))
	}

	resp, err := orchestrate(h, req, vnic)
	if err != nil {
		return object.New(err)
	}

	return object.New(nil, resp)
}

func (h *chatHandler) Put(elem ifs.IElements, vnic ifs.IVNic) ifs.IElements {
	return object.New(nil)
}

func (h *chatHandler) Patch(elem ifs.IElements, vnic ifs.IVNic) ifs.IElements {
	return object.New(nil)
}

func (h *chatHandler) Delete(elem ifs.IElements, vnic ifs.IVNic) ifs.IElements {
	return object.New(nil)
}

func (h *chatHandler) Get(elem ifs.IElements, vnic ifs.IVNic) ifs.IElements {
	return object.New(nil)
}

func (h *chatHandler) Failed(elem ifs.IElements, vnic ifs.IVNic, msg *ifs.Message) ifs.IElements {
	return object.New(nil)
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
