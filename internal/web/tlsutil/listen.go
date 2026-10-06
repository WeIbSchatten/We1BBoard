package tlsutil

import (
	"crypto/tls"
	"fmt"
	"net/http"
	"os"
	"time"
)

// Listen starts HTTP or HTTPS depending on cert/key presence.
func Listen(addr, certFile, keyFile string, handler http.Handler) error {
	srv := &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       60 * time.Second,
		WriteTimeout:      120 * time.Second,
		IdleTimeout:       120 * time.Second,
		MaxHeaderBytes:    1 << 20,
	}

	if certFile == "" || keyFile == "" {
		fmt.Printf("TLS disabled (no cert) — listening HTTP on %s\n", addr)
		return srv.ListenAndServe()
	}
	if _, err := os.Stat(certFile); err != nil {
		return fmt.Errorf("cert file: %w", err)
	}
	if _, err := os.Stat(keyFile); err != nil {
		return fmt.Errorf("key file: %w", err)
	}
	srv.TLSConfig = &tls.Config{
		MinVersion: tls.VersionTLS12,
	}
	fmt.Printf("TLS enabled — listening HTTPS on %s\n", addr)
	return srv.ListenAndServeTLS(certFile, keyFile)
}
