/*
(c) 2025 Sharon Aicler (saichler@gmail.com)

Layer 8 Ecosystem is licensed under the Apache License, Version 2.0.
*/

// Package prompts provides the AgentPrompt CRUD service.
package prompts

import (
	"database/sql"
	"fmt"
	"time"

	_ "github.com/lib/pq"
	"github.com/saichler/l8agent/go/types/l8agent"
	"github.com/saichler/l8orm/go/orm/persist"
	"github.com/saichler/l8orm/go/orm/plugins/postgres"
	"github.com/saichler/l8types/go/ifs"
	"github.com/saichler/l8types/go/types/l8api"
	"github.com/saichler/l8types/go/types/l8web"
	"github.com/saichler/l8utils/go/utils/web"
)

const (
	ServiceName = "AgntPrmpt"
)

var serviceArea byte

func Activate(creds, dbname string, area byte, vnic ifs.IVNic) error {
	serviceArea = area

	realdb, user, pass, _, err := vnic.Resources().Security().Credential(creds, dbname, vnic.Resources())
	if err != nil {
		return fmt.Errorf("failed to retrieve database credentials: %w", err)
	}
	db, err := openDB(realdb, user, pass)
	if err != nil {
		return err
	}
	p := postgres.NewPostgres(db, vnic.Resources())

	sla := ifs.NewServiceLevelAgreement(&persist.OrmService{}, ServiceName, area, true, newPromptCallback())
	sla.SetServiceItem(&l8agent.L8AgentPrompt{})
	sla.SetServiceItemList(&l8agent.L8AgentPromptList{})
	sla.SetPrimaryKeys("PromptId")
	sla.SetArgs(p, true)
	sla.SetTransactional(true)
	sla.SetReplication(true)
	sla.SetReplicationCount(3)

	ws := web.New(ServiceName, area, 0)
	ws.AddEndpoint(&l8agent.L8AgentPrompt{}, ifs.POST, &l8web.L8Empty{})
	ws.AddEndpoint(&l8agent.L8AgentPromptList{}, ifs.POST, &l8web.L8Empty{})
	ws.AddEndpoint(&l8agent.L8AgentPrompt{}, ifs.PUT, &l8web.L8Empty{})
	ws.AddEndpoint(&l8agent.L8AgentPrompt{}, ifs.PATCH, &l8web.L8Empty{})
	ws.AddEndpoint(&l8api.L8Query{}, ifs.DELETE, &l8web.L8Empty{})
	ws.AddEndpoint(&l8api.L8Query{}, ifs.GET, &l8agent.L8AgentPromptList{})
	sla.SetWebService(ws)

	vnic.Resources().Services().Activate(sla, vnic)
	return nil
}

func openDB(dbname, user, pass string) (*sql.DB, error) {
	psqlInfo := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable",
		"127.0.0.1", 5432, user, pass, dbname)
	db, err := sql.Open("postgres", psqlInfo)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}
	err = db.Ping()
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(10)
	db.SetConnMaxLifetime(5 * time.Minute)
	return db, nil
}
