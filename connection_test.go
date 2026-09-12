package main

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"net"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestConnectionTLSModes(t *testing.T) {
	for _, tc := range []struct {
		host, mode      string
		secure, wantErr bool
	}{
		{"db.example.com", "", true, false}, {"localhost", "", false, false},
		{"127.0.0.1", "auto", false, false}, {"::1", "auto", false, false},
		{"localhost", "true", true, false}, {"db.example.com", "false", false, false},
		{"db.example.com", "skip-verify", false, true}, {"db.example.com", "preferred", false, true},
	} {
		t.Run(tc.host+"/"+tc.mode, func(t *testing.T) {
			cfg, err := connectionConfig(map[string]string{"username": "root", "password": "a@b:/?secret", "hostname": tc.host, "port": "3306", "tls": tc.mode}, time.Second)
			if (err != nil) != tc.wantErr {
				t.Fatalf("unexpected error: %v", err)
			}
			if err != nil {
				return
			}
			if cfg.Collation != "utf8mb4_general_ci" {
				t.Fatalf("unexpected collation: %q", cfg.Collation)
			}
			if _, ok := cfg.Params["charset"]; ok {
				t.Fatal("charset must not be sent as a server system variable")
			}
			if (cfg.TLS != nil) != tc.secure {
				t.Fatal("wrong TLS mode")
			}
			if cfg.TLS != nil && (cfg.TLS.InsecureSkipVerify || cfg.TLS.MinVersion < tls.VersionTLS12 || cfg.TLS.ServerName != tc.host) {
				t.Fatal("unsafe TLS")
			}
			if cfg.AllowFallbackToPlaintext {
				t.Fatal("TLS downgrade allowed")
			}
			if cfg.Passwd != "a@b:/?secret" {
				t.Fatal("password damaged by DSN parsing")
			}
			if tc.host == "::1" && cfg.Addr != "[::1]:3306" {
				t.Fatal("invalid IPv6 address")
			}
		})
	}
}
func TestCustomCAAndSocket(t *testing.T) {
	server := httptest.NewTLSServer(nil)
	defer server.Close()
	cert := server.Certificate()
	path := filepath.Join(t.TempDir(), "ca.pem")
	os.WriteFile(path, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: cert.Raw}), 0600)
	values := map[string]string{"username": "root", "hostname": "127.0.0.1", "port": "3306", "tls-ca": path}
	cfg, err := connectionConfig(values, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.TLS == nil {
		t.Fatal("CA must enable TLS on loopback")
	}
	if _, err := cert.Verify(x509.VerifyOptions{Roots: cfg.TLS.RootCAs}); err != nil {
		t.Fatal(err)
	}
	values["tls"] = "false"
	if _, err := connectionConfig(values, time.Second); err == nil {
		t.Fatal("conflicting CA accepted")
	}
	values["tls"] = "true"
	os.WriteFile(path, []byte("invalid"), 0600)
	if _, err := connectionConfig(values, time.Second); err == nil {
		t.Fatal("invalid CA accepted")
	}
	values = map[string]string{"username": "root", "socket": filepath.Join(t.TempDir(), "mysql.sock")}
	cfg, err = connectionConfig(values, time.Second)
	if err != nil || cfg.Net != "unix" {
		t.Fatalf("socket: %v", err)
	}
	values["tls"] = "true"
	if _, err := connectionConfig(values, time.Second); err == nil {
		t.Fatal("TLS with socket accepted")
	}
}

func TestTLSHandshakeVerification(t *testing.T) {
	server := httptest.NewTLSServer(nil)
	defer server.Close()
	address := strings.TrimPrefix(server.URL, "https://")
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "ca.pem")
	if err := os.WriteFile(path, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: server.Certificate().Raw}), 0600); err != nil {
		t.Fatal(err)
	}
	values := map[string]string{"username": "root", "hostname": host, "port": port, "tls": "true", "tls-ca": path}
	cfg, err := connectionConfig(values, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	dial := func(config *tls.Config) error {
		conn, err := tls.DialWithDialer(&net.Dialer{Timeout: time.Second}, "tcp", address, config)
		if conn != nil {
			conn.Close()
		}
		return err
	}
	if err := dial(cfg.TLS); err != nil {
		t.Fatalf("trusted TLS failed: %v", err)
	}
	wrongHost := cfg.TLS.Clone()
	wrongHost.ServerName = "wrong.example.invalid"
	if err := dial(wrongHost); err == nil {
		t.Fatal("wrong hostname accepted")
	}
	untrusted := cfg.TLS.Clone()
	untrusted.RootCAs = x509.NewCertPool()
	if err := dial(untrusted); err == nil {
		t.Fatal("unknown CA accepted")
	}
}
