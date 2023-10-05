package main

import (
	"bytes"
	"log/slog"
	"os"
	"strings"
	"testing"

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
