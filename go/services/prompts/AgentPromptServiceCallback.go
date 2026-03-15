/*
(c) 2025 Sharon Aicler (saichler@gmail.com)

Layer 8 Ecosystem is licensed under the Apache License, Version 2.0.
*/

package prompts

import (
	"errors"

	"github.com/saichler/l8agent/go/types/l8agent"
	"github.com/saichler/l8types/go/ifs"
)

type promptCallback struct{}

func newPromptCallback() ifs.IServiceCallback {
	return &promptCallback{}
}

func (c *promptCallback) Before(any interface{}, action ifs.Action, cont bool, vnic ifs.IVNic) (interface{}, bool, error) {
	entity, ok := any.(*l8agent.L8AgentPrompt)
	if !ok {
		return nil, false, errors.New("invalid L8AgentPrompt type")
	}

	if action == ifs.POST {
		if entity.PromptId == "" {
			entity.PromptId = ifs.NewUuid()
		}
		if entity.Status == 0 {
			entity.Status = int32(l8agent.L8AgentPromptStatus_L8_AGENT_PROMPT_STATUS_ACTIVE)
		}
	}

	if err := validatePrompt(entity); err != nil {
		return nil, false, err
	}

	return nil, true, nil
}

func (c *promptCallback) After(any interface{}, action ifs.Action, cont bool, vnic ifs.IVNic) (interface{}, bool, error) {
	return nil, true, nil
}

func validatePrompt(entity *l8agent.L8AgentPrompt) error {
	if entity.PromptId == "" {
		return errors.New("PromptId is required")
	}
	if entity.Name == "" {
		return errors.New("Name is required")
	}
	if entity.SystemPrompt == "" {
		return errors.New("SystemPrompt is required")
	}
	return nil
}
