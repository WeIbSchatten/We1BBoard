package security

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// EqualSecret compares secrets in constant time via SHA-256 digests
// (avoids length-leak from subtle.ConstantTimeCompare on unequal lengths).
func EqualSecret(a, b string) bool {
	ha := sha256.Sum256([]byte(a))
	hb := sha256.Sum256([]byte(b))
	var v byte
	for i := 0; i < len(ha); i++ {
		v |= ha[i] ^ hb[i]
	}
	return v == 0 && a != "" && b != ""
}

// ValidShareHost allows hostname or IP for subscription/share links (no URLs/paths).
func ValidShareHost(host string) bool {
	host = strings.TrimSpace(host)
	if host == "" || len(host) > 253 {
		return false
	}
	if strings.ContainsAny(host, "/:\\@?#%") {
		return false
	}
	if ip := net.ParseIP(host); ip != nil {
		return true
	}
	// basic hostname
	for _, p := range strings.Split(host, ".") {
		if p == "" || len(p) > 63 {
			return false
		}
		for _, c := range p {
			if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '-' {
				continue
			}
			return false
		}
	}
	return true
}

func isBlockedIP(ip net.IP) bool {
	if ip == nil {
		return true
	}
	return ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() ||
		ip.IsLinkLocalMulticast() || ip.IsUnspecified() ||
		ip.IsMulticast() || ip.IsInterfaceLocalMulticast()
}

func blockedHostName(host string) bool {
	lower := strings.ToLower(host)
	return lower == "localhost" || strings.HasSuffix(lower, ".localhost") ||
		lower == "metadata.google.internal" || lower == "metadata" ||
		strings.HasSuffix(lower, ".internal")
}

// ValidateDialHost rejects localhost/metadata names and private/loopback IPs
// (literal or after DNS resolve). Used for SSRF-sensitive dials (e.g. REALITY scan).
func ValidateDialHost(host string) error {
	host = strings.TrimSpace(host)
	if host == "" {
		return fmt.Errorf("host is required")
	}
	if blockedHostName(host) {
		return fmt.Errorf("host not allowed")
	}
	if ip := net.ParseIP(host); ip != nil {
		if isBlockedIP(ip) {
			return fmt.Errorf("must not point to private/loopback addresses")
		}
		return nil
	}
	ips, err := net.LookupIP(host)
	if err != nil || len(ips) == 0 {
		return fmt.Errorf("host cannot be resolved")
	}
	for _, ip := range ips {
		if isBlockedIP(ip) {
			return fmt.Errorf("must not point to private/loopback addresses")
		}
	}
	return nil
}

// ValidateNodeURL rejects non-http(s) schemes and SSRF targets for remote nodes.
// For hostnames it resolves DNS and rejects private/loopback/link-local answers.
func ValidateNodeURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("invalid node url")
	}
	if u.Scheme != "https" && u.Scheme != "http" {
		return fmt.Errorf("node url must be http or https")
	}
	if u.Host == "" || u.User != nil {
		return fmt.Errorf("invalid node url host")
	}
	host := u.Hostname()
	if host == "" {
		return fmt.Errorf("invalid node url host")
	}
	if err := ValidateDialHost(host); err != nil {
		return fmt.Errorf("node url %v", err)
	}
	return nil
}

// SafeHTTPClient dials only after re-checking resolved IPs (mitigates DNS rebinding).
func SafeHTTPClient(insecureTLS bool, timeout time.Duration) *http.Client {
	base := &net.Dialer{Timeout: 10 * time.Second, KeepAlive: 30 * time.Second}
	tr := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			host, port, err := net.SplitHostPort(addr)
			if err != nil {
				return nil, err
			}
			if blockedHostName(host) {
				return nil, fmt.Errorf("blocked host")
			}
			if ip := net.ParseIP(host); ip != nil {
				if isBlockedIP(ip) {
					return nil, fmt.Errorf("blocked address")
				}
				return base.DialContext(ctx, network, net.JoinHostPort(ip.String(), port))
			}
			ips, err := net.DefaultResolver.LookupIPAddr(ctx, host)
			if err != nil || len(ips) == 0 {
				return nil, fmt.Errorf("resolve failed")
			}
			var last error
			for _, ipa := range ips {
				if isBlockedIP(ipa.IP) {
					last = fmt.Errorf("blocked address")
					continue
				}
				conn, err := base.DialContext(ctx, network, net.JoinHostPort(ipa.IP.String(), port))
				if err == nil {
					return conn, nil
				}
				last = err
			}
			if last == nil {
				last = fmt.Errorf("no usable address")
			}
			return nil, last
		},
		TLSClientConfig:       &tls.Config{InsecureSkipVerify: insecureTLS}, //nolint:gosec // operator TLSMode=skip
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          16,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}
	return &http.Client{
		Timeout:   timeout,
		Transport: tr,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
}

// MaskSecret truncates a secret for API responses.
func MaskSecret(s string) string {
	if s == "" {
		return ""
	}
	if len(s) > 8 {
		return s[:4] + "…" + s[len(s)-4:]
	}
	return "***"
}
