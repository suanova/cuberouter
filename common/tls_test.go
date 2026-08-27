package common

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"io"
	"math/big"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// clearTLSSettingsEnv blanks every TLS_* env var so each test starts from a
// known state regardless of the caller's environment.
func clearTLSSettingsEnv(t *testing.T) {
	t.Setenv("TLS_CERT_FILE", "")
	t.Setenv("TLS_KEY_FILE", "")
	t.Setenv("TLS_PORT", "")
}

// TestGetTLSSettingsDisabledWithoutEnv guards the backward-compatible default:
// with no TLS_* env vars the server must keep running pure HTTP.
func TestGetTLSSettingsDisabledWithoutEnv(t *testing.T) {
	clearTLSSettingsEnv(t)

	settings, err := GetTLSSettings()
	require.NoError(t, err)
	assert.False(t, settings.Enabled)
	assert.Empty(t, settings.CertFile)
	assert.Empty(t, settings.KeyFile)
	assert.Empty(t, settings.Port)
}

// TestGetTLSSettingsDisabledWithGarbagePort guards that an unset TLS stays
// disabled even when TLS_PORT holds an invalid value; the port is only
// meaningful once HTTPS is enabled.
func TestGetTLSSettingsDisabledWithGarbagePort(t *testing.T) {
	clearTLSSettingsEnv(t)
	t.Setenv("TLS_PORT", "garbage")

	settings, err := GetTLSSettings()
	require.NoError(t, err)
	assert.False(t, settings.Enabled)
}

// TestGetTLSSettingsRequiresCertAndKeyPair guards against half-configured
// HTTPS: one file without the other is a misconfiguration, not a feature.
func TestGetTLSSettingsRequiresCertAndKeyPair(t *testing.T) {
	clearTLSSettingsEnv(t)
	t.Setenv("TLS_CERT_FILE", "/tmp/does-not-matter.pem")
	_, err := GetTLSSettings()
	require.Error(t, err)

	clearTLSSettingsEnv(t)
	t.Setenv("TLS_KEY_FILE", "/tmp/does-not-matter.key")
	_, err = GetTLSSettings()
	require.Error(t, err)
}

// TestGetTLSSettingsMissingFile guards that a configured but unreadable cert or
// key is rejected at startup instead of failing later inside the listener.
func TestGetTLSSettingsMissingFile(t *testing.T) {
	clearTLSSettingsEnv(t)
	dir := t.TempDir()
	t.Setenv("TLS_CERT_FILE", filepath.Join(dir, "missing.pem"))
	t.Setenv("TLS_KEY_FILE", filepath.Join(dir, "missing.key"))

	_, err := GetTLSSettings()
	require.Error(t, err)
}

// TestGetTLSSettingsRejectsMalformedPEM guards that non-certificate content in
// the configured files fails startup validation instead of the listener.
func TestGetTLSSettingsRejectsMalformedPEM(t *testing.T) {
	clearTLSSettingsEnv(t)
	dir := t.TempDir()
	cert := filepath.Join(dir, "cert.pem")
	key := filepath.Join(dir, "key.pem")
	require.NoError(t, os.WriteFile(cert, []byte("not a certificate"), 0o600))
	require.NoError(t, os.WriteFile(key, []byte("not a key"), 0o600))
	t.Setenv("TLS_CERT_FILE", cert)
	t.Setenv("TLS_KEY_FILE", key)

	_, err := GetTLSSettings()
	require.Error(t, err)
}

// TestGetTLSSettingsRejectsMismatchedPair guards that a certificate and key
// that do not belong together fail startup validation.
func TestGetTLSSettingsRejectsMismatchedPair(t *testing.T) {
	clearTLSSettingsEnv(t)
	cert, _ := generateSelfSignedCert(t)
	_, key := generateSelfSignedCert(t)
	t.Setenv("TLS_CERT_FILE", cert)
	t.Setenv("TLS_KEY_FILE", key)

	_, err := GetTLSSettings()
	require.Error(t, err)
}

// TestGetTLSSettingsRejectsDirectory guards that a directory path is not
// accepted as a cert or key file.
func TestGetTLSSettingsRejectsDirectory(t *testing.T) {
	clearTLSSettingsEnv(t)
	dir := t.TempDir()
	cert, key := generateSelfSignedCert(t)
	t.Setenv("TLS_CERT_FILE", dir)
	t.Setenv("TLS_KEY_FILE", key)
	_, err := GetTLSSettings()
	require.Error(t, err)

	clearTLSSettingsEnv(t)
	t.Setenv("TLS_CERT_FILE", cert)
	t.Setenv("TLS_KEY_FILE", dir)
	_, err = GetTLSSettings()
	require.Error(t, err)
}

// TestGetTLSSettingsRejectsUnreadableFile guards that a file the process
// cannot open fails startup validation.
func TestGetTLSSettingsRejectsUnreadableFile(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root ignores permission bits; cannot exercise an unreadable file")
	}
	clearTLSSettingsEnv(t)
	cert, _ := generateSelfSignedCert(t)
	key, _ := generateSelfSignedCert(t)
	require.NoError(t, os.Chmod(key, 0o000))
	t.Cleanup(func() { _ = os.Chmod(key, 0o600) })
	t.Setenv("TLS_CERT_FILE", cert)
	t.Setenv("TLS_KEY_FILE", key)

	_, err := GetTLSSettings()
	require.Error(t, err)
}

// TestGetTLSSettingsDefaultsPortToFlag guards the default HTTPS port: with no
// TLS_PORT env var the -tls-port flag value (443) is used.
func TestGetTLSSettingsDefaultsPortToFlag(t *testing.T) {
	clearTLSSettingsEnv(t)
	cert, key := generateSelfSignedCert(t)
	t.Setenv("TLS_CERT_FILE", cert)
	t.Setenv("TLS_KEY_FILE", key)

	settings, err := GetTLSSettings()
	require.NoError(t, err)
	assert.True(t, settings.Enabled)
	assert.Equal(t, cert, settings.CertFile)
	assert.Equal(t, key, settings.KeyFile)
	assert.Equal(t, "443", settings.Port)
}

// TestGetTLSSettingsPortEnvWinsOverFlag guards the env-over-flag precedence
// that PORT already uses for the HTTP listener.
func TestGetTLSSettingsPortEnvWinsOverFlag(t *testing.T) {
	clearTLSSettingsEnv(t)
	cert, key := generateSelfSignedCert(t)
	t.Setenv("TLS_CERT_FILE", cert)
	t.Setenv("TLS_KEY_FILE", key)
	t.Setenv("TLS_PORT", "8443")

	old := *TLSPort
	*TLSPort = 1234
	defer func() { *TLSPort = old }()

	settings, err := GetTLSSettings()
	require.NoError(t, err)
	assert.Equal(t, "8443", settings.Port)
}

// TestGetTLSSettingsRejectsInvalidPort guards that a non-numeric or
// out-of-range TLS_PORT fails startup loudly instead of dying in a goroutine.
func TestGetTLSSettingsRejectsInvalidPort(t *testing.T) {
	clearTLSSettingsEnv(t)
	cert, key := generateSelfSignedCert(t)
	t.Setenv("TLS_CERT_FILE", cert)
	t.Setenv("TLS_KEY_FILE", key)

	for _, bad := range []string{"abc", "0", "-1", "70000"} {
		t.Setenv("TLS_PORT", bad)
		_, err := GetTLSSettings()
		require.Error(t, err, "port %q must be rejected", bad)
	}
}

// generateSelfSignedCert writes a real server-auth certificate and key pair to
// a temp dir, so the listener test exercises an actual TLS handshake.
func generateSelfSignedCert(t *testing.T) (certFile, keyFile string) {
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "localhost"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	der, err := x509.CreateCertificate(rand.Reader, &template, &template, &privateKey.PublicKey, privateKey)
	require.NoError(t, err)
	keyDer, err := x509.MarshalECPrivateKey(privateKey)
	require.NoError(t, err)

	dir := t.TempDir()
	certFile = filepath.Join(dir, "cert.pem")
	keyFile = filepath.Join(dir, "key.pem")
	require.NoError(t, os.WriteFile(certFile, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), 0o600))
	require.NoError(t, os.WriteFile(keyFile, pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDer}), 0o600))
	return certFile, keyFile
}

// TestNewTLSServerServesHTTPS guards the full HTTPS wiring: the constructed
// server must complete a real TLS handshake on the configured port while plain
// HTTP against the same port fails.
func TestNewTLSServerServesHTTPS(t *testing.T) {
	certFile, keyFile := generateSelfSignedCert(t)
	// Bind the listener ourselves so serving starts before any request is
	// issued; ServeTLS exercises the same cert/key wiring as ListenAndServeTLS.
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	settings := TLSSettings{Enabled: true, CertFile: certFile, KeyFile: keyFile,
		Port: strconv.Itoa(listener.Addr().(*net.TCPAddr).Port)}

	srv, err := NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("tls ok"))
	}), settings)
	require.NoError(t, err)
	assert.Equal(t, ":"+settings.Port, srv.Addr)
	go func() { _ = srv.ServeTLS(listener, settings.CertFile, settings.KeyFile) }()
	t.Cleanup(func() { _ = srv.Close() })

	client := &http.Client{Transport: &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}}
	resp, err := client.Get("https://127.0.0.1:" + settings.Port + "/")
	require.NoError(t, err)
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.NoError(t, resp.Body.Close())
	assert.Equal(t, "tls ok", string(body))

	// Plain HTTP against the TLS-only listener is not served: Go's TLS server
	// sniffs the non-TLS handshake and answers 400 instead of hanging.
	resp, err = http.Get("http://127.0.0.1:" + settings.Port + "/")
	require.NoError(t, err)
	body, err = io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.NoError(t, resp.Body.Close())
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	assert.Contains(t, string(body), "Client sent an HTTP request to an HTTPS server")
}

// TestNewTLSServerRejectsDisabledSettings guards the constructor contract:
// building a TLS server without enabled settings is a programming error.
func TestNewTLSServerRejectsDisabledSettings(t *testing.T) {
	_, err := NewTLSServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}), TLSSettings{})
	require.Error(t, err)
}
