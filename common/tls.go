package common

import (
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
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
// TLS_PORT is ignored. A half-configured or invalid TLS setup is a startup
// error so misconfiguration is loud instead of failing inside the listener.
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

	for _, file := range []string{certFile, keyFile} {
		if _, err := os.Stat(file); err != nil {
			return settings, fmt.Errorf("cannot read TLS file %q: %w", file, err)
		}
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

// NewTLSServer builds the HTTPS server that shares the given handler with the
// HTTP listener. Certificate content is not validated here; the listener fails
// loudly at startup if the files are unusable.
func NewTLSServer(handler http.Handler, settings TLSSettings) (*http.Server, error) {
	if !settings.Enabled {
		return nil, fmt.Errorf("cannot build TLS server: TLS settings are not enabled")
	}
	return &http.Server{
		Addr:    ":" + settings.Port,
		Handler: handler,
	}, nil
}
