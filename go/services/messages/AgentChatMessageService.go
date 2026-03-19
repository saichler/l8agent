/*
(c) 2025 Sharon Aicler (saichler@gmail.com)

Layer 8 Ecosystem is licensed under the Apache License, Version 2.0.
*/

// Package messages provides the AgentChatMessage CRUD service.
package messages

import (
	"database/sql"
	"fmt"
	"time"

	_ "github.com/lib/pq"
	"github.com/saichler/l8agent/go/types/l8agent"
	"github.com/saichler/l8orm/go/orm/persist"
	"github.com/saichler/l8orm/go/orm/plugins/postgres"
	"github.com/saichler/l8srlz/go/serialize/object"
	"github.com/saichler/l8types/go/ifs"
	"github.com/saichler/l8types/go/types/l8api"
	"github.com/saichler/l8types/go/types/l8web"
	"github.com/saichler/l8utils/go/utils/web"
)

const (
	ServiceName = "AgntMsg"
)

var serviceArea byte

func Activate(creds, dbname string, area byte, vnic ifs.IVNic) {
	serviceArea = area

	realdb, user, pass, _, err := vnic.Resources().Security().Credential(creds, dbname, vnic.Resources())
	if err != nil {
		panic(err)
	}
	db := openDB(realdb, user, pass)
	p := postgres.NewPostgres(db, vnic.Resources())

	sla := ifs.NewServiceLevelAgreement(&persist.OrmService{}, ServiceName, area, true, newMessageCallback())
	sla.SetServiceItem(&l8agent.L8AgentChatMessage{})
	sla.SetServiceItemList(&l8agent.L8AgentChatMessageList{})
	sla.SetPrimaryKeys("ConversationId")
	sla.SetArgs(p, true)
	sla.SetTransactional(true)
	sla.SetReplication(true)
	sla.SetReplicationCount(3)

	ws := web.New(ServiceName, area, 0)
	ws.AddEndpoint(&l8agent.L8AgentChatMessage{}, ifs.POST, &l8web.L8Empty{})
	ws.AddEndpoint(&l8agent.L8AgentChatMessageList{}, ifs.POST, &l8web.L8Empty{})
	ws.AddEndpoint(&l8agent.L8AgentChatMessage{}, ifs.PUT, &l8web.L8Empty{})
	ws.AddEndpoint(&l8api.L8Query{}, ifs.DELETE, &l8web.L8Empty{})
	ws.AddEndpoint(&l8api.L8Query{}, ifs.GET, &l8agent.L8AgentChatMessageList{})
	sla.SetWebService(ws)

	vnic.Resources().Services().Activate(sla, vnic)
}

// GetMessages retrieves all messages for a conversation, ordered by sequence.
func GetMessages(conversationId string, vnic ifs.IVNic) ([]*l8agent.L8AgentChatMessage, error) {
	filter := &l8agent.L8AgentChatMessage{ConversationId: conversationId}
	handler, ok := vnic.Resources().Services().ServiceHandler(ServiceName, serviceArea)
	if !ok {
		resp := vnic.Request("", ServiceName, serviceArea, ifs.GET, filter, 30)
		if resp.Error() != nil {
			return nil, resp.Error()
		}
		if resp.Element() != nil {
			list, ok := resp.Element().(*l8agent.L8AgentChatMessageList)
			if ok && list != nil {
				return list.List, nil
			}
		}
		return nil, nil
	}
	resp := handler.Get(object.New(nil, filter), vnic)
	if resp.Error() != nil {
		return nil, resp.Error()
	}
	if resp.Element() != nil {
		list, ok := resp.Element().(*l8agent.L8AgentChatMessageList)
		if ok && list != nil {
			return list.List, nil
		}
	}
	return nil, nil
}

// SaveMessage persists a single chat message.
func SaveMessage(msg *l8agent.L8AgentChatMessage, vnic ifs.IVNic) error {
	handler, ok := vnic.Resources().Services().ServiceHandler(ServiceName, serviceArea)
	if !ok {
		resp := vnic.Request("", ServiceName, serviceArea, ifs.POST, msg, 30)
		return resp.Error()
	}
	resp := handler.Post(object.New(nil, msg), vnic)
	return resp.Error()
}

// DeleteMessages removes all messages for a conversation.
func DeleteMessages(conversationId string, vnic ifs.IVNic) error {
	filter := &l8agent.L8AgentChatMessage{ConversationId: conversationId}
	handler, ok := vnic.Resources().Services().ServiceHandler(ServiceName, serviceArea)
	if !ok {
		resp := vnic.Request("", ServiceName, serviceArea, ifs.DELETE, filter, 30)
		return resp.Error()
	}
	resp := handler.Delete(object.New(nil, filter), vnic)
	return resp.Error()
}

func openDB(dbname, user, pass string) *sql.DB {
	psqlInfo := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable",
		"127.0.0.1", 5432, user, pass, dbname)
	db, err := sql.Open("postgres", psqlInfo)
	if err != nil {
		panic(err)
	}
	err = db.Ping()
	if err != nil {
		panic(fmt.Errorf("failed to connect to database: %w", err))
	}
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(10)
	db.SetConnMaxLifetime(5 * time.Minute)
	return db
}
