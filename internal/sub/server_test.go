package sub

import (
	"testing"
	"time"

	"github.com/we1bboard/we1bboard/internal/database/model"
)

func TestValidSubID(t *testing.T) {
	ok := []string{"0123456789abcdef", "0123456789abcdef0123456789abcdef", "aB_c-D12eFgH4567"}
	for _, id := range ok {
		if !ValidSubID(id) {
			t.Fatalf("expected valid: %s", id)
		}
	}
	bad := []string{"", "short", "abcd1234", "../etc", "id with space", "hex!", string(make([]byte, 70))}
	for _, id := range bad {
		if ValidSubID(id) {
			t.Fatalf("expected invalid: %q", id)
		}
	}
}

func TestDetectFormat(t *testing.T) {
	cases := []struct{ ua, q, want string }{
		{"ClashMeta/1.0", "", "clash"},
		{"sing-box/1.8", "", "singbox"},
		{"SFA/1.0", "", "singbox"},
		{"v2rayN/6", "", "base64"},
		{"", "clash", "clash"},
		{"Clash", "singbox", "singbox"},
		{"", "sing-box", "singbox"},
	}
	for _, tc := range cases {
		if got := detectFormat(tc.ua, tc.q); got != tc.want {
			t.Fatalf("ua=%q q=%q got %s want %s", tc.ua, tc.q, got, tc.want)
		}
	}
}

func TestClientActiveExpiry(t *testing.T) {
	now := time.Now().UnixMilli()
	if !clientActive(model.Client{Enable: true}, now) {
		t.Fatal("enabled no expiry")
	}
	if clientActive(model.Client{Enable: true, ExpiryTime: now - 1000}, now) {
		t.Fatal("expired should be filtered")
	}
	if !clientActive(model.Client{Enable: true, ExpiryTime: now + 60_000}, now) {
		t.Fatal("future expiry ok")
	}
	if clientActive(model.Client{Enable: true, TotalGB: 1, Up: 1 << 30, Down: 1 << 30}, now) {
		t.Fatal("traffic exceeded")
	}
}

func TestNormalizePath(t *testing.T) {
	if NormalizePath("sub") != "/sub/" {
		t.Fatal(NormalizePath("sub"))
	}
	if NormalizePath("/sub") != "/sub/" {
		t.Fatal(NormalizePath("/sub"))
	}
}

func TestValidateSettings(t *testing.T) {
	if err := ValidateSettings("subPort", "2096"); err != nil {
		t.Fatal(err)
	}
	if err := ValidateSettings("subPort", "99999"); err == nil {
		t.Fatal("bad port")
	}
	if err := ValidateSettings("subPath", "/sub/"); err != nil {
		t.Fatal(err)
	}
	if err := ValidateSettings("subPath", "../x"); err == nil {
		t.Fatal("path traversal")
	}
	if err := ValidateSettings("subPath", "/"); err == nil {
		t.Fatal("root path")
	}
	if err := ValidateSettings("subEnable", "maybe"); err == nil {
		t.Fatal("bad enable")
	}
	if err := ValidateSettings("subHost", "evil.com/path"); err == nil {
		t.Fatal("bad host")
	}
}
