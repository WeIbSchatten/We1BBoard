package xray

import (
	"fmt"
	"strings"

	"github.com/we1bboard/we1bboard/internal/protocol"
)

// ValidateStreamSettings checks TLS/REALITY fields that would crash Xray if invalid.
func ValidateStreamSettings(stream map[string]any) error {
	if stream == nil {
		return nil
	}
	security, _ := stream["security"].(string)
	switch strings.ToLower(security) {
	case "reality":
		rs, _ := stream["realitySettings"].(map[string]any)
		if rs == nil {
			return fmt.Errorf("reality: realitySettings is required")
		}
		pk, _ := rs["privateKey"].(string)
		if strings.TrimSpace(pk) == "" {
			return fmt.Errorf("reality: privateKey is required")
		}
		if _, err := protocol.RealityPublicKeyFromPrivate(pk); err != nil {
			return fmt.Errorf("reality: invalid privateKey: %w", err)
		}
		dest, _ := rs["dest"].(string)
		target, _ := rs["target"].(string)
		if strings.TrimSpace(dest) == "" && strings.TrimSpace(target) == "" {
			return fmt.Errorf("reality: dest or target is required")
		}
		if !hasNonEmptyServerNames(rs["serverNames"]) {
			return fmt.Errorf("reality: serverNames must be non-empty")
		}
		if !hasShortIds(rs["shortIds"]) {
			return fmt.Errorf("reality: shortIds is required (use [\"\"] to allow empty client shortId)")
		}
		network, _ := stream["network"].(string)
		if network == "" {
			network = "tcp"
		}
		switch strings.ToLower(network) {
		case "tcp", "raw", "xhttp", "splithttp", "grpc", "gun":
		default:
			return fmt.Errorf("reality: network %q is not supported (use tcp, xhttp, or grpc)", network)
		}
	case "tls":
		ts, _ := stream["tlsSettings"].(map[string]any)
		if ts == nil {
			return fmt.Errorf("tls: tlsSettings is required")
		}
		certs, _ := ts["certificates"].([]any)
		if !hasUsableTLSCert(certs) {
			return fmt.Errorf("tls: at least one certificate with certificateFile or certificate content is required")
		}
	}
	return nil
}

// ValidateInboundStream parses streamSettings JSON and validates TLS/REALITY.
func ValidateInboundStream(streamJSON string) error {
	return ValidateStreamSettings(protocol.ParseStream(streamJSON))
}

func hasNonEmptyServerNames(v any) bool {
	switch names := v.(type) {
	case []any:
		for _, n := range names {
			if strings.TrimSpace(fmt.Sprint(n)) != "" {
				return true
			}
		}
	case []string:
		for _, n := range names {
			if strings.TrimSpace(n) != "" {
				return true
			}
		}
	}
	return false
}

func hasShortIds(v any) bool {
	switch ids := v.(type) {
	case []any:
		return len(ids) > 0
	case []string:
		return len(ids) > 0
	case string:
		return true // single value present
	}
	return false
}

func hasUsableTLSCert(certs []any) bool {
	for _, c := range certs {
		m, ok := c.(map[string]any)
		if !ok {
			continue
		}
		if file, _ := m["certificateFile"].(string); strings.TrimSpace(file) != "" {
			return true
		}
		if hasNonEmptyStringList(m["certificate"]) {
			return true
		}
	}
	return false
}

func hasNonEmptyStringList(v any) bool {
	switch list := v.(type) {
	case []any:
		for _, item := range list {
			if strings.TrimSpace(fmt.Sprint(item)) != "" {
				return true
			}
		}
	case []string:
		for _, item := range list {
			if strings.TrimSpace(item) != "" {
				return true
			}
		}
	case string:
		return strings.TrimSpace(list) != ""
	}
	return false
}
