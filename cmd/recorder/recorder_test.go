package main

import (
	"testing"

	"github.com/mattermost/calls-recorder/cmd/recorder/config"

	"github.com/stretchr/testify/require"
)

func TestNewRecorder(t *testing.T) {
	t.Run("invalid config", func(t *testing.T) {
		rec, err := NewRecorder(config.RecorderConfig{})
		require.EqualError(t, err, "invalid config: config cannot be empty")
		require.Nil(t, rec)
	})

	t.Run("valid config", func(t *testing.T) {
		cfg := config.RecorderConfig{
			SiteURL:     "http://localhost:8065",
			CallID:      "8w8jorhr7j83uqr6y1st894hqe",
			ThreadID:    "udzdsg7dwidbzcidx5khrf8nee",
			RecordingID: "67t5u6cmtfbb7jug739d43xa9e",
			AuthToken:   "qj75unbsef83ik9p7ueypb6iyw",
		}
		cfg.SetDefaults()
		rec, err := NewRecorder(cfg)
		require.NoError(t, err)
		require.NotNil(t, rec)
	})
}
