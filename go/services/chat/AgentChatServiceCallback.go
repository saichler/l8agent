/*
(c) 2025 Sharon Aicler (saichler@gmail.com)

Layer 8 Ecosystem is licensed under the Apache License, Version 2.0.
*/

package chat

import (
	"github.com/saichler/l8types/go/ifs"
)

// chatCallback is a no-op callback. Chat orchestration runs in the handler's Post().
type chatCallback struct{}

func newChatCallback(handler *chatHandler) ifs.IServiceCallback {
	return &chatCallback{}
}

func (c *chatCallback) Before(any interface{}, action ifs.Action, cont bool, vnic ifs.IVNic) (interface{}, bool, error) {
	return nil, true, nil
}

func (c *chatCallback) After(any interface{}, action ifs.Action, cont bool, vnic ifs.IVNic) (interface{}, bool, error) {
	return nil, true, nil
}
