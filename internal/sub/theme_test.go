package sub

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveThemeFile(t *testing.T) {
	dir := t.TempDir()
	if _, err := ResolveThemeFile(""); err != nil {
		t.Fatal(err)
	}
	if err := ValidateThemeDir(""); err != nil {
		t.Fatal(err)
	}
	if err := ValidateThemeDir("relative/path"); err == nil {
		t.Fatal("expected absolute path required")
	}
	// empty dir → error when validating non-empty path with no file
	if err := ValidateThemeDir(dir); err == nil {
		t.Fatal("expected missing template error")
	}
	p := filepath.Join(dir, "sub.html")
	if err := os.WriteFile(p, []byte(`<html><body>{{.subTitle}}</body></html>`), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := ResolveThemeFile(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got != p {
		t.Fatalf("got %s want %s", got, p)
	}
	if err := ValidateThemeDir(dir); err != nil {
		t.Fatal(err)
	}
}

func TestResolveThemePrefersSubHTML(t *testing.T) {
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "index.html"), []byte(`index`), 0o644)
	_ = os.WriteFile(filepath.Join(dir, "sub.html"), []byte(`sub`), 0o644)
	got, err := ResolveThemeFile(dir)
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(got) != "sub.html" {
		t.Fatal(got)
	}
}

func TestRejectSymlinkThemeFile(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "secret.txt")
	_ = os.WriteFile(target, []byte("secret"), 0o644)
	link := filepath.Join(dir, "sub.html")
	if err := os.Symlink(target, link); err != nil {
		t.Skip("symlinks not supported:", err)
	}
	if _, err := ResolveThemeFile(dir); err == nil {
		t.Fatal("expected symlink reject")
	}
}

func TestRejectHugeTheme(t *testing.T) {
	dir := t.TempDir()
	big := make([]byte, maxThemeFileBytes+1)
	_ = os.WriteFile(filepath.Join(dir, "sub.html"), big, 0o644)
	if _, err := ResolveThemeFile(dir); err == nil {
		t.Fatal("expected size reject")
	}
}
