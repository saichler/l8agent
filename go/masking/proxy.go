/*
© 2025 Sharon Aicler (saichler@gmail.com)

Layer 8 Ecosystem is licensed under the Apache License, Version 2.0.
*/
package masking

import (
	"encoding/json"
	"fmt"
	"regexp"
)

// Proxy is the Data Masking Proxy that masks/unmasks sensitive data.
type Proxy struct {
	config *Config
}

// NewProxy creates a new Data Masking Proxy with the given configuration.
func NewProxy(config *Config) *Proxy {
	return &Proxy{config: config}
}

// MaskJSON takes a JSON string (tool result) and returns a masked version.
// The modelName is used for field classification.
func (p *Proxy) MaskJSON(jsonStr string, modelName string, tokenMap *TokenMap) string {
	fmt.Println("[masking] MaskJSON: model:", modelName, "input length:", len(jsonStr))
	var data interface{}
	if err := json.Unmarshal([]byte(jsonStr), &data); err != nil {
		fmt.Println("[masking] MaskJSON: invalid JSON, returning original")
		return jsonStr // Return original if not valid JSON
	}

	sizeBefore := tokenMap.Size()
	masked := p.maskValue(data, modelName, tokenMap)
	sizeAfter := tokenMap.Size()

	result, err := json.Marshal(masked)
	if err != nil {
		fmt.Println("[masking] MaskJSON: marshal error:", err)
		return jsonStr
	}
	if sizeAfter > sizeBefore {
		fmt.Println("[masking] MaskJSON: created", sizeAfter-sizeBefore, "new tokens (total:", sizeAfter, ")")
	}
	return string(result)
}

// Compiled regexes for free-text pattern detection.
var (
	ssnRegex   = regexp.MustCompile(`\b\d{3}-\d{2}-\d{4}\b`)
	moneyRegex = regexp.MustCompile(`\$[\d,]+(?:\.\d{2})?`)
	emailRegex = regexp.MustCompile(`\b[A-Za-z0-9._%+\-]+@[A-Za-z0-9.\-]+\.[A-Za-z]{2,}\b`)
	phoneRegex = regexp.MustCompile(`\(?\d{3}\)?[\s\-]?\d{3}[\s\-]?\d{4}`)
)

// MaskText masks obvious sensitive patterns in free-form text (user prompts).
// Detects SSNs, money amounts, emails, and phone numbers using regex.
func (p *Proxy) MaskText(text string, tokenMap *TokenMap) string {
	sizeBefore := tokenMap.Size()
	// SSN patterns → always mask
	text = ssnRegex.ReplaceAllStringFunc(text, func(match string) string {
		fmt.Println("[masking] MaskText: detected SSN pattern")
		return tokenMap.Mask("ssn", match, ClassAlwaysMask)
	})
	// Money patterns → money mask
	text = moneyRegex.ReplaceAllStringFunc(text, func(match string) string {
		fmt.Println("[masking] MaskText: detected money pattern:", match)
		return tokenMap.Mask("amount", match, ClassMaskMoney)
	})
	// Email patterns → name mask
	text = emailRegex.ReplaceAllStringFunc(text, func(match string) string {
		fmt.Println("[masking] MaskText: detected email pattern")
		return tokenMap.Mask("email", match, ClassMaskName)
	})
	// Phone patterns → name mask
	text = phoneRegex.ReplaceAllStringFunc(text, func(match string) string {
		fmt.Println("[masking] MaskText: detected phone pattern")
		return tokenMap.Mask("phone", match, ClassMaskName)
	})
	sizeAfter := tokenMap.Size()
	if sizeAfter > sizeBefore {
		fmt.Println("[masking] MaskText: created", sizeAfter-sizeBefore, "new tokens (total:", sizeAfter, ")")
	}
	return text
}

// maskValue recursively masks values in a JSON structure.
func (p *Proxy) maskValue(data interface{}, modelName string, tokenMap *TokenMap) interface{} {
	switch v := data.(type) {
	case map[string]interface{}:
		result := make(map[string]interface{})
		for key, val := range v {
			classification := p.config.ClassifyField(modelName, key)
			if classification == ClassNeverMask {
				// Recurse into nested objects but don't mask this field
				result[key] = p.maskValue(val, modelName, tokenMap)
			} else {
				// Mask the value
				result[key] = tokenMap.Mask(key, val, classification)
			}
		}
		return result

	case []interface{}:
		result := make([]interface{}, len(v))
		for i, item := range v {
			result[i] = p.maskValue(item, modelName, tokenMap)
		}
		return result

	default:
		return v
	}
}
