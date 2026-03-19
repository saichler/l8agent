/*
(c) 2025 Sharon Aicler (saichler@gmail.com)

Layer 8 Ecosystem is licensed under the Apache License, Version 2.0.
*/

package messages

import (
	"errors"
	"time"

	"github.com/saichler/l8agent/go/types/l8agent"
	"github.com/saichler/l8types/go/ifs"
)

type messageCallback struct{}

func newMessageCallback() ifs.IServiceCallback {
	return &messageCallback{}
}

func (c *messageCallback) Before(any interface{}, action ifs.Action, cont bool, vnic ifs.IVNic) (interface{}, bool, error) {
	entity, ok := any.(*l8agent.L8AgentChatMessage)
	if !ok {
		return nil, false, errors.New("invalid L8AgentChatMessage type")
	}

	if action == ifs.POST {
		if entity.Timestamp == 0 {
			entity.Timestamp = time.Now().Unix()
		}
	}

	if err := validateMessage(entity); err != nil {
		return nil, false, err
	}

	return nil, true, nil
}

func (c *messageCallback) After(any interface{}, action ifs.Action, cont bool, vnic ifs.IVNic) (interface{}, bool, error) {
	return nil, true, nil
}

func validateMessage(entity *l8agent.L8AgentChatMessage) error {
	if entity.ConversationId == "" {
		return errors.New("ConversationId is required")
	}
	if entity.Message == "" {
		return errors.New("Message is required")
	}
	return nil
}
