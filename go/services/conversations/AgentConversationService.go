/*
(c) 2025 Sharon Aicler (saichler@gmail.com)

Layer 8 Ecosystem is licensed under the Apache License, Version 2.0.
*/

// Package conversations provides the AgentConversation CRUD service.
package conversations

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
	ServiceName = "AgntConvo"
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

	sla := ifs.NewServiceLevelAgreement(&persist.OrmService{}, ServiceName, area, true, newConversationCallback())
	sla.SetServiceItem(&l8agent.L8AgentConversation{})
	sla.SetServiceItemList(&l8agent.L8AgentConversationList{})
	sla.SetPrimaryKeys("ConversationId")
	sla.SetArgs(p, true)
	sla.SetTransactional(true)
	sla.SetReplication(true)
	sla.SetReplicationCount(3)

	ws := web.New(ServiceName, area, 0)
	ws.AddEndpoint(&l8agent.L8AgentConversation{}, ifs.POST, &l8web.L8Empty{})
	ws.AddEndpoint(&l8agent.L8AgentConversationList{}, ifs.POST, &l8web.L8Empty{})
	ws.AddEndpoint(&l8agent.L8AgentConversation{}, ifs.PUT, &l8web.L8Empty{})
	ws.AddEndpoint(&l8agent.L8AgentConversation{}, ifs.PATCH, &l8web.L8Empty{})
	ws.AddEndpoint(&l8api.L8Query{}, ifs.DELETE, &l8web.L8Empty{})
	ws.AddEndpoint(&l8api.L8Query{}, ifs.GET, &l8agent.L8AgentConversationList{})
	sla.SetWebService(ws)

	vnic.Resources().Services().Activate(sla, vnic)
}

// Conversation retrieves a single conversation by ID.
func Conversation(conversationId string, vnic ifs.IVNic) (*l8agent.L8AgentConversation, error) {
	filter := &l8agent.L8AgentConversation{ConversationId: conversationId}
	handler, ok := vnic.Resources().Services().ServiceHandler(ServiceName, serviceArea)
	if !ok {
		resp := vnic.Request("", ServiceName, serviceArea, ifs.GET, filter, 30)
		if resp.Error() != nil {
			return nil, resp.Error()
		}
		if resp.Element() != nil {
			return resp.Element().(*l8agent.L8AgentConversation), nil
		}
		return nil, nil
	}
	resp := handler.Get(object.New(nil, filter), vnic)
	if resp.Error() != nil {
		return nil, resp.Error()
	}
	if resp.Element() != nil {
		return resp.Element().(*l8agent.L8AgentConversation), nil
	}
	return nil, nil
}

// SaveConversation creates or updates a conversation.
func SaveConversation(convo *l8agent.L8AgentConversation, isNew bool, vnic ifs.IVNic) error {
	handler, ok := vnic.Resources().Services().ServiceHandler(ServiceName, serviceArea)
	if !ok {
		var action ifs.Action
		if isNew {
			action = ifs.POST
		} else {
			action = ifs.PUT
		}
		resp := vnic.Request("", ServiceName, serviceArea, action, convo, 30)
		return resp.Error()
	}
	if isNew {
		resp := handler.Post(object.New(nil, convo), vnic)
		return resp.Error()
	}
	resp := handler.Put(object.New(nil, convo), vnic)
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
