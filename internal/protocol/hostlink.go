package protocol

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"github.com/we1bboard/we1bboard/internal/database/model"
)

// HostMatchesInbound reports whether host applies to the given inbound.
// InboundID > 0 → exact ID. InboundID 0 + InboundTag → tag match.
// InboundID 0 + empty tag → all inbounds.
func HostMatchesInbound(h model.Host, in *model.Inbound) bool {
	if in == nil {
		return false
	}
	if h.InboundID > 0 {
		return h.InboundID == in.ID
	}
	if h.InboundTag != "" {
		return h.InboundTag == in.Tag
	}
	return true
}

// ApplyHostOverrides returns a copy of inbound with port / stream overrides from host.
func ApplyHostOverrides(in model.Inbound, h model.Host) model.Inbound {
	if h.Port > 0 {
		in.Port = h.Port
	}
	stream := ParseStream(in.StreamSettings)
	security, _ := stream["security"].(string)
	network, _ := stream["network"].(string)
	if network == "" {
		network = "tcp"
	}

	if h.SNI != "" || h.Fingerprint != "" || h.ALPN != "" || h.AllowInsecure {
		switch security {
		case "reality":
			rs, _ := stream["realitySettings"].(map[string]any)
			if rs == nil {
				rs = map[string]any{}
			}
			if h.SNI != "" {
				rs["serverNames"] = []any{h.SNI}
			}
			if h.Fingerprint != "" {
				rs["fingerprint"] = h.Fingerprint
			}
			stream["realitySettings"] = rs
		default:
			ts, _ := stream["tlsSettings"].(map[string]any)
			if ts == nil {
				ts = map[string]any{}
			}
			if h.SNI != "" {
				ts["serverName"] = h.SNI
			}
			if h.Fingerprint != "" {
				ts["fingerprint"] = h.Fingerprint
			}
			if h.ALPN != "" {
				parts := splitCSV(h.ALPN)
				alpn := make([]any, 0, len(parts))
				for _, p := range parts {
					alpn = append(alpn, p)
				}
				ts["alpn"] = alpn
			}
			if h.AllowInsecure {
				ts["allowInsecure"] = true
			}
			stream["tlsSettings"] = ts
		}
	}

	if h.Path != "" || h.HostHeader != "" {
		switch network {
		case "ws":
			ws, _ := stream["wsSettings"].(map[string]any)
			if ws == nil {
				ws = map[string]any{}
			}
			if h.Path != "" {
				ws["path"] = h.Path
			}
			if h.HostHeader != "" {
				headers, _ := ws["headers"].(map[string]any)
				if headers == nil {
					headers = map[string]any{}
				}
				headers["Host"] = h.HostHeader
				ws["headers"] = headers
			}
			stream["wsSettings"] = ws
		case "httpupgrade":
			hs, _ := stream["httpupgradeSettings"].(map[string]any)
			if hs == nil {
				hs = map[string]any{}
			}
			if h.Path != "" {
				hs["path"] = h.Path
			}
			if h.HostHeader != "" {
				hs["host"] = h.HostHeader
			}
			stream["httpupgradeSettings"] = hs
		case "xhttp":
			xs, _ := stream["xhttpSettings"].(map[string]any)
			if xs == nil {
				xs = map[string]any{}
			}
			if h.Path != "" {
				xs["path"] = h.Path
			}
			if h.HostHeader != "" {
				xs["host"] = h.HostHeader
			}
			stream["xhttpSettings"] = xs
		case "grpc":
			if h.Path != "" {
				gs, _ := stream["grpcSettings"].(map[string]any)
				if gs == nil {
					gs = map[string]any{}
				}
				gs["serviceName"] = h.Path
				stream["grpcSettings"] = gs
			}
		}
	}

	b, _ := json.Marshal(stream)
	in.StreamSettings = string(b)
	return in
}

func splitCSV(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

// ShareResult is one generated client link with the address/inbound used to build it.
type ShareResult struct {
	Link    string
	Address string
	Inbound model.Inbound
}

// ShareLinksForHosts builds one share link per enabled matching host.
// If hosts is empty, returns a single link with defaultHost (current behavior).
func ShareLinksForHosts(in *model.Inbound, c model.Client, defaultHost string, hosts []model.Host) ([]string, error) {
	results, err := ShareResultsForHosts(in, c, defaultHost, hosts)
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(results))
	for _, r := range results {
		out = append(out, r.Link)
	}
	return out, nil
}

// ShareResultsForHosts is like ShareLinksForHosts but also returns the address and patched inbound
// used for each link (needed by Clash/sing-box so Host overrides apply).
func ShareResultsForHosts(in *model.Inbound, c model.Client, defaultHost string, hosts []model.Host) ([]ShareResult, error) {
	adap, err := Get(in.Protocol)
	if err != nil {
		return nil, err
	}
	matched := make([]model.Host, 0, len(hosts))
	for _, h := range hosts {
		if !h.Enable {
			continue
		}
		if HostMatchesInbound(h, in) {
			matched = append(matched, h)
		}
	}
	if len(matched) == 0 {
		link, err := adap.ShareLink(in, c, defaultHost)
		if err != nil {
			return nil, err
		}
		return []ShareResult{{Link: link, Address: defaultHost, Inbound: *in}}, nil
	}
	out := make([]ShareResult, 0, len(matched))
	for _, h := range matched {
		addr := strings.TrimSpace(h.Address)
		if addr == "" {
			addr = defaultHost
		}
		patched := ApplyHostOverrides(*in, h)
		link, err := adap.ShareLink(&patched, c, addr)
		if err != nil {
			continue
		}
		if h.Remark != "" {
			link = appendRemark(link, h.Remark)
		}
		out = append(out, ShareResult{Link: link, Address: addr, Inbound: patched})
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("no share links generated")
	}
	return out, nil
}

func appendRemark(link, remark string) string {
	enc := strings.ReplaceAll(url.QueryEscape(remark), "+", "%20")
	if i := strings.LastIndex(link, "#"); i >= 0 {
		return link[:i] + "#" + enc
	}
	return link + "#" + enc
}
