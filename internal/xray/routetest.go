package xray

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// RouteTestInput is a simplified traffic sample for first-match rule evaluation.
type RouteTestInput struct {
	InboundTag string `json:"inboundTag"`
	Domain     string `json:"domain"`
	IP         string `json:"ip"`
	Port       string `json:"port"`
	Network    string `json:"network"`
	Protocol   string `json:"protocol"`
	User       string `json:"user"`
}

// RouteTestResult is returned by RouteTest.
type RouteTestResult struct {
	OutboundTag string         `json:"outboundTag,omitempty"`
	BalancerTag string         `json:"balancerTag,omitempty"`
	MatchedRule map[string]any `json:"matchedRule"`
}

// RouteTest evaluates merged DB+template routing rules (first match).
func (m *Manager) RouteTest(in RouteTestInput) (*RouteTestResult, error) {
	cfg, err := m.GenerateConfig()
	if err != nil {
		return nil, err
	}
	routing, _ := cfg["routing"].(map[string]any)
	if routing == nil {
		return &RouteTestResult{MatchedRule: nil}, nil
	}
	for _, rule := range routingRulesFrom(routing["rules"]) {
		if !matchRoutingRule(rule, in) {
			continue
		}
		res := &RouteTestResult{MatchedRule: rule}
		if bt := strings.TrimSpace(asString(rule["balancerTag"])); bt != "" {
			res.BalancerTag = bt
		} else if ot := strings.TrimSpace(asString(rule["outboundTag"])); ot != "" {
			res.OutboundTag = ot
		}
		return res, nil
	}
	return &RouteTestResult{MatchedRule: nil}, nil
}

func routingRulesFrom(v any) []map[string]any {
	switch t := v.(type) {
	case []map[string]any:
		return t
	case []any:
		out := make([]map[string]any, 0, len(t))
		for _, r := range t {
			if m, ok := r.(map[string]any); ok {
				out = append(out, m)
			}
		}
		return out
	default:
		return nil
	}
}

func asString(v any) string {
	if v == nil {
		return ""
	}
	switch t := v.(type) {
	case string:
		return t
	case float64:
		return strconv.FormatInt(int64(t), 10)
	case json.Number:
		return t.String()
	default:
		return fmt.Sprint(v)
	}
}

func matchRoutingRule(rule map[string]any, in RouteTestInput) bool {
	if tags := asStringSlice(rule["inboundTag"]); len(tags) > 0 {
		if in.InboundTag == "" || !stringInFold(tags, in.InboundTag) {
			return false
		}
	}
	if domains := asStringSlice(rule["domain"]); len(domains) > 0 {
		if in.Domain == "" || !domainMatches(domains, in.Domain) {
			return false
		}
	}
	if ips := asStringSlice(rule["ip"]); len(ips) > 0 {
		if in.IP == "" || !stringInFold(ips, in.IP) {
			return false
		}
	}
	if portCond := strings.TrimSpace(asString(rule["port"])); portCond != "" {
		if in.Port == "" || !portMatches(portCond, in.Port) {
			return false
		}
	}
	if netCond := strings.TrimSpace(asString(rule["network"])); netCond != "" {
		if in.Network == "" || !csvContains(netCond, in.Network) {
			return false
		}
	}
	if protos := asStringSlice(rule["protocol"]); len(protos) > 0 {
		if in.Protocol == "" || !stringInFold(protos, in.Protocol) {
			return false
		}
	}
	if users := asStringSlice(rule["user"]); len(users) > 0 {
		if in.User == "" || !stringInFold(users, in.User) {
			return false
		}
	}
	return true
}

func stringInFold(list []string, want string) bool {
	want = strings.TrimSpace(want)
	for _, s := range list {
		if strings.EqualFold(strings.TrimSpace(s), want) {
			return true
		}
	}
	return false
}

func csvContains(csv, want string) bool {
	parts := splitCSV(csv)
	return stringInFold(parts, want)
}

func domainMatches(ruleDomains []string, domain string) bool {
	domain = strings.ToLower(strings.TrimSpace(domain))
	for _, d := range ruleDomains {
		d = strings.TrimSpace(d)
		if d == "" {
			continue
		}
		low := strings.ToLower(d)
		switch {
		case strings.EqualFold(d, domain):
			return true
		case strings.HasPrefix(low, "domain:"):
			suf := strings.TrimPrefix(low, "domain:")
			if domain == suf || strings.HasSuffix(domain, "."+suf) {
				return true
			}
		case strings.HasPrefix(low, "full:"):
			if domain == strings.TrimPrefix(low, "full:") {
				return true
			}
		case strings.HasPrefix(low, "geosite:"):
			if strings.EqualFold(d, domain) {
				return true
			}
		}
	}
	return false
}

func portMatches(cond, portStr string) bool {
	port, err := strconv.Atoi(strings.TrimSpace(portStr))
	if err != nil {
		return false
	}
	for _, part := range splitCSV(cond) {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if strings.Contains(part, "-") {
			ab := strings.SplitN(part, "-", 2)
			if len(ab) != 2 {
				continue
			}
			a, errA := strconv.Atoi(strings.TrimSpace(ab[0]))
			b, errB := strconv.Atoi(strings.TrimSpace(ab[1]))
			if errA == nil && errB == nil && port >= a && port <= b {
				return true
			}
			continue
		}
		if p, err := strconv.Atoi(part); err == nil && p == port {
			return true
		}
	}
	return false
}
