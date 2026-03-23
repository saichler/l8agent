/*
© 2025 Sharon Aicler (saichler@gmail.com)

Layer 8 Ecosystem is licensed under the Apache License, Version 2.0.
*/
package masking

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"
)

// TokenMap maintains a per-request mapping between opaque tokens and real values.
// It lives only in server memory for the duration of one chat request.
type TokenMap struct {
	mu       sync.Mutex
	tokens   map[string]interface{} // "[NAME_1]" → "John"
	counters map[string]int         // "NAME" → 4 (next index)
}

// NewTokenMap creates a new empty token map.
func NewTokenMap() *TokenMap {
	return &TokenMap{
		tokens:   make(map[string]interface{}),
		counters: make(map[string]int),
	}
}

// Mask replaces a field value with a token based on its classification.
// Returns the token string. For ClassNeverMask, returns the original value as string.
// For ClassAlwaysMask, returns "[MASKED]".
func (m *TokenMap) Mask(fieldName string, value interface{}, classification string) string {
	if value == nil {
		return ""
	}

	switch classification {
	case ClassAlwaysMask:
		return "[MASKED]"

	case ClassMaskName:
		return m.createToken("NAME", value)

	case ClassMaskMoney:
		return m.createToken("MONEY", value)

	case ClassNeverMask:
		return valueToString(value)

	default:
		return valueToString(value)
	}
}

// createToken generates a new indexed token and stores the real value.
func (m *TokenMap) createToken(prefix string, value interface{}) string {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.counters[prefix]++
	token := fmt.Sprintf("[%s_%d]", prefix, m.counters[prefix])
	m.tokens[token] = value
	return token
}

// Unmask replaces all tokens in the text with their real values.
func (m *TokenMap) Unmask(text string) string {
	m.mu.Lock()
	defer m.mu.Unlock()

	replacements := 0
	result := text
	for token, value := range m.tokens {
		if strings.Contains(result, token) {
			replacements++
		}
		var display string
		if strings.HasPrefix(token, "[MONEY_") {
			display = formatMoney(value)
		} else {
			display = valueToString(value)
		}
		result = strings.ReplaceAll(result, token, display)
	}
	if replacements > 0 {
		fmt.Println("[masking] Unmask: replaced", replacements, "tokens out of", len(m.tokens), "total")
	}
	return result
}

// formatMoney formats a numeric value as a dollar amount with commas (e.g., "$60,011,642.00").
// TODO: support currency codes from the actual data instead of hardcoding "$".
func formatMoney(value interface{}) string {
	var cents float64
	switch v := value.(type) {
	case float64:
		cents = v
	case float32:
		cents = float64(v)
	case int64:
		cents = float64(v)
	case int:
		cents = float64(v)
	default:
		return fmt.Sprintf("%v", value)
	}
	dollars := cents / 100.0
	negative := dollars < 0
	if negative {
		dollars = -dollars
	}
	// Format with 2 decimal places then insert commas
	raw := fmt.Sprintf("%.2f", dollars)
	parts := strings.SplitN(raw, ".", 2)
	intPart := parts[0]
	// Insert commas from right to left
	n := len(intPart)
	var buf strings.Builder
	for i, ch := range intPart {
		if i > 0 && (n-i)%3 == 0 {
			buf.WriteByte(',')
		}
		buf.WriteRune(ch)
	}
	if negative {
		return "-$" + buf.String() + "." + parts[1]
	}
	return "$" + buf.String() + "." + parts[1]
}

// valueToString converts a value to its string representation.
// For complex types (maps, slices), uses JSON marshaling to produce valid JSON
// instead of Go's fmt.Sprintf which produces map[key:value] format.
func valueToString(value interface{}) string {
	switch v := value.(type) {
	case map[string]interface{}, []interface{}:
		j, err := json.Marshal(value)
		if err == nil {
			return string(j)
		}
	case float64:
		// Use fixed-point notation to avoid scientific notation (e.g., 6.0011642e+07)
		return fmt.Sprintf("%.2f", v)
	case float32:
		return fmt.Sprintf("%.2f", v)
	}
	return fmt.Sprintf("%v", value)
}

// Size returns the number of tokens in the map.
func (m *TokenMap) Size() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.tokens)
}
