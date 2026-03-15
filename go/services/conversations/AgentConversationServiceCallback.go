/*
(c) 2025 Sharon Aicler (saichler@gmail.com)

Layer 8 Ecosystem is licensed under the Apache License, Version 2.0.
*/

package conversations

import (
	"errors"
	"time"

	"github.com/saichler/l8agent/go/types/l8agent"
	"github.com/saichler/l8types/go/ifs"
)

type conversationCallback struct{}

func newConversationCallback() ifs.IServiceCallback {
	return &conversationCallback{}
}

func (c *conversationCallback) Before(any interface{}, action ifs.Action, cont bool, vnic ifs.IVNic) (interface{}, bool, error) {
	entity, ok := any.(*l8agent.L8AgentConversation)
	if !ok {
		return nil, false, errors.New("invalid L8AgentConversation type")
	}

	if action == ifs.POST {
		generateID(&entity.ConversationId)
		now := time.Now().Unix()
		entity.CreatedAt = now
		entity.UpdatedAt = now
		if entity.Status == 0 {
			entity.Status = int32(l8agent.L8AgentConvoStatus_L8_AGENT_CONVO_STATUS_ACTIVE)
		}
	}

	if action == ifs.PUT || action == ifs.PATCH {
		entity.UpdatedAt = time.Now().Unix()
	}

	if err := validateConversation(entity); err != nil {
		return nil, false, err
	}

	return nil, true, nil
}

func (c *conversationCallback) After(any interface{}, action ifs.Action, cont bool, vnic ifs.IVNic) (interface{}, bool, error) {
	return nil, true, nil
}

func validateConversation(entity *l8agent.L8AgentConversation) error {
	if entity.ConversationId == "" {
		return errors.New("ConversationId is required")
	}
	if entity.UserId == "" {
		return errors.New("UserId is required")
	}
	return nil
}

func generateID(field *string) {
	if *field == "" {
		*field = ifs.NewUuid()
	}
}
