/*
© 2025 Sharon Aicler (saichler@gmail.com)

Layer 8 Ecosystem is licensed under the Apache License, Version 2.0.
*/

// Package masking provides the Data Masking Proxy for protecting sensitive data
// from being sent to the LLM. It classifies fields by sensitivity based on
// protobuf field types and name patterns.
package masking

import (
	"strings"
)

// Classification levels for field sensitivity.
const (
	ClassAlwaysMask = "always"  // SSN, tax IDs, bank accounts — replace with [MASKED]
	ClassMaskName   = "name"    // Names, addresses, emails, phones — replace with [NAME_N]
	ClassMaskMoney  = "money"   // Salaries, amounts, prices — replace with [MONEY_N]
	ClassNeverMask  = "never"   // IDs, enums, statuses, dates, booleans, codes
)

// alwaysMaskPatterns are field name patterns that always get masked.
var alwaysMaskPatterns = []string{
	"ssn", "nationalid", "bankaccount", "routingnumber",
	"taxid", "ein", "socialsecurity",
}

// nameMaskPatterns are field name patterns that get name-masked.
var nameMaskPatterns = []string{
	"firstname", "lastname", "name", "address", "email",
	"phone", "street", "city", "zip", "zipcode",
}

// moneyMaskPatterns are field name patterns that get money-masked.
var moneyMaskPatterns = []string{
	"salary", "amount", "price", "cost", "payment",
	"balance", "revenue", "income", "wage", "compensation",
}

// neverMaskPatterns are field name patterns that never get masked.
var neverMaskPatterns = []string{
	"id", "status", "type", "code", "date", "createdat",
	"updatedat", "bool", "enum", "count", "key",
}

// OverrideFunc allows consumer projects to override field classification.
// Return nil to use the default classification, or a pointer to the desired class.
type OverrideFunc func(modelName string, fieldName string) *string

// Config holds the masking configuration.
type Config struct {
	overrides OverrideFunc
}

// NewConfig creates a new masking configuration.
func NewConfig(overrides OverrideFunc) *Config {
	return &Config{overrides: overrides}
}

// ClassifyField determines the masking classification for a field.
func (c *Config) ClassifyField(modelName, fieldName string) string {
	// Check for consumer overrides first
	if c.overrides != nil {
		if override := c.overrides(modelName, fieldName); override != nil {
			return *override
		}
	}

	lower := strings.ToLower(fieldName)

	// Check always-mask patterns
	for _, pat := range alwaysMaskPatterns {
		if strings.Contains(lower, pat) {
			return ClassAlwaysMask
		}
	}

	// Check money patterns
	for _, pat := range moneyMaskPatterns {
		if strings.Contains(lower, pat) {
			return ClassMaskMoney
		}
	}

	// Check name patterns
	for _, pat := range nameMaskPatterns {
		if strings.Contains(lower, pat) {
			return ClassMaskName
		}
	}

	// Default: never mask (IDs, enums, dates, booleans, etc.)
	return ClassNeverMask
}
