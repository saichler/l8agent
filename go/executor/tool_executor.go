/*
© 2025 Sharon Aicler (saichler@gmail.com)

Layer 8 Ecosystem is licensed under the Apache License, Version 2.0.
*/

// Package executor provides the Tool Executor that runs LLM tool calls
// against Layer 8 service endpoints using internal HTTP requests.
package executor

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/saichler/l8agent/go/schema"
	"github.com/saichler/l8types/go/ifs"
)

const (
	toolCallTimeout = 10 * time.Second
)

// ToolExecutor executes tool calls against Layer 8 service endpoints.
type ToolExecutor struct {
	prefix    string // e.g., "/erp/"
	resources ifs.IResources
	schema    *schema.Provider
	webPort   int
	client    *http.Client
}

// NewToolExecutor creates a new ToolExecutor.
func NewToolExecutor(prefix string, resources ifs.IResources, schema *schema.Provider, webPort int) *ToolExecutor {
	return &ToolExecutor{
		prefix:    prefix,
		resources: resources,
		schema:    schema,
		webPort:   webPort,
		client:    &http.Client{Timeout: toolCallTimeout},
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
		return t.executeQuery(inputMap, bearerToken)
	case "create_record":
		return t.executeCreate(inputMap, bearerToken)
	case "update_record":
		return t.executeUpdate(inputMap, bearerToken)
	case "delete_record":
		return t.executeDelete(inputMap, bearerToken)
	case "list_modules":
		return t.schema.GetTier1Schema(), nil
	case "describe_model":
		model, _ := inputMap["model"].(string)
		return t.schema.DescribeModel(model), nil
	default:
		return "", errors.New("unknown tool: " + toolName)
	}
}

// executeQuery runs an L8Query GET request against a service.
func (t *ToolExecutor) executeQuery(input map[string]interface{}, bearerToken string) (string, error) {
	query, _ := input["query"].(string)
	area, _ := input["area"].(float64)
	service, _ := input["service"].(string)

	if query == "" || service == "" {
		return "", errors.New("l8query requires 'query', 'area', and 'service'")
	}

	body := url.QueryEscape(fmt.Sprintf(`{"text":"%s"}`, query))
	endpoint := fmt.Sprintf("%s%d/%s?body=%s", t.prefix, int(area), service, body)

	return t.doRequest("GET", endpoint, nil, bearerToken)
}

// executeCreate runs a POST request to create a record.
func (t *ToolExecutor) executeCreate(input map[string]interface{}, bearerToken string) (string, error) {
	area, _ := input["area"].(float64)
	service, _ := input["service"].(string)
	data, _ := input["data"].(map[string]interface{})

	if service == "" || data == nil {
		return "", errors.New("create_record requires 'area', 'service', and 'data'")
	}

	endpoint := fmt.Sprintf("%s%d/%s", t.prefix, int(area), service)
	jsonData, _ := json.Marshal(data)

	return t.doRequest("POST", endpoint, jsonData, bearerToken)
}

// executeUpdate runs a PUT request to update a record.
func (t *ToolExecutor) executeUpdate(input map[string]interface{}, bearerToken string) (string, error) {
	area, _ := input["area"].(float64)
	service, _ := input["service"].(string)
	data, _ := input["data"].(map[string]interface{})

	if service == "" || data == nil {
		return "", errors.New("update_record requires 'area', 'service', and 'data'")
	}

	endpoint := fmt.Sprintf("%s%d/%s", t.prefix, int(area), service)
	jsonData, _ := json.Marshal(data)

	return t.doRequest("PUT", endpoint, jsonData, bearerToken)
}

// executeDelete runs a DELETE request.
func (t *ToolExecutor) executeDelete(input map[string]interface{}, bearerToken string) (string, error) {
	area, _ := input["area"].(float64)
	service, _ := input["service"].(string)
	query, _ := input["query"].(string)

	if service == "" || query == "" {
		return "", errors.New("delete_record requires 'area', 'service', and 'query'")
	}

	endpoint := fmt.Sprintf("%s%d/%s", t.prefix, int(area), service)
	jsonData, _ := json.Marshal(map[string]string{"text": query})

	return t.doRequest("DELETE", endpoint, jsonData, bearerToken)
}

// doRequest executes an HTTP request against the local web service.
func (t *ToolExecutor) doRequest(method, endpoint string, body []byte, bearerToken string) (string, error) {
	fullURL := fmt.Sprintf("http://127.0.0.1:%d%s", t.webPort, endpoint)

	var bodyReader io.Reader
	if body != nil {
		bodyReader = strings.NewReader(string(body))
	}

	req, err := http.NewRequest(method, fullURL, bodyReader)
	if err != nil {
		return "", err
	}

	req.Header.Set("Content-Type", "application/json")
	if bearerToken != "" {
		req.Header.Set("Authorization", "Bearer "+bearerToken)
	}

	resp, err := t.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("service error (%d): %s", resp.StatusCode, string(respBody))
	}

	return string(respBody), nil
}
