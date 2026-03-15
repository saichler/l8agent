/*
© 2025 Sharon Aicler (saichler@gmail.com)

Layer 8 Ecosystem is licensed under the Apache License, Version 2.0.
*/
package masking

import (
	"encoding/json"
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
	var data interface{}
	if err := json.Unmarshal([]byte(jsonStr), &data); err != nil {
		return jsonStr // Return original if not valid JSON
	}

	masked := p.maskValue(data, modelName, tokenMap)

	result, err := json.Marshal(masked)
	if err != nil {
		return jsonStr
	}
	return string(result)
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
