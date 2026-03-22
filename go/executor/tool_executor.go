/*
© 2025 Sharon Aicler (saichler@gmail.com)

Layer 8 Ecosystem is licensed under the Apache License, Version 2.0.
*/

// Package executor provides the Tool Executor that runs LLM tool calls
// against Layer 8 service endpoints using internal vnic requests.
package executor

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/saichler/l8agent/go/schema"
	"github.com/saichler/l8types/go/ifs"
	"github.com/saichler/l8types/go/types/l8api"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

const (
	requestTimeout = 30 // seconds
)

// ToolExecutor executes tool calls against Layer 8 service endpoints via the vnic.
type ToolExecutor struct {
	vnic   ifs.IVNic
	schema *schema.Provider
}

// NewToolExecutor creates a new ToolExecutor.
func NewToolExecutor(vnic ifs.IVNic, schema *schema.Provider) *ToolExecutor {
	return &ToolExecutor{
		vnic:   vnic,
		schema: schema,
	}
}

// Execute runs a tool call and returns the result as a JSON string.
func (t *ToolExecutor) Execute(toolName string, input string, bearerToken string) (string, error) {
	var inputMap map[string]interface{}
	if err := json.Unmarshal([]byte(input), &inputMap); err != nil {
		return "", errors.New("invalid tool input: " + err.Error())
	}

	switch toolName {
	case "l8query":
		return t.executeQuery(inputMap)
	case "create_record":
		return t.executeMutate(inputMap, ifs.POST)
	case "update_record":
		return t.executeMutate(inputMap, ifs.PUT)
	case "delete_record":
		return t.executeDelete(inputMap)
	case "list_modules":
		return t.schema.GetTier1Schema(), nil
	case "describe_model":
		model, _ := inputMap["model"].(string)
		return t.schema.DescribeModel(model), nil
	default:
		return "", errors.New("unknown tool: " + toolName)
	}
}

// aggregateKeywords are the L8Query aggregate function prefixes.
var aggregateKeywords = []string{"sum(", "count(", "avg(", "min(", "max("}

// hasAggregate checks if an L8Query uses aggregate functions.
func hasAggregate(query string) bool {
	lower := strings.ToLower(query)
	for _, kw := range aggregateKeywords {
		if strings.Contains(lower, kw) {
			return true
		}
	}
	return false
}

// executeQuery sends an L8Query GET request to a service via the vnic.
// It enforces the use of aggregate functions for queries that fetch all records
// without a specific ID filter — the LLM must use sum/count/avg/min/max instead
// of fetching raw data and computing totals itself.
func (t *ToolExecutor) executeQuery(input map[string]interface{}) (string, error) {
	query, _ := input["query"].(string)
	area, _ := input["area"].(float64)
	service, _ := input["service"].(string)

	if query == "" || service == "" {
		return "", errors.New("l8query requires 'query', 'area', and 'service'")
	}

	// Enforce aggregate usage: reject select * without a specific ID filter.
	// The LLM must use sum(), count(), avg(), etc. for totals and summaries.
	if !hasAggregate(query) {
		lower := strings.ToLower(query)
		if strings.Contains(lower, "select *") && !strings.Contains(lower, "id=") {
			return "", fmt.Errorf("rejected: use aggregate functions (sum, count, avg, min, max) instead of 'select *' for summaries. Example: select sum(totalAmount.amount) from SalesOrder")
		}
	}

	elems := t.vnic.LeaderRequest(service, byte(area), ifs.GET, query, requestTimeout)
	if elems.Error() != nil {
		return "", elems.Error()
	}

	return t.marshalResponse(elems)
}

// executeMutate sends a POST or PUT request with JSON data to a service via the vnic.
func (t *ToolExecutor) executeMutate(input map[string]interface{}, action ifs.Action) (string, error) {
	area, _ := input["area"].(float64)
	service, _ := input["service"].(string)
	data, _ := input["data"].(map[string]interface{})

	if service == "" || data == nil {
		return "", fmt.Errorf("%s requires 'area', 'service', and 'data'", actionName(action))
	}

	// Resolve the protobuf type from the service catalog
	modelName := t.resolveModelName(service, int32(area))
	if modelName == "" {
		return "", fmt.Errorf("cannot resolve model type for service %s area %d", service, int(area))
	}

	// Create a new protobuf instance and unmarshal the JSON data into it
	pbMsg, err := t.unmarshalToProto(modelName, data)
	if err != nil {
		return "", fmt.Errorf("failed to unmarshal data for %s: %s", modelName, err.Error())
	}

	elems := t.vnic.LeaderRequest(service, byte(area), action, pbMsg, requestTimeout)
	if elems.Error() != nil {
		return "", elems.Error()
	}

	return t.marshalResponse(elems)
}

// executeDelete sends a DELETE request with an L8Query to a service via the vnic.
func (t *ToolExecutor) executeDelete(input map[string]interface{}) (string, error) {
	area, _ := input["area"].(float64)
	service, _ := input["service"].(string)
	query, _ := input["query"].(string)

	if service == "" || query == "" {
		return "", errors.New("delete_record requires 'area', 'service', and 'query'")
	}

	l8query := &l8api.L8Query{Text: query}
	elems := t.vnic.LeaderRequest(service, byte(area), ifs.DELETE, l8query, requestTimeout)
	if elems.Error() != nil {
		return "", elems.Error()
	}

	return t.marshalResponse(elems)
}

// resolveModelName looks up the protobuf type name for a service name and area
// from the local service registry.
func (t *ToolExecutor) resolveModelName(serviceName string, serviceArea int32) string {
	services := t.vnic.Resources().SysConfig().Services
	if services == nil || services.ServiceToAreas == nil {
		return ""
	}
	svcAreas, ok := services.ServiceToAreas[serviceName]
	if !ok || svcAreas.Models == nil {
		return ""
	}
	return svcAreas.Models[serviceArea]
}

// unmarshalToProto creates a new protobuf instance of the given type and
// unmarshals JSON data into it.
func (t *ToolExecutor) unmarshalToProto(modelName string, data map[string]interface{}) (proto.Message, error) {
	info, err := t.vnic.Resources().Registry().Info(modelName)
	if err != nil {
		return nil, fmt.Errorf("unknown model type %s: %s", modelName, err.Error())
	}
	instance, err := info.NewInstance()
	if err != nil {
		return nil, fmt.Errorf("failed to create instance of %s: %s", modelName, err.Error())
	}
	pbMsg, ok := instance.(proto.Message)
	if !ok {
		return nil, fmt.Errorf("type %s is not a proto.Message", modelName)
	}

	jsonBytes, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}
	if err := protojson.Unmarshal(jsonBytes, pbMsg); err != nil {
		return nil, err
	}
	return pbMsg, nil
}

// marshalResponse serializes the response elements to JSON.
func (t *ToolExecutor) marshalResponse(elems ifs.IElements) (string, error) {
	if elems.Element() == nil {
		// Check for aggregate results in metadata
		md := elems.Metadata()
		if md != nil && md.KeyCount != nil && len(md.KeyCount.Counts) > 0 {
			return formatAggregateCounts(md.KeyCount.Counts), nil
		}
		return "{}", nil
	}

	// Try to convert to a list wrapper first
	listProto, err := elems.AsList(t.vnic.Resources().Registry())
	if err == nil && listProto != nil {
		if msg, ok := listProto.(proto.Message); ok {
			opts := protojson.MarshalOptions{UseEnumNumbers: true}
			j, e := opts.Marshal(msg)
			if e != nil {
				return "", e
			}
			return string(j), nil
		}
	}

	// Fall back to marshaling the single element
	if msg, ok := elems.Element().(proto.Message); ok {
		opts := protojson.MarshalOptions{UseEnumNumbers: true}
		j, e := opts.Marshal(msg)
		if e != nil {
			return "", e
		}
		return string(j), nil
	}

	return "{}", nil
}

// formatAggregateCounts formats aggregate results as JSON without scientific notation.
func formatAggregateCounts(counts map[string]float64) string {
	parts := make([]string, 0, len(counts))
	for k, v := range counts {
		parts = append(parts, fmt.Sprintf("%q:%.2f", k, v))
	}
	return "{" + strings.Join(parts, ",") + "}"
}

// actionName returns a human-readable name for an action.
func actionName(action ifs.Action) string {
	switch action {
	case ifs.POST:
		return "create_record"
	case ifs.PUT:
		return "update_record"
	default:
		return "unknown_action"
	}
}
