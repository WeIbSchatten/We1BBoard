package xray

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/we1bboard/we1bboard/internal/database"
)

// DefaultTemplate returns the 3x-ui-style base Xray config template.
// inbounds/outbounds are filled at GenerateConfig time; routing includes the api rule.
func DefaultTemplate() map[string]any {
	return map[string]any{
		"log": map[string]any{
			"loglevel": "warning",
		},
		"api": map[string]any{
			"tag":      "api",
			"services": []string{"HandlerService", "LoggerService", "StatsService"},
		},
		"stats": map[string]any{},
		"policy": map[string]any{
			"levels": map[string]any{
				"0": map[string]any{
					"statsUserUplink":   true,
					"statsUserDownlink": true,
				},
			},
			"system": map[string]any{
				"statsInboundUplink":    true,
				"statsInboundDownlink":  true,
				"statsOutboundUplink":   true,
				"statsOutboundDownlink": true,
			},
		},
		"routing": map[string]any{
			"domainStrategy": "AsIs",
			"rules": []any{
				map[string]any{
					"type":        "field",
					"inboundTag":  []any{"api"},
					"outboundTag": "api",
				},
			},
		},
	}
}

// LoadTemplate reads xrayTemplate setting; empty/invalid → DefaultTemplate (deep-copied).
func LoadTemplate() map[string]any {
	raw := strings.TrimSpace(database.GetSetting("xrayTemplate"))
	if raw == "" {
		return deepCopyMap(DefaultTemplate())
	}
	var tpl map[string]any
	if err := json.Unmarshal([]byte(raw), &tpl); err != nil || tpl == nil {
		return deepCopyMap(DefaultTemplate())
	}
	return tpl
}

// ValidateTemplateJSON parses and validates a JSON object template.
func ValidateTemplateJSON(raw []byte) (map[string]any, error) {
	raw = []byte(strings.TrimSpace(string(raw)))
	if len(raw) == 0 {
		return nil, fmt.Errorf("template JSON is empty")
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, fmt.Errorf("invalid JSON: %w", err)
	}
	if m == nil {
		return nil, fmt.Errorf("template must be a JSON object")
	}
	return m, nil
}

func deepCopyMap(m map[string]any) map[string]any {
	b, err := json.Marshal(m)
	if err != nil {
		return map[string]any{}
	}
	var out map[string]any
	if err := json.Unmarshal(b, &out); err != nil || out == nil {
		return map[string]any{}
	}
	return out
}

func isAPIRule(rule map[string]any) bool {
	ot, _ := rule["outboundTag"].(string)
	if ot != "api" {
		return false
	}
	for _, s := range asStringSlice(rule["inboundTag"]) {
		if s == "api" {
			return true
		}
	}
	return false
}

func asStringSlice(v any) []string {
	switch t := v.(type) {
	case nil:
		return nil
	case string:
		if t == "" {
			return nil
		}
		return []string{t}
	case []string:
		return t
	case []any:
		out := make([]string, 0, len(t))
		for _, x := range t {
			if s, ok := x.(string); ok && s != "" {
				out = append(out, s)
			}
		}
		return out
	default:
		return nil
	}
}
