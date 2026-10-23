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
