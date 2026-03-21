/*
(c) 2025 Sharon Aicler (saichler@gmail.com)

Layer 8 Ecosystem is licensed under the Apache License, Version 2.0.
*/

// Package l8agent provides the AI Agent shared library for Layer 8 ecosystem projects.
// Consumer projects call Initialize() with their config to activate all agent services.
package l8agent

import (
	"fmt"

	"github.com/saichler/l8agent/go/executor"
	"github.com/saichler/l8agent/go/llm"
	"github.com/saichler/l8agent/go/masking"
	"github.com/saichler/l8agent/go/schema"
	"github.com/saichler/l8agent/go/services/chat"
	"github.com/saichler/l8agent/go/services/conversations"
	"github.com/saichler/l8agent/go/services/messages"
	"github.com/saichler/l8agent/go/services/prompts"
	"github.com/saichler/l8agent/go/types/l8agent"
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
	LLMCreds         string              // LLM credential key (e.g., "Anthropic")
	MaskingOverrides MaskingOverrides    // Optional: project-specific field overrides
	DefaultPrompts   []*l8agent.L8AgentPrompt // Optional: built-in prompt templates
}

// Initialize activates ORM agent services (Conversations, Messages, Prompts).
// Call this during parallel service activation. Chat service is NOT activated here
// because it needs the introspector to be fully populated first.
func Initialize(config AgentConfig, vnic ifs.IVNic) error {
	// Register types for introspection
	registerTypes(config.Resources)

	// Activate Conversation CRUD service (Service 1: metadata only)
	conversations.Activate(config.DBCreds, config.DBName, config.ServiceArea, vnic)

	// Activate Message CRUD service (Service 2: chat messages)
	messages.Activate(config.DBCreds, config.DBName, config.ServiceArea, vnic)

	// Activate Prompt CRUD service
	prompts.Activate(config.DBCreds, config.DBName, config.ServiceArea, vnic)

	// Seed default prompts if provided
	if len(config.DefaultPrompts) > 0 {
		seedDefaultPrompts(config.DefaultPrompts, config.ServiceArea, vnic)
	}

	fmt.Printf("[agent] AI Agent ORM services initialized (area=%d)\n", config.ServiceArea)
	return nil
}

// InitializeChat activates the Chat orchestration service.
// Call this AFTER all other services are activated so the introspector is fully populated.
func InitializeChat(config AgentConfig, vnic ifs.IVNic) error {
	// Create LLM client from security credentials
	var llmClient *llm.Client
	_, _, apiKey, _, err := vnic.Resources().Security().Credential(config.LLMCreds, "API_KEY", vnic.Resources())
	if err != nil {
		fmt.Println("[agent] Warning: failed to retrieve LLM credentials:", err)
	} else if apiKey != "" {
		llmClient = llm.NewClient(apiKey)
	} else {
		fmt.Println("[agent] Warning: LLM API key is empty. Chat service will return errors.")
	}

	// Create Schema Provider
	schemaProvider := schema.NewProvider(config.Resources)

	// Create Masking Proxy
	maskingConfig := masking.NewConfig(config.MaskingOverrides)
	maskingProxy := masking.NewProxy(maskingConfig)

	// Create Tool Executor
	toolExec := executor.NewToolExecutor(vnic, schemaProvider)

	// Activate Chat orchestration service
	chat.Activate(config.ServiceArea, vnic, llmClient, schemaProvider, toolExec, maskingProxy)

	fmt.Printf("[agent] AI Agent Chat service initialized (area=%d)\n", config.ServiceArea)
	return nil
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
		fmt.Println("[agent] Warning: could not get prompt service API for seeding")
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
