package main

import (
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
