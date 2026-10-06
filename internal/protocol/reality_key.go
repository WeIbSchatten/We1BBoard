package protocol

import (
	"crypto/ecdh"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
)

// RealityPublicKeyFromPrivate derives the X25519 public key (base64.RawURLEncoding)
// from a REALITY private key. Accepts RawURL or Std base64.
func RealityPublicKeyFromPrivate(privateKey string) (string, error) {
	privateKey = strings.TrimSpace(privateKey)
	if privateKey == "" {
		return "", fmt.Errorf("empty private key")
	}
	raw, err := decodeKeyBytes(privateKey)
	if err != nil {
		return "", err
	}
	priv, err := ecdh.X25519().NewPrivateKey(raw)
	if err != nil {
		return "", fmt.Errorf("invalid x25519 private key: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(priv.PublicKey().Bytes()), nil
}

func decodeKeyBytes(s string) ([]byte, error) {
	if b, err := base64.RawURLEncoding.DecodeString(s); err == nil && (len(b) == 32 || len(b) == 16) {
		return b, nil
	}
	if b, err := base64.URLEncoding.DecodeString(s); err == nil && (len(b) == 32 || len(b) == 16) {
		return b, nil
	}
	if b, err := base64.StdEncoding.DecodeString(s); err == nil && (len(b) == 32 || len(b) == 16) {
		return b, nil
	}
	if b, err := base64.RawStdEncoding.DecodeString(s); err == nil && (len(b) == 32 || len(b) == 16) {
		return b, nil
	}
	return nil, fmt.Errorf("cannot decode key")
}

// ResolveRealityPublicKey returns pbk for share links: stored field, nested settings, or derived.
func ResolveRealityPublicKey(rs map[string]any) string {
	if rs == nil {
		return ""
	}
	if pbk := asString(rs["publicKey"]); pbk != "" {
		return pbk
	}
	if settings, ok := rs["settings"].(map[string]any); ok {
		if pbk := asString(settings["publicKey"]); pbk != "" {
			return pbk
		}
	}
	if priv := asString(rs["privateKey"]); priv != "" {
		if pbk, err := RealityPublicKeyFromPrivate(priv); err == nil {
			return pbk
		}
	}
	return ""
}

// EnsureRealityPublicKey fills realitySettings.publicKey (and settings.publicKey)
// from privateKey when missing so share links always have pbk.
func EnsureRealityPublicKey(streamJSON string) string {
	stream := ParseStream(streamJSON)
	if asString(stream["security"]) != "reality" {
		return streamJSON
	}
	rs, _ := stream["realitySettings"].(map[string]any)
	if rs == nil {
		return streamJSON
	}
	pbk := ResolveRealityPublicKey(rs)
	if pbk == "" {
		return streamJSON
	}
	rs["publicKey"] = pbk
	settings, _ := rs["settings"].(map[string]any)
	if settings == nil {
		settings = map[string]any{}
	}
	settings["publicKey"] = pbk
	if asString(settings["fingerprint"]) == "" {
		if fp := asString(rs["fingerprint"]); fp != "" {
			settings["fingerprint"] = fp
		}
	}
	if asString(settings["serverName"]) == "" {
		if sni := firstString(rs["serverNames"]); sni != "" {
			settings["serverName"] = sni
		}
	}
	if asString(settings["spiderX"]) == "" {
		if spx := asString(rs["spiderX"]); spx != "" {
			settings["spiderX"] = spx
		}
	}
	rs["settings"] = settings
	stream["realitySettings"] = rs
	b, err := json.Marshal(stream)
	if err != nil {
		return streamJSON
	}
	return string(b)
}
