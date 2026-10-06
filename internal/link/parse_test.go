package link

import "testing"

func TestParseVless(t *testing.T) {
	p, err := ParseOutboundLink("vless://11111111-1111-1111-1111-111111111111@example.com:443?type=ws&security=tls&sni=example.com&path=%2Fws#test")
	if err != nil {
		t.Fatal(err)
	}
	if p.Protocol != "vless" {
		t.Fatalf("protocol %s", p.Protocol)
	}
	if p.Remark != "test" {
		t.Fatalf("remark %q", p.Remark)
	}
	if p.Settings == "" || p.StreamSettings == "" {
		t.Fatal("empty settings")
	}
}

func TestParseTrojan(t *testing.T) {
	p, err := ParseOutboundLink("trojan://secret@10.0.0.1:443?security=tls&sni=a.com#tr")
	if err != nil {
		t.Fatal(err)
	}
	if p.Protocol != "trojan" {
		t.Fatalf("protocol %s", p.Protocol)
	}
}

func TestExtractShareLinksBase64(t *testing.T) {
	// "vless://x@h:1#a\nvless://y@h:2#b" base64
	body := "dmxlc3M6Ly94QGg6MSNhCnZsZXNzOi8veUBoOjIjYg=="
	links := ExtractShareLinks(body)
	if len(links) != 2 {
		t.Fatalf("got %d links: %#v", len(links), links)
	}
}
