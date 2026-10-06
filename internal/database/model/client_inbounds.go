package model

import (
	"strconv"
	"strings"
)

// ParseInboundIDList returns unique inbound IDs for a client (primary + inboundIds csv).
func ParseInboundIDList(c *Client) []uint {
	if c == nil {
		return nil
	}
	seen := map[uint]bool{}
	out := make([]uint, 0, 4)
	add := func(id uint) {
		if id == 0 || seen[id] {
			return
		}
		seen[id] = true
		out = append(out, id)
	}
	add(c.InboundID)
	for _, part := range strings.Split(c.InboundIDs, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		n, err := strconv.ParseUint(part, 10, 64)
		if err != nil || n == 0 {
			continue
		}
		add(uint(n))
	}
	return out
}

// ClientBelongsToInbound reports whether the client is attached to inbound id.
func ClientBelongsToInbound(c *Client, id uint) bool {
	if c == nil || id == 0 {
		return false
	}
	if c.InboundID == id {
		return true
	}
	needle := strconv.FormatUint(uint64(id), 10)
	for _, part := range strings.Split(c.InboundIDs, ",") {
		if strings.TrimSpace(part) == needle {
			return true
		}
	}
	return false
}

// NormalizeInboundIDs sets InboundID to the first id and InboundIDs to a deduped csv.
// If ids is empty, keeps primary (or leaves zero).
func NormalizeInboundIDs(c *Client, ids []uint) {
	if c == nil {
		return
	}
	seen := map[uint]bool{}
	clean := make([]uint, 0, len(ids))
	for _, id := range ids {
		if id == 0 || seen[id] {
			continue
		}
		seen[id] = true
		clean = append(clean, id)
	}
	if len(clean) == 0 {
		if c.InboundID != 0 {
			c.InboundIDs = strconv.FormatUint(uint64(c.InboundID), 10)
		}
		return
	}
	c.InboundID = clean[0]
	parts := make([]string, len(clean))
	for i, id := range clean {
		parts[i] = strconv.FormatUint(uint64(id), 10)
	}
	c.InboundIDs = strings.Join(parts, ",")
}

// ParseInboundIDsCSV parses a csv of inbound ids into a slice.
func ParseInboundIDsCSV(s string) []uint {
	out := make([]uint, 0)
	seen := map[uint]bool{}
	for _, part := range strings.Split(s, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		n, err := strconv.ParseUint(part, 10, 64)
		if err != nil || n == 0 || seen[uint(n)] {
			continue
		}
		seen[uint(n)] = true
		out = append(out, uint(n))
	}
	return out
}

// ExtraLinkLines splits ExtraLinks into non-empty trimmed lines.
func ExtraLinkLines(s string) []string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	raw := strings.ReplaceAll(s, "\r\n", "\n")
	raw = strings.ReplaceAll(raw, "\r", "\n")
	parts := strings.Split(raw, "\n")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

// NormalizeTrafficReset returns a valid trafficReset value.
func NormalizeTrafficReset(v string) string {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "daily", "weekly", "monthly":
		return strings.ToLower(strings.TrimSpace(v))
	default:
		return "never"
	}
}
