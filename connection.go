package main

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-sql-driver/mysql"
)

func connectionConfig(values map[string]string, timeout time.Duration) (*mysql.Config, error) {
	cfg := mysql.NewConfig()
	cfg.Logger = silentDBLogger{}
	cfg.User, cfg.Passwd = values["username"], values["password"]
	if cfg.User == "" {
		return nil, errors.New("config missing username")
	}
	cfg.Timeout, cfg.ReadTimeout, cfg.WriteTimeout = timeout, timeout, timeout
	cfg.ParseTime = true
	cfg.Loc = time.Local
	// Config.Params is for server system variables. Putting "charset" there
	// becomes SET charset=..., which MariaDB rejects with error 1193. Use the
	// driver's collation field instead; utf8mb4_general_ci is broadly supported.
	cfg.Collation = "utf8mb4_general_ci"
	mode := strings.ToLower(values["tls"])
	if mode == "" {
		mode = "auto"
	}
	if mode != "auto" && mode != "true" && mode != "false" {
		return nil, errors.New("tls must be auto, true or false")
	}
	ca := values["tls-ca"]
	if values["socket"] != "" {
		if !filepath.IsAbs(values["socket"]) {
			return nil, errors.New("socket path must be absolute")
		}
		if mode == "true" || ca != "" {
			return nil, errors.New("TLS options cannot be used with a Unix socket")
		}
		cfg.Net, cfg.Addr = "unix", values["socket"]
		return cfg, nil
	}
	host, port := values["hostname"], values["port"]
	if host == "" || port == "" {
		return nil, errors.New("TCP requires hostname and port")
	}
	cfg.Net, cfg.Addr = "tcp", net.JoinHostPort(host, port)
	ip := net.ParseIP(host)
	local := strings.EqualFold(host, "localhost") || (ip != nil && ip.IsLoopback())
	secure := mode == "true" || (mode == "auto" && (!local || ca != ""))
	if mode == "false" && ca != "" {
		return nil, errors.New("tls-ca conflicts with tls=false")
	}
	if secure {
		cfg.TLS = &tls.Config{MinVersion: tls.VersionTLS12, ServerName: host}
		if ca != "" {
			pem, err := os.ReadFile(ca)
			if err != nil {
				return nil, fmt.Errorf("read TLS CA: %w", err)
			}
			roots, err := x509.SystemCertPool()
			if err != nil {
				roots = x509.NewCertPool()
			}
			if !roots.AppendCertsFromPEM(pem) {
				return nil, errors.New("TLS CA contains no valid certificates")
			}
			cfg.TLS.RootCAs = roots
		}
	} else if !local {
		fmt.Fprintln(os.Stderr, "WARNING: remote TCP without TLS exposes credentials and data")
	}
	return cfg, nil
}

type silentDBLogger struct{}

func (silentDBLogger) Print(...any) {}
