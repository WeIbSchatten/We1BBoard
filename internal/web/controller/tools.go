package controller

import (
	"crypto/ecdh"
	"crypto/rand"
	"crypto/tls"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/we1bboard/we1bboard/internal/security"
)

// RealityKeys generates an X25519 keypair + shortId for REALITY inbounds (3x-ui style).
func (a *API) RealityKeys(c *gin.Context) {
	curve := ecdh.X25519()
	priv, err := curve.GenerateKey(rand.Reader)
	if err != nil {
		fail(c, 500, err)
		return
	}
	pub := priv.PublicKey()
	sid := make([]byte, 8)
	if _, err := rand.Read(sid); err != nil {
		fail(c, 500, err)
		return
	}
	ok(c, gin.H{
		"privateKey": base64.RawURLEncoding.EncodeToString(priv.Bytes()),
		"publicKey":  base64.RawURLEncoding.EncodeToString(pub.Bytes()),
		"shortId":    hex.EncodeToString(sid),
	})
}

// WireGuardKeys generates a Curve25519 keypair in standard base64 (WireGuard / Xray format).
func (a *API) WireGuardKeys(c *gin.Context) {
	curve := ecdh.X25519()
	priv, err := curve.GenerateKey(rand.Reader)
	if err != nil {
		fail(c, 500, err)
		return
	}
	pub := priv.PublicKey()
	ok(c, gin.H{
		"privateKey": base64.StdEncoding.EncodeToString(priv.Bytes()),
		"publicKey":  base64.StdEncoding.EncodeToString(pub.Bytes()),
		"secretKey":  base64.StdEncoding.EncodeToString(priv.Bytes()),
	})
}

// RandomUUID returns a new UUID string for client forms.
func (a *API) RandomUUID(c *gin.Context) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		fail(c, http.StatusInternalServerError, err)
		return
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	ok(c, gin.H{
		"uuid": hex.EncodeToString(b[0:4]) + "-" +
			hex.EncodeToString(b[4:6]) + "-" +
			hex.EncodeToString(b[6:8]) + "-" +
			hex.EncodeToString(b[8:10]) + "-" +
			hex.EncodeToString(b[10:16]),
	})
}

const fetchSubMaxBytes = 2 << 20 // 2 MiB

// FetchSub GETs a remote subscription URL (SSRF-guarded) and returns the body.
func (a *API) FetchSub(c *gin.Context) {
	var req struct {
		URL string `json:"url" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, http.StatusBadRequest, err)
		return
	}
	if err := security.ValidateNodeURL(req.URL); err != nil {
		fail(c, http.StatusBadRequest, err)
		return
	}
	client := security.SafeHTTPClient(false, 30*time.Second)
	resp, err := client.Get(req.URL)
	if err != nil {
		fail(c, http.StatusBadGateway, fmt.Errorf("fetch failed: %w", err))
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		fail(c, http.StatusBadGateway, fmt.Errorf("upstream status %d", resp.StatusCode))
		return
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, fetchSubMaxBytes+1))
	if err != nil {
		fail(c, http.StatusBadGateway, fmt.Errorf("read failed: %w", err))
		return
	}
	if len(body) > fetchSubMaxBytes {
		fail(c, http.StatusBadRequest, fmt.Errorf("body exceeds %d bytes", fetchSubMaxBytes))
		return
	}
	ok(c, gin.H{"body": string(body)})
}

// RealityScan dials TLS to target (host:port) with SNI=host and returns cert SANs.
// POST /tools/reality-scan  body: { "target": "www.cloudflare.com:443" }
func (a *API) RealityScan(c *gin.Context) {
	var req struct {
		Target string `json:"target" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, http.StatusBadRequest, err)
		return
	}
	target := strings.TrimSpace(req.Target)
	host, portStr, err := net.SplitHostPort(target)
	if err != nil {
		// allow bare host → :443
		host = target
		portStr = "443"
		target = net.JoinHostPort(host, portStr)
	}
	host = strings.TrimSpace(host)
	port, err := strconv.Atoi(portStr)
	if err != nil || port < 1 || port > 65535 {
		ok(c, gin.H{"dest": target, "serverNames": []string{}, "ok": false, "error": "invalid port"})
		return
	}
	if err := security.ValidateDialHost(host); err != nil {
		ok(c, gin.H{"dest": target, "serverNames": []string{}, "ok": false, "error": err.Error()})
		return
	}
	addr := net.JoinHostPort(host, strconv.Itoa(port))
	dialer := &net.Dialer{Timeout: 5 * time.Second}
	raw, err := dialer.Dial("tcp", addr)
	if err != nil {
		ok(c, gin.H{"dest": addr, "serverNames": []string{}, "ok": false, "error": err.Error()})
		return
	}
	defer raw.Close()
	_ = raw.SetDeadline(time.Now().Add(5 * time.Second))
	tlsConn := tls.Client(raw, &tls.Config{
		ServerName:         host,
		InsecureSkipVerify: true, //nolint:gosec // probe only; we only read SANs
		MinVersion:         tls.VersionTLS12,
	})
	if err := tlsConn.Handshake(); err != nil {
		ok(c, gin.H{"dest": addr, "serverNames": []string{}, "ok": false, "error": err.Error()})
		return
	}
	defer tlsConn.Close()
	st := tlsConn.ConnectionState()
	names := make([]string, 0)
	seen := map[string]bool{}
	add := func(n string) {
		n = strings.TrimSpace(n)
		if n == "" || strings.HasPrefix(n, "*.") || seen[n] {
			return
		}
		seen[n] = true
		names = append(names, n)
	}
	if len(st.PeerCertificates) > 0 {
		leaf := st.PeerCertificates[0]
		for _, n := range leaf.DNSNames {
			add(n)
		}
		for _, ip := range leaf.IPAddresses {
			add(ip.String())
		}
	}
	ok(c, gin.H{"dest": addr, "serverNames": names, "ok": true})
}
