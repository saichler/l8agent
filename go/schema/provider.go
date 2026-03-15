/*
(c) 2025 Sharon Aicler (saichler@gmail.com)

Layer 8 Ecosystem is licensed under the Apache License, Version 2.0.
*/

// Package schema provides the Schema Provider that generates compressed
// schema metadata for the LLM system prompt from the project's introspector.
package schema

import (
	"bytes"
	"github.com/saichler/l8types/go/ifs"
	"github.com/saichler/l8types/go/types/l8reflect"
)

// Provider generates schema metadata from the project's introspector.
type Provider struct {
	tier1Cache string
	resources  ifs.IResources
}

// NewProvider creates a new Schema Provider and caches Tier 1 metadata.
func NewProvider(resources ifs.IResources) *Provider {
	p := &Provider{resources: resources}
	p.tier1Cache = p.buildTier1()
	return p
}

// GetTier1Schema returns the cached Tier 1 schema (module list, service names, model names).
func (p *Provider) GetTier1Schema() string {
	return p.tier1Cache
}

// DescribeModel returns the full field definitions for a specific model (Tier 2).
func (p *Provider) DescribeModel(modelName string) string {
	node, ok := p.resources.Introspector().Node(modelName)
	if !ok {
		return "Model '" + modelName + "' not found."
	}

	buff := bytes.Buffer{}
	buff.WriteString("Model: ")
	buff.WriteString(node.TypeName)
	buff.WriteString("\nFields:\n")

	for name, attr := range node.Attributes {
		buff.WriteString("  - ")
		buff.WriteString(name)
		buff.WriteString(" (")
		buff.WriteString(attr.TypeName)
		buff.WriteString(")\n")
	}

	return buff.String()
}

// buildTier1 builds the compact Tier 1 schema from the introspector.
func (p *Provider) buildTier1() string {
	buff := bytes.Buffer{}
	buff.WriteString("Available models and services:\n")

	// Nodes(rootsOnly, structsOnly) - get all root struct nodes
	nodes := p.resources.Introspector().Nodes(true, true)
	for _, node := range nodes {
		if node == nil {
			continue
		}
		buff.WriteString("- ")
		buff.WriteString(node.TypeName)
		pk := p.primaryKeyOf(node)
		if pk != "" {
			buff.WriteString(" (pk: ")
			buff.WriteString(pk)
			buff.WriteString(")")
		}
		buff.WriteString("\n")
	}

	return buff.String()
}

// primaryKeyOf extracts the primary key field name for a node using decorators.
func (p *Provider) primaryKeyOf(node *l8reflect.L8Node) string {
	fields, err := p.resources.Introspector().Decorators().Fields(node, l8reflect.L8DecoratorType_Primary)
	if err != nil || len(fields) == 0 {
		return ""
	}
	return fields[0]
}

// RefreshCache regenerates the Tier 1 cache.
func (p *Provider) RefreshCache() {
	p.tier1Cache = p.buildTier1()
}
