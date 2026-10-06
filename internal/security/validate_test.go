package security

import "testing"

func TestEqualSecret(t *testing.T) {
	if !EqualSecret("abc", "abc") {
		t.Fatal("equal")
	}
	if EqualSecret("a", "b") {
		t.Fatal("should differ")
	}
	if EqualSecret("", "") {
		t.Fatal("empty not allowed")
	}
}

func equalSecret(a, b string) bool { return EqualSecret(a, b) }

func TestValidShareHost(t *testing.T) {
	for _, h := range []string{"example.com", "sub.example.com", "1.2.3.4"} {
		if !ValidShareHost(h) {
			t.Fatalf("expected ok: %s", h)
		}
	}
	for _, h := range []string{"", "http://x", "a/b", "x@y", "evil.com/path"} {
		if ValidShareHost(h) {
			t.Fatalf("expected bad: %s", h)
		}
	}
}

func TestValidateNodeURL(t *testing.T) {
	// literal public IP — no DNS needed
	if err := ValidateNodeURL("https://93.184.216.34:2053"); err != nil {
		t.Fatal(err)
	}
	for _, u := range []string{
		"file:///etc/passwd",
		"http://127.0.0.1/",
		"http://10.0.0.1/",
		"http://169.254.169.254/",
		"http://localhost/",
		"http://metadata.google.internal/",
	} {
		if err := ValidateNodeURL(u); err == nil {
			t.Fatalf("expected reject %s", u)
		}
	}
}

func TestMaskSecret(t *testing.T) {
	if MaskSecret("abcdefghijklmnop") != "abcd…mnop" {
		t.Fatal(MaskSecret("abcdefghijklmnop"))
	}
	if MaskSecret("short") != "***" {
		t.Fatal(MaskSecret("short"))
	}
}

func TestValidateDownloadRedirectURL(t *testing.T) {
	for _, u := range []string{
		"https://objects.githubusercontent.com/github-production-release-asset/1/x",
		"https://release-assets.githubusercontent.com/github-production-release-asset/1/x",
		"https://github.com/XTLS/Xray-core/releases/download/v1.8.24/Xray-linux-64.zip",
	} {
		if err := ValidateDownloadRedirectURL(u); err != nil {
			t.Fatalf("expected allow %s: %v", u, err)
		}
	}
	for _, u := range []string{
		"https://evil.example.com/payload",
		"file:///etc/passwd",
		"http://127.0.0.1/x",
	} {
		if err := ValidateDownloadRedirectURL(u); err == nil {
			t.Fatalf("expected reject %s", u)
		}
	}
}
