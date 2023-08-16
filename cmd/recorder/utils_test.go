package main

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSanitizeLog(t *testing.T) {
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
		{
			name:     "bearer",
			input:    "BEARER zyyh3sp1w3gijeg7a33g994wyh",
			expected: "BEARER XXX",
		},
		{
			name:     "hash",
			input:    "http://localhost:8065/plugins/com.mattermost.calls/standalone/recording.html?call_id=yu8njrnigpnt5nz7b1u46a5xee#eyJ0b2tlbiI6ICJhemY4bmZnend0ODlkZ3NnaWV6MXQ4Y3RmeSJ9",
			expected: "http://localhost:8065/plugins/com.mattermost.calls/standalone/recording.html?call_id=yu8njrnigpnt5nz7b1u46a5xee#XXX",
		},
		{
			name:     "token",
			input:    `"{\"action\":\"authentication_challenge\",\"seq\":1,\"data\":{\"token\":\"oqgcyx86qtgpbpwifid4wx7oay\"}}"}}`,
			expected: `"{\"action\":\"authentication_challenge\",\"seq\":1,\"data\":{\"token\":\"XXX\"}}"}}`,
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.expected, sanitizeLog(tc.input))
		})
	}
}
