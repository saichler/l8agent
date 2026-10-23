/*
(c) 2025 Sharon Aicler (saichler@gmail.com)

Layer 8 Ecosystem is licensed under the Apache License, Version 2.0.
*/

package schema

import (
	"bytes"
	"testing"

	"github.com/saichler/l8types/go/types/l8reflect"
)

func TestWriteFieldsExpandsNestedStructs(t *testing.T) {
	audit := &l8reflect.L8Node{TypeName: "AuditInfo", IsStruct: true, Attributes: map[string]*l8reflect.L8Node{
		"ModifiedAt": {TypeName: "int64"},
		"CreatedAt":  {TypeName: "int64"},
	}}
	node := &l8reflect.L8Node{TypeName: "Client", Attributes: map[string]*l8reflect.L8Node{
		"Name":      {TypeName: "string"},
		"AuditInfo": audit,
		"Tags":      {TypeName: "string", IsSlice: true},
	}}
	buff := bytes.Buffer{}
	writeFields(&buff, node, "", 0)
	want := "  - AuditInfo (AuditInfo)\n" +
		"  - AuditInfo.CreatedAt (int64)\n" +
		"  - AuditInfo.ModifiedAt (int64)\n" +
		"  - Name (string)\n" +
		"  - Tags (list of string)\n"
	if buff.String() != want {
		t.Errorf("got:\n%s\nwant:\n%s", buff.String(), want)
	}
}
