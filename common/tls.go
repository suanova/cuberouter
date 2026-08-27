package common

import (
	"crypto/tls"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

// TLSSettings is the resolved HTTPS listener configuration. Enabled is true
// only when both TLS_CERT_FILE and TLS_KEY_FILE are set; otherwise the gateway
// keeps serving pure HTTP on the regular port.
type TLSSettings struct {
	Enabled  bool
	CertFile string
	KeyFile  string
	Port     string
}

// GetTLSSettings resolves HTTPS configuration from the TLS_CERT_FILE,
// TLS_KEY_FILE and TLS_PORT environment variables plus the -tls-port flag.
// HTTPS is opt-in: without a cert/key pair no TLS server is started and
// TLS_PORT is ignored. A half-configured, unreadable or invalid TLS setup is a
// startup error so misconfiguration is loud instead of failing inside the
// listener.
func GetTLSSettings() (TLSSettings, error) {
	certFile := strings.TrimSpace(os.Getenv("TLS_CERT_FILE"))
	keyFile := strings.TrimSpace(os.Getenv("TLS_KEY_FILE"))

	settings := TLSSettings{}
	if certFile == "" && keyFile == "" {
		return settings, nil
	}
	if certFile == "" || keyFile == "" {
		return settings, fmt.Errorf("TLS_CERT_FILE and TLS_KEY_FILE must be set together")
	}
	if err := validateTLSCertPair(certFile, keyFile); err != nil {
		return settings, err
	}

	port := strings.TrimSpace(os.Getenv("TLS_PORT"))
	if port == "" {
		port = strconv.Itoa(*TLSPort)
	}
	portNumber, err := strconv.Atoi(port)
	if err != nil || portNumber < 1 || portNumber > 65535 {
		return settings, fmt.Errorf("invalid TLS_PORT %q: must be a port number between 1 and 65535", port)
	}

	settings.Enabled = true
	settings.CertFile = certFile
	settings.KeyFile = keyFile
	settings.Port = strconv.Itoa(portNumber)
	return settings, nil
}

// validateTLSCertPair checks that both files are readable regular files and
// form a matching key pair, so a broken TLS setup fails startup synchronously
// instead of dying asynchronously inside the TLS listener. The loaded pair is
// discarded; the listener reloads it when serving.
func validateTLSCertPair(certFile, keyFile string) error {
	for _, file := range []string{certFile, keyFile} {
		f, err := os.Open(file)
		if err != nil {
			return fmt.Errorf("cannot read TLS file %q: %w", file, err)
		}
		info, statErr := f.Stat()
		closeErr := f.Close()
		if statErr != nil {
			return fmt.Errorf("cannot stat TLS file %q: %w", file, statErr)
		}
		if closeErr != nil {
			return fmt.Errorf("cannot close TLS file %q: %w", file, closeErr)
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("TLS file %q is not a regular file", file)
		}
	}
	if _, err := tls.LoadX509KeyPair(certFile, keyFile); err != nil {
		return fmt.Errorf("invalid TLS certificate/key pair: %w", err)
	}
	return nil
}

// NewTLSServer builds the HTTPS server that shares the given handler with the
// HTTP listener. GetTLSSettings already validated the certificate pair; the
// listener reloads it when serving. The timeouts keep slow clients from
// holding connections open; WriteTimeout is deliberately omitted because SSE
// responses stream for a long time.
func NewTLSServer(handler http.Handler, settings TLSSettings) (*http.Server, error) {
	if !settings.Enabled {
		return nil, fmt.Errorf("cannot build TLS server: TLS settings are not enabled")
	}
	return &http.Server{
		Addr:              ":" + settings.Port,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       60 * time.Second,
	}, nil
}
