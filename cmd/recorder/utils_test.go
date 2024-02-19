package main

import (
	"bytes"
	"log/slog"
	"os"
	"strings"
	"testing"

	"github.com/mattermost/calls-recorder/cmd/recorder/config"

	"github.com/stretchr/testify/require"
)

func TestSanitizeConsoleLog(t *testing.T) {
	tcs := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name: "empty string",
		},
		{
			name:     "alphanumeric",
			input:    "ice-pwd:aBc123",
			expected: "ice-pwd:XXX",
		},
		{
			name:     "special chars",
			input:    "ice-pwd:/aBc+1/2",
			expected: "ice-pwd:XXX",
		},
		{
			name:     "with ending",
			input:    "ice-pwd:abc123\\rtest",
			expected: "ice-pwd:XXX\\rtest",
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.expected, sanitizeConsoleLog(tc.input))
		})
	}
}

func TestCheckOSRequirements(t *testing.T) {
	var logBuf bytes.Buffer

	logger := slog.New(slog.NewTextHandler(&logBuf, &slog.HandlerOptions{
		Level: slog.LevelDebug,
		ReplaceAttr: func(_ []string, a slog.Attr) slog.Attr {
			if a.Key == slog.TimeKey {
				a.Value = slog.StringValue("")
			}
			return a
		},
	}))

	defLogger := slog.Default()
	slog.SetDefault(logger)
	defer slog.SetDefault(defLogger)

	defer func(path string) {
		unpriviledgeUsersCloneSysctlPath = path
	}(unpriviledgeUsersCloneSysctlPath)

	f, err := os.CreateTemp("", "unprivileged_userns_clone")
	require.NoError(t, err)
	defer f.Close()
	defer os.Remove(f.Name())
	unpriviledgeUsersCloneSysctlPath = f.Name()

	t.Run("on", func(t *testing.T) {
		_, err := f.Write([]byte("1"))
		require.NoError(t, err)
		err = checkOSRequirements()
		require.NoError(t, err)
		require.Equal(t, `time="" level=DEBUG msg="kernel.unprivileged_userns_clone is correctly set"`, strings.TrimSpace(logBuf.String()))
	})

	t.Run("off", func(t *testing.T) {
		_, err := f.Seek(0, 0)
		require.NoError(t, err)

		_, err = f.Write([]byte("0"))
		require.NoError(t, err)

		err = checkOSRequirements()
		require.EqualError(t, err, "kernel.unprivileged_userns_clone should be enabled for the recording process to work")
	})

	t.Run("missing", func(t *testing.T) {
		unpriviledgeUsersCloneSysctlPath = "/tmp/invalid"
		logBuf.Reset()
		err = checkOSRequirements()
		require.NoError(t, err)
		require.Equal(t, `time="" level=WARN msg="failed to read sysctl" err="open /tmp/invalid: no such file or directory"`, strings.TrimSpace(logBuf.String()))
	})
}

func TestSanitizeFilename(t *testing.T) {
	tcs := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name: "empty string",
		},
		{
			name:     "spaces",
			input:    "some file name with spaces.mp4",
			expected: "some_file_name_with_spaces.mp4",
		},
		{
			name:     "special chars",
			input:    "somefile*with??special/\\chars.mp4",
			expected: "somefile_with__special__chars.mp4",
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.expected, sanitizeFilename(tc.input))
		})
	}
}

func TestGenChromiumOptions(t *testing.T) {
	t.Run("secure SiteURL", func(t *testing.T) {
		var cfg config.RecorderConfig
		cfg.SetDefaults()
		cfg.SiteURL = "https://mm-server"
		opts, ctxOpts, err := genChromiumOptions(cfg)
		require.NoError(t, err)
		require.Len(t, opts, 34)
		require.Len(t, ctxOpts, 1)
	})

	t.Run("plain text SiteURL", func(t *testing.T) {
		var cfg config.RecorderConfig
		cfg.SetDefaults()
		cfg.SiteURL = "http://mm-server"
		opts, ctxOpts, err := genChromiumOptions(cfg)
		require.NoError(t, err)
		require.Len(t, opts, 35)
		require.Len(t, ctxOpts, 1)
	})

	t.Run("dev mode", func(t *testing.T) {
		os.Setenv("DEV_MODE", "true")
		defer os.Unsetenv("DEV_MODE")
		var cfg config.RecorderConfig
		cfg.SetDefaults()
		cfg.SiteURL = "http://localhost:8065"
		opts, ctxOpts, err := genChromiumOptions(cfg)
		require.NoError(t, err)
		require.Len(t, opts, 36)
		require.Len(t, ctxOpts, 3)
	})
}

func TestGetInsecureOrigins(t *testing.T) {
	tcs := []struct {
		name     string
		siteURL  string
		expected []string
		err      string
		devMode  bool
	}{
		{
			name: "empty string",
			err:  "invalid siteURL: should not be empty",
		},
		{
			name:    "parse failure",
			siteURL: string([]byte{0x7f}),
			err:     `failed to parse SiteURL: parse "\x7f": net/url: invalid control character in URL`,
		},
		{
			name:    "secure siteURL",
			siteURL: "https://localhost",
		},
		{
			name:    "plain text siteURL",
			siteURL: "http://localhost",
			expected: []string{
				"http://localhost",
			},
		},
		{
			name:    "secure siteURL, dev mode",
			siteURL: "https://localhost",
			devMode: true,
			expected: []string{
				"http://172.17.0.1:8065",
				"http://host.docker.internal:8065",
				"http://mm-server:8065",
				"http://host.minikube.internal:8065",
			},
		},
		{
			name:    "plain text siteURL, dev mode",
			siteURL: "http://localhost",
			devMode: true,
			expected: []string{
				"http://localhost",
				"http://172.17.0.1:8065",
				"http://host.docker.internal:8065",
				"http://mm-server:8065",
				"http://host.minikube.internal:8065",
			},
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			if tc.devMode {
				os.Setenv("DEV_MODE", "true")
				defer os.Unsetenv("DEV_MODE")
			}

			origins, err := getInsecureOrigins(tc.siteURL)
			if tc.err != "" {
				require.EqualError(t, err, tc.err)
				require.Empty(t, origins)
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.expected, origins)
			}
		})
	}
}
