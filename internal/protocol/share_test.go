package protocol

import (
	"encoding/base64"
	"encoding/json"
	"net/url"
	"strings"
	"testing"

	"github.com/we1bboard/we1bboard/internal/database/model"
)

func TestRealityPublicKeyFromPrivate(t *testing.T) {
	// Fixed X25519 private key (32 bytes) → known public
	priv := base64.RawURLEncoding.EncodeToString(make([]byte, 32))
	pub, err := RealityPublicKeyFromPrivate(priv)
	if err != nil {
		t.Fatal(err)
	}
	if pub == "" || pub == priv {
		t.Fatalf("bad pub %q", pub)
	}
	// round-trip via ResolveRealityPublicKey
	rs := map[string]any{"privateKey": priv}
	if got := ResolveRealityPublicKey(rs); got != pub {
		t.Fatalf("derive: %q != %q", got, pub)
	}
	rs2 := map[string]any{"settings": map[string]any{"publicKey": "storedPBK"}}
	if got := ResolveRealityPublicKey(rs2); got != "storedPBK" {
		t.Fatalf("nested: %q", got)
	}
}

func TestVLESSRealityShareLink(t *testing.T) {
	priv := base64.RawURLEncoding.EncodeToString([]byte{
		1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16,
		17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31, 32,
	})
	pub, err := RealityPublicKeyFromPrivate(priv)
	if err != nil {
		t.Fatal(err)
	}
	stream, _ := json.Marshal(map[string]any{
		"network":  "tcp",
		"security": "reality",
		"realitySettings": map[string]any{
			"privateKey":  priv,
			"serverNames": []any{"www.cloudflare.com"},
			"shortIds":    []any{"abcd1234"},
			"fingerprint": "chrome",
			"spiderX":     "/we1b",
			"dest":        "www.cloudflare.com:443",
			"settings": map[string]any{
				"publicKey":   pub,
				"fingerprint": "chrome",
				"serverName":  "www.cloudflare.com",
				"spiderX":     "/we1b",
			},
		},
	})
	in := &model.Inbound{
		Protocol:      model.ProtoVLESS,
		Port:          443,
		Remark:        "test",
		StreamSettings: string(stream),
		Settings:      `{"decryption":"none","clients":[]}`,
	}
	c := model.Client{UUID: "11111111-2222-3333-4444-555555555555", Email: "u@we1b", Flow: "xtls-rprx-vision"}
	adap, err := Get(model.ProtoVLESS)
	if err != nil {
		t.Fatal(err)
	}
	link, err := adap.ShareLink(in, c, "vpn.example.com")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(link, "vless://11111111-2222-3333-4444-555555555555@vpn.example.com:443?") {
		t.Fatalf("prefix: %s", link)
	}
	u, err := url.Parse(link)
	if err != nil {
		t.Fatal(err)
	}
	q := u.Query()
	if q.Get("security") != "reality" {
		t.Fatal(q.Get("security"))
	}
	if q.Get("pbk") != pub {
		t.Fatalf("pbk: %q want %q", q.Get("pbk"), pub)
	}
	if q.Get("sid") != "abcd1234" {
		t.Fatal(q.Get("sid"))
	}
	if q.Get("sni") != "www.cloudflare.com" {
		t.Fatal(q.Get("sni"))
	}
	if q.Get("spx") != "/we1b" {
		t.Fatal(q.Get("spx"))
	}
	if q.Get("flow") != "xtls-rprx-vision" {
		t.Fatal(q.Get("flow"))
	}
	if q.Get("fp") != "chrome" {
		t.Fatal(q.Get("fp"))
	}
	if q.Get("type") != "tcp" {
		t.Fatal(q.Get("type"))
	}
}

func TestVLESSRealityDerivesPBKWhenMissing(t *testing.T) {
	priv := base64.RawURLEncoding.EncodeToString([]byte{
		9, 8, 7, 6, 5, 4, 3, 2, 1, 0, 1, 2, 3, 4, 5, 6,
		7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22,
	})
	want, _ := RealityPublicKeyFromPrivate(priv)
	stream, _ := json.Marshal(map[string]any{
		"network": "tcp", "security": "reality",
		"realitySettings": map[string]any{
			"privateKey":  priv,
			"serverNames": []any{"a.com"},
			"shortIds":    []any{"aa"},
			// no publicKey on purpose
		},
	})
	in := &model.Inbound{Protocol: model.ProtoVLESS, Port: 8443, StreamSettings: string(stream)}
	c := model.Client{UUID: "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee", Email: "x"}
	adap, _ := Get(model.ProtoVLESS)
	link, err := adap.ShareLink(in, c, "1.2.3.4")
	if err != nil {
		t.Fatal(err)
	}
	u, _ := url.Parse(link)
	if u.Query().Get("pbk") != want {
		t.Fatalf("got %q want %q link=%s", u.Query().Get("pbk"), want, link)
	}
}

func TestVMessShareLinkHasHostPath(t *testing.T) {
	stream, _ := json.Marshal(map[string]any{
		"network": "ws", "security": "tls",
		"wsSettings": map[string]any{
			"path": "/ray",
			"headers": map[string]any{"Host": "cdn.example.com"},
		},
		"tlsSettings": map[string]any{
			"serverName":  "cdn.example.com",
			"fingerprint": "chrome",
			"alpn":        []any{"h2", "http/1.1"},
		},
	})
	in := &model.Inbound{Protocol: model.ProtoVMess, Port: 443, StreamSettings: string(stream)}
	c := model.Client{UUID: "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee", Email: "vm"}
	adap, _ := Get(model.ProtoVMess)
	link, err := adap.ShareLink(in, c, "vpn.example.com")
	if err != nil {
		t.Fatal(err)
	}
	raw, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(link, "vmess://"))
	if err != nil {
		t.Fatal(err)
	}
	var obj map[string]any
	if err := json.Unmarshal(raw, &obj); err != nil {
		t.Fatal(err)
	}
	if obj["host"] != "cdn.example.com" || obj["path"] != "/ray" || obj["tls"] != "tls" {
		t.Fatalf("%v", obj)
	}
	if obj["sni"] != "cdn.example.com" || obj["fp"] != "chrome" {
		t.Fatalf("%v", obj)
	}
}

func TestIPv6ShareHost(t *testing.T) {
	in := &model.Inbound{
		Protocol: model.ProtoVLESS, Port: 443,
		StreamSettings: `{"network":"tcp","security":"none"}`,
	}
	c := model.Client{UUID: "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee", Email: "x"}
	adap, _ := Get(model.ProtoVLESS)
	link, err := adap.ShareLink(in, c, "2001:db8::1")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(link, "@[2001:db8::1]:443?") {
		t.Fatal(link)
	}
}
