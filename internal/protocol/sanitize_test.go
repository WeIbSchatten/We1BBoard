package protocol

import (
	"encoding/json"
	"testing"
)

func TestSanitizeStreamForXrayReality(t *testing.T) {
	raw := map[string]any{
		"network":  "tcp",
		"security": "reality",
		"realitySettings": map[string]any{
			"show":        false,
			"dest":        "www.cloudflare.com:443",
			"target":      "www.cloudflare.com:443",
			"serverNames": []any{"www.cloudflare.com"},
			"privateKey":  "priv",
			"shortIds":    []any{"abcd"},
			"publicKey":   "SHOULD_STRIP",
			"fingerprint": "chrome",
			"spiderX":     "/",
			"settings": map[string]any{
				"publicKey":   "SHOULD_STRIP",
				"fingerprint": "chrome",
			},
		},
	}
	clean := SanitizeStreamForXray(raw)
	rs := clean["realitySettings"].(map[string]any)
	if _, ok := rs["publicKey"]; ok {
		t.Fatal("publicKey must be stripped")
	}
	if _, ok := rs["settings"]; ok {
		t.Fatal("settings must be stripped")
	}
	if _, ok := rs["fingerprint"]; ok {
		t.Fatal("fingerprint must be stripped")
	}
	if rs["privateKey"] != "priv" {
		t.Fatal("privateKey kept")
	}
	if rs["dest"] != "www.cloudflare.com:443" {
		t.Fatal("dest kept")
	}
	// original untouched
	orig := raw["realitySettings"].(map[string]any)
	if orig["publicKey"] != "SHOULD_STRIP" {
		t.Fatal("original mutated")
	}
}

func TestSanitizeInboundStreamJSON(t *testing.T) {
	in := `{"network":"tcp","security":"reality","realitySettings":{"privateKey":"x","dest":"a:443","serverNames":["a"],"shortIds":[""],"publicKey":"y","settings":{"publicKey":"y"}}}`
	out := SanitizeInboundStreamJSON(in)
	var m map[string]any
	if err := json.Unmarshal([]byte(out), &m); err != nil {
		t.Fatal(err)
	}
	rs := m["realitySettings"].(map[string]any)
	if _, ok := rs["publicKey"]; ok {
		t.Fatal(out)
	}
}
