/*
© 2025 Sharon Aicler (saichler@gmail.com)

Layer 8 Ecosystem is licensed under the Apache License, Version 2.0.
*/
package masking

import (
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
		return fmt.Sprintf("%v", value)

	default:
		return fmt.Sprintf("%v", value)
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
		result = strings.ReplaceAll(result, token, fmt.Sprintf("%v", value))
	}
	if replacements > 0 {
		fmt.Println("[masking] Unmask: replaced", replacements, "tokens out of", len(m.tokens), "total")
	}
	return result
}

// Size returns the number of tokens in the map.
func (m *TokenMap) Size() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.tokens)
}
