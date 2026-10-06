package protocol

import "encoding/json"

// SanitizeStreamForXray strips client-only / panel-only fields from streamSettings
// before writing server config. DB storage keeps the full blob for share links.
func SanitizeStreamForXray(stream map[string]any) map[string]any {
	if stream == nil {
		return map[string]any{}
	}
	// deep-ish copy via JSON to avoid mutating the caller's map
	b, err := json.Marshal(stream)
	if err != nil {
		return stream
	}
	out := map[string]any{}
	if err := json.Unmarshal(b, &out); err != nil {
		return stream
	}

	security := asString(out["security"])
	switch security {
	case "reality":
		rs, _ := out["realitySettings"].(map[string]any)
		if rs != nil {
			out["realitySettings"] = sanitizeRealityServer(rs)
		}
	case "tls":
		ts, _ := out["tlsSettings"].(map[string]any)
		if ts != nil {
			// strip nested client-ish settings block if present
			delete(ts, "settings")
			out["tlsSettings"] = ts
		}
	}

	return out
}

func sanitizeRealityServer(rs map[string]any) map[string]any {
	clean := map[string]any{}
	// Server-side fields recognized by Xray-core inbound REALITY
	keepKeys := []string{
		"show", "dest", "target", "type", "xver",
		"serverNames", "privateKey", "minClientVer", "maxClientVer", "maxTimeDiff",
		"shortIds", "mldsa65Seed",
		"limitFallbackUpload", "limitFallbackDownload",
	}
	for _, k := range keepKeys {
		if v, ok := rs[k]; ok {
			clean[k] = v
		}
	}
	// Prefer target (newer) but keep dest for older cores
	if asString(clean["dest"]) == "" && asString(clean["target"]) != "" {
		clean["dest"] = clean["target"]
	}
	if asString(clean["target"]) == "" && asString(clean["dest"]) != "" {
		clean["target"] = clean["dest"]
	}
	// Ensure shortIds exists (required by REALITY)
	if _, ok := clean["shortIds"]; !ok {
		clean["shortIds"] = []any{""}
	}
	if clean["show"] == nil {
		clean["show"] = false
	}
	if clean["xver"] == nil {
		clean["xver"] = 0
	}
	return clean
}

// SanitizeInboundStreamJSON applies SanitizeStreamForXray to a JSON string.
func SanitizeInboundStreamJSON(streamJSON string) string {
	stream := ParseStream(streamJSON)
	clean := SanitizeStreamForXray(stream)
	b, err := json.Marshal(clean)
	if err != nil {
		return streamJSON
	}
	return string(b)
}
