/*
(c) 2025 Sharon Aicler (saichler@gmail.com)

Layer 8 Ecosystem is licensed under the Apache License, Version 2.0.
*/

// Package l8agent provides the AI Agent shared library for Layer 8 ecosystem projects.
// Consumer projects call Initialize() with their config to activate all agent services.
package l8agent

import (
	"net/http"

	"github.com/saichler/l8agent/go/executor"
	"github.com/saichler/l8agent/go/lanfirst"
	"github.com/saichler/l8agent/go/llm"
	"github.com/saichler/l8agent/go/masking"
	"github.com/saichler/l8agent/go/schema"
	"github.com/saichler/l8agent/go/services/chat"
	"github.com/saichler/l8agent/go/services/conversations"
	"github.com/saichler/l8agent/go/services/messages"
	"github.com/saichler/l8agent/go/services/prompts"
	"github.com/saichler/l8agent/go/types/l8agent"
	"github.com/saichler/l8agent/go/webtools"
	"github.com/saichler/l8types/go/ifs"
)

// MaskingOverrides is a function that overrides the default field classification.
type MaskingOverrides = masking.OverrideFunc

// AgentConfig is the configuration for initializing the agent.
type AgentConfig struct {
	Resources        ifs.IResources
	ServiceArea      byte                // Service area for agent services
	DBCreds          string              // Database credential key
	DBName           string              // Database name
	LLMCreds         string              // LLM credential group (e.g., "Anthropic")
	LLMProvider      string              // "anthropic" (default) or "openai" (OpenAI-compatible endpoint)
	LLMCredKey       string              // Credential name within LLMCreds (default "API_KEY")
	MaskingOverrides MaskingOverrides    // Optional: project-specific field overrides
	DefaultPrompts   []*l8agent.L8AgentPrompt // Optional: built-in prompt templates
	WebToolsURL      string              // Optional: base URL of a web tools service (/v1/search, /v1/fetch) giving '@web' prompts live web search; authenticated with the LLM API key
	LANRoutes        map[string]string   // Optional: public host -> LAN "ip:port" (e.g. "llm.test-llm.dev": "192.168.1.124:8080"). Those hosts are tried on the LAN first; the public address is used only if the LAN cannot be reached
	LANCA            string              // Optional: PEM of the CA that signed the LAN servers' certificates
	HelpDocs         string              // Optional: markdown help for '@help <question>' prompts (menus and how-tos of the project)
}

// Initialize activates ORM agent services (Conversations, Messages, Prompts).
// Call this during parallel service activation. Chat service is NOT activated here
// because it needs the introspector to be fully populated first.
func Initialize(config AgentConfig, vnic ifs.IVNic) error {
	// Register types for introspection
	registerTypes(config.Resources)

	// Activate Conversation CRUD service (Service 1: metadata only)
	if err := conversations.Activate(config.DBCreds, config.DBName, config.ServiceArea, vnic); err != nil {
		return err
	}

	// Activate Message CRUD service (Service 2: chat messages)
	if err := messages.Activate(config.DBCreds, config.DBName, config.ServiceArea, vnic); err != nil {
		return err
	}

	// Activate Prompt CRUD service
	if err := prompts.Activate(config.DBCreds, config.DBName, config.ServiceArea, vnic); err != nil {
		return err
	}

	// Seed default prompts if provided
	if len(config.DefaultPrompts) > 0 {
		seedDefaultPrompts(config.DefaultPrompts, config.ServiceArea, vnic)
	}

	return nil
}

// InitializeChat activates the Chat orchestration service.
// Call this AFTER all other services are activated so the introspector is fully populated.
func InitializeChat(config AgentConfig, vnic ifs.IVNic) error {
	// Create LLM client from security credentials
	var llmClient *llm.Client
	credKey := config.LLMCredKey
	if credKey == "" {
		credKey = "API_KEY"
	}
	// Credential layout: aside=base URL, yside=model (both used by the openai provider), zside=API key.
	_, baseURL, apiKey, model, err := vnic.Resources().Security().Credential(config.LLMCreds, credKey, vnic.Resources())
	if err != nil {
		vnic.Resources().Logger().Warning("Failed to retrieve LLM credentials: ", err.Error())
	} else if apiKey != "" {
		if config.LLMProvider == "openai" {
			llmClient = llm.NewOpenAIClient(baseURL, apiKey, model)
			llmClient.WrapTransport(lanWrap(config, vnic))
		} else {
			llmClient = llm.NewClient(apiKey)
		}
	} else {
		vnic.Resources().Logger().Warning("LLM API key is empty. Chat service will return errors.")
	}

	// Create Schema Provider
	schemaProvider := schema.NewProvider(config.Resources)

	// Create Masking Proxy
	maskingConfig := masking.NewConfig(config.MaskingOverrides)
	maskingProxy := masking.NewProxy(maskingConfig)

	// Create Tool Executor
	toolExec := executor.NewToolExecutor(vnic, schemaProvider)

	// Activate Chat orchestration service
	chat.Activate(config.ServiceArea, vnic, llmClient, schemaProvider, toolExec, maskingProxy, config.HelpDocs, webtools.New(config.WebToolsURL, apiKey, lanWrap(config, vnic)))

	return nil
}

// lanWrap returns a transport wrapper that tries config.LANRoutes on the local network before the public
// address. With no routes it changes nothing; a bad LANCA is logged and the LAN is not used.
func lanWrap(config AgentConfig, vnic ifs.IVNic) func(http.RoundTripper) http.RoundTripper {
	return func(rt http.RoundTripper) http.RoundTripper {
		wrapped, err := lanfirst.Wrap(rt, config.LANRoutes, config.LANCA)
		if err != nil {
			vnic.Resources().Logger().Warning("LAN-first disabled: ", err.Error())
			return rt
		}
		return wrapped
	}
}

// registerTypes registers agent protobuf types with the introspector.
func registerTypes(resources ifs.IResources) {
	resources.Introspector().Decorators().AddPrimaryKeyDecorator(&l8agent.L8AgentConversation{}, "ConversationId")
	resources.Registry().Register(&l8agent.L8AgentConversationList{})

	resources.Introspector().Decorators().AddPrimaryKeyDecorator(&l8agent.L8AgentPrompt{}, "PromptId")
	resources.Registry().Register(&l8agent.L8AgentPromptList{})

	// Register chat message type
	resources.Introspector().Decorators().AddPrimaryKeyDecorator(&l8agent.L8AgentChatMessage{}, "ConversationId")
	resources.Registry().Register(&l8agent.L8AgentChatMessageList{})

	// Register chat conversation facade type for web service deserialization
	resources.Registry().Register(&l8agent.L8AgentChatConversation{})
	resources.Registry().Register(&l8agent.L8AgentChatConversationList{})
}

// seedDefaultPrompts creates built-in prompt templates if they don't already exist.
func seedDefaultPrompts(defaults []*l8agent.L8AgentPrompt, area byte, vnic ifs.IVNic) {
	api := vnic.ServiceAPI(prompts.ServiceName, area)
	if api == nil {
		return
	}
	for _, prompt := range defaults {
		if prompt.PromptId == "" {
			prompt.PromptId = ifs.NewUuid()
		}
		if prompt.Status == 0 {
			prompt.Status = int32(l8agent.L8AgentPromptStatus_L8_AGENT_PROMPT_STATUS_ACTIVE)
		}
		api.Post(prompt)
	}
}
