/*
© 2025 Sharon Aicler (saichler@gmail.com)

Layer 8 Ecosystem is licensed under the Apache License, Version 2.0.
*/

// Package tools provides the LLM tool definitions for the AI agent.
// These are the tools the LLM can call during a conversation.
package tools

import (
	"github.com/saichler/l8agent/go/llm"
)

// GetToolDefinitions returns the tool definitions for the Claude API.
func GetToolDefinitions() []llm.Tool {
	return []llm.Tool{
		{
			Name: "l8query",
			Description: `Execute an L8Query against a service endpoint.

Syntax: select <columns|aggregates> from <Model> [where <conditions>] [group-by <fields>] [sort-by <field> [descending]] [limit <n>] [page <n>]

Aggregate functions: count(*), sum(field), avg(field), min(field), max(field)
- For totals/sums/averages, ALWAYS use aggregate queries instead of fetching all records.
- Aggregate queries return result maps with auto-generated aliases (e.g., sumTotalAmount, countSalesOrderId).

Examples:
  select * from Employee where departmentId=D001 limit 10
  select count(*) from Employee
  select sum(totalAmount) from SalesOrder
  select avg(salary) from Employee group-by departmentId
  select count(*),sum(totalAmount) from SalesOrder where status=2

Note: Money fields are nested objects with 'amount' (in cents) and 'currencyCode'. Use sum(totalAmount.amount) for monetary sums.`,
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"query": map[string]interface{}{
						"type":        "string",
						"description": "The L8Query string. Use aggregate functions (sum, count, avg, min, max) for totals instead of fetching all records.",
					},
					"area": map[string]interface{}{
						"type":        "integer",
						"description": "The service area number (e.g., 30 for HCM, 40 for FIN)",
					},
					"service": map[string]interface{}{
						"type":        "string",
						"description": "The service name (e.g., 'Employee', 'Dept'). Max 10 characters.",
					},
				},
				"required": []string{"query", "area", "service"},
			},
		},
		{
			Name:        "create_record",
			Description: "Create a new entity by POSTing to a service endpoint. The data fields must match the model's protobuf field names.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"area": map[string]interface{}{
						"type":        "integer",
						"description": "The service area number",
					},
					"service": map[string]interface{}{
						"type":        "string",
						"description": "The service name",
					},
					"data": map[string]interface{}{
						"type":        "object",
						"description": "The entity data as key-value pairs matching protobuf field names",
					},
				},
				"required": []string{"area", "service", "data"},
			},
		},
		{
			Name:        "update_record",
			Description: "Update an existing entity by PUTting to a service endpoint. Must include the primary key field.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"area": map[string]interface{}{
						"type":        "integer",
						"description": "The service area number",
					},
					"service": map[string]interface{}{
						"type":        "string",
						"description": "The service name",
					},
					"data": map[string]interface{}{
						"type":        "object",
						"description": "The entity data with primary key and fields to update",
					},
				},
				"required": []string{"area", "service", "data"},
			},
		},
		{
			Name:        "delete_record",
			Description: "Delete an entity. Requires explicit user confirmation before calling this tool.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"area": map[string]interface{}{
						"type":        "integer",
						"description": "The service area number",
					},
					"service": map[string]interface{}{
						"type":        "string",
						"description": "The service name",
					},
					"query": map[string]interface{}{
						"type":        "string",
						"description": "L8Query identifying the record to delete",
					},
				},
				"required": []string{"area", "service", "query"},
			},
		},
		{
			Name:        "list_modules",
			Description: "List all available modules, services, and their service areas. Use this to discover what data is available.",
			InputSchema: map[string]interface{}{
				"type":       "object",
				"properties": map[string]interface{}{},
			},
		},
		{
			Name:        "describe_model",
			Description: "Get the field definitions for a specific model type. Use this to learn field names before constructing queries.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"model": map[string]interface{}{
						"type":        "string",
						"description": "The protobuf model type name (e.g., 'Employee', 'Department')",
					},
				},
				"required": []string{"model"},
			},
		},
	}
}
