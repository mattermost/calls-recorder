package main

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewRecorder(t *testing.T) {
	t.Run("invalid config", func(t *testing.T) {
		rec, err := NewRecorder(RecorderConfig{})
		require.EqualError(t, err, "invalid config: config cannot be empty")
		require.Nil(t, rec)
	})

	t.Run("valid config", func(t *testing.T) {
		cfg := RecorderConfig{
			SiteURL:   "http://localhost:8065",
			CallID:    "test-call-id",
			ThreadID:  "test-thread-id",
			AuthToken: "test-auth-token",
		}
		cfg.SetDefaults()
		rec, err := NewRecorder(cfg)
		require.NoError(t, err)
		require.NotNil(t, rec)
	})
}
