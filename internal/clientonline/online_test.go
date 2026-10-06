package clientonline

import "testing"

func TestParseAccessLine(t *testing.T) {
	line := "2024/01/02 15:04:05.123456 from 1.2.3.4:555 accepted tcp:example.com:443 [inbound-tag >> outbound-tag] email: alice@example.com"
	e, ok := ParseAccessLine(line)
	if !ok {
		t.Fatal("expected ok")
	}
	if e.Email != "alice@example.com" {
		t.Fatalf("email = %q", e.Email)
	}
	if e.IP != "1.2.3.4" {
		t.Fatalf("ip = %q", e.IP)
	}
	if _, ok := ParseAccessLine("no email here"); ok {
		t.Fatal("expected fail without email")
	}
}
