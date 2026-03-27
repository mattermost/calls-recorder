package main

import (
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/mattermost/calls-recorder/cmd/recorder/config"

	"github.com/stretchr/testify/require"
)

// writeTLSServerCert writes the certificate of a httptest.TLSServer to a temp
// file in PEM format and returns its path. The caller is responsible for
// removing the file.
func writeTLSServerCert(t *testing.T, ts *httptest.Server) string {
	t.Helper()
	certFile, err := os.CreateTemp("", "test_ca_*.pem")
	require.NoError(t, err)
	cert := ts.Certificate()
	err = pem.Encode(certFile, &pem.Block{Type: "CERTIFICATE", Bytes: cert.Raw})
	require.NoError(t, err)
	require.NoError(t, certFile.Close())
	t.Cleanup(func() { os.Remove(certFile.Name()) })
	return certFile.Name()
}

func TestNewRecorder(t *testing.T) {
	t.Run("invalid config", func(t *testing.T) {
		rec, err := NewRecorder(config.RecorderConfig{}, getDataDir(""))
		require.EqualError(t, err, "invalid config: config cannot be empty")
		require.Nil(t, rec)
	})

	t.Run("invalid data path", func(t *testing.T) {
		cfg := config.RecorderConfig{
			SiteURL:     "http://localhost:8065",
			CallID:      "8w8jorhr7j83uqr6y1st894hqe",
			PostID:      "udzdsg7dwidbzcidx5khrf8nee",
			RecordingID: "67t5u6cmtfbb7jug739d43xa9e",
			AuthToken:   "qj75unbsef83ik9p7ueypb6iyw",
		}
		cfg.SetDefaults()

		rec, err := NewRecorder(cfg, "")
		require.EqualError(t, err, "data path cannot be empty")
		require.Nil(t, rec)
	})

	t.Run("valid config", func(t *testing.T) {
		cfg := config.RecorderConfig{
			SiteURL:     "http://localhost:8065",
			CallID:      "8w8jorhr7j83uqr6y1st894hqe",
			PostID:      "udzdsg7dwidbzcidx5khrf8nee",
			RecordingID: "67t5u6cmtfbb7jug739d43xa9e",
			AuthToken:   "qj75unbsef83ik9p7ueypb6iyw",
		}
		cfg.SetDefaults()
		rec, err := NewRecorder(cfg, getDataDir(""))
		require.NoError(t, err)
		require.NotNil(t, rec)
	})
}

func TestNewRecorderTLS(t *testing.T) {
	ts := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	validCertFile := writeTLSServerCert(t, ts)

	baseConfig := func() config.RecorderConfig {
		cfg := config.RecorderConfig{
			SiteURL:     ts.URL,
			CallID:      "8w8jorhr7j83uqr6y1st894hqe",
			PostID:      "udzdsg7dwidbzcidx5khrf8nee",
			RecordingID: "67t5u6cmtfbb7jug739d43xa9e",
			AuthToken:   "qj75unbsef83ik9p7ueypb6iyw",
		}
		cfg.SetDefaults()
		return cfg
	}

	t.Run("ca cert file - connects successfully", func(t *testing.T) {
		cfg := baseConfig()
		cfg.TLSCACertFile = validCertFile
		rec, err := NewRecorder(cfg, getDataDir(""))
		require.NoError(t, err)
		require.NotNil(t, rec)

		resp, err := rec.client.HTTPClient.Get(ts.URL)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, resp.StatusCode)
	})

	t.Run("ca cert file - missing file", func(t *testing.T) {
		cfg := baseConfig()
		cfg.TLSCACertFile = "/nonexistent/cert.pem"
		rec, err := NewRecorder(cfg, getDataDir(""))
		require.EqualError(t, err, "failed to read CA certificate: open /nonexistent/cert.pem: no such file or directory")
		require.Nil(t, rec)
	})

	t.Run("ca cert file - invalid PEM", func(t *testing.T) {
		f, err := os.CreateTemp("", "bad_cert_*.pem")
		require.NoError(t, err)
		_, err = f.WriteString("not a valid certificate")
		require.NoError(t, err)
		require.NoError(t, f.Close())
		t.Cleanup(func() { os.Remove(f.Name()) })

		cfg := baseConfig()
		cfg.TLSCACertFile = f.Name()
		rec, err := NewRecorder(cfg, getDataDir(""))
		require.EqualError(t, err, "failed to parse CA certificate")
		require.Nil(t, rec)
	})

	t.Run("insecure skip verify - connects successfully", func(t *testing.T) {
		cfg := baseConfig()
		cfg.TLSInsecureSkipVerify = true
		rec, err := NewRecorder(cfg, getDataDir(""))
		require.NoError(t, err)
		require.NotNil(t, rec)

		resp, err := rec.client.HTTPClient.Get(ts.URL)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, resp.StatusCode)
	})

	t.Run("no tls config - fails without cert", func(t *testing.T) {
		cfg := baseConfig()
		rec, err := NewRecorder(cfg, getDataDir(""))
		require.NoError(t, err)
		require.NotNil(t, rec)

		// Default transport should reject self-signed cert
		_, err = rec.client.HTTPClient.Get(ts.URL)
		require.Error(t, err)
		require.Contains(t, err.Error(), "certificate")
	})
}
