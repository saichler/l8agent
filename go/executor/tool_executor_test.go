/*
© 2025 Sharon Aicler (saichler@gmail.com)

Layer 8 Ecosystem is licensed under the Apache License, Version 2.0.
*/

package executor

import "testing"

func TestCheckQueryPolicy(t *testing.T) {
	rejected := []string{
		"select * from Employee",
		"select * from Employee where status=1",
		"select * from Employee limit 500",
	}
	for _, q := range rejected {
		if checkQueryPolicy(q) == nil {
			t.Errorf("expected %q to be rejected", q)
		}
	}
	allowed := []string{
		"select * from Employee limit 10",
		"select * from Employee sort-by hireDate descending LIMIT 100",
		"select * from Employee where employeeId=E1",
		"select firstName,lastName from Employee",
		"select count(*) from Employee",
	}
	for _, q := range allowed {
		if err := checkQueryPolicy(q); err != nil {
			t.Errorf("expected %q to be allowed: %v", q, err)
		}
	}
}

func TestResolveService(t *testing.T) {
	catalog := map[string]map[int32]string{
		"PhyClient": {50: "PhysioClient"},
		"PhyAppt":   {50: "Appointment"},
		"AgntMsg":   {55: "L8AgentChatMessage"},
	}
	cases := []struct {
		service, model string
		area           int32
		wantSvc        string
		wantArea       int32
	}{
		{"PhyClient", "PhysioClient", 50, "PhyClient", 50},  // valid as given
		{"PhysioClie", "PhysioClient", 50, "PhyClient", 50}, // truncated model name as service
		{"PhyClient", "PhysioClient", 0, "PhyClient", 50},   // wrong area
		{"PhyAppt", "PhysioClient", 50, "PhyClient", 50},    // from clause wins over a wrong service
		{"PhysioClient", "", 50, "PhyClient", 50},           // model name passed as service, no query
		{"AgntMsg", "", 50, "AgntMsg", 55},                  // only one area for the service
	}
	for _, c := range cases {
		svc, area, err := resolveService(catalog, c.service, c.area, c.model)
		if err != nil || svc != c.wantSvc || area != c.wantArea {
			t.Errorf("resolveService(%q, %d, %q) = %q, %d, %v; want %q, %d", c.service, c.area, c.model, svc, area, err, c.wantSvc, c.wantArea)
		}
	}
	if _, _, err := resolveService(catalog, "Nope", 50, "Nothing"); err == nil {
		t.Error("expected an error for an unknown service and model")
	}
	if got := queryModel("select count(*) FROM PhysioClient where status=1"); got != "PhysioClient" {
		t.Errorf("queryModel = %q", got)
	}
}
