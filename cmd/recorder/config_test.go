package main

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestConfigIsValid(t *testing.T) {
	tcs := []struct {
		name          string
		cfg           RecorderConfig
		expectedError string
	}{
		{
			name:          "empty config",
			cfg:           RecorderConfig{},
			expectedError: "config cannot be empty",
		},
		{
			name: "missing CallID",
			cfg: RecorderConfig{
				SiteURL: "http://localhost:8065",
			},
			expectedError: "CallID cannot be empty",
		},
		{
			name: "missing ThreadID",
			cfg: RecorderConfig{
				SiteURL: "http://localhost:8065",
				CallID:  "test-call-id",
			},
			expectedError: "ThreadID cannot be empty",
		},
		{
			name: "missing AuthToken",
			cfg: RecorderConfig{
				SiteURL:  "http://localhost:8065",
				CallID:   "test-call-id",
				ThreadID: "test-thread-id",
			},
			expectedError: "AuthToken cannot be empty",
		},
		{
			name: "invalid Width",
			cfg: RecorderConfig{
				SiteURL:   "http://localhost:8065",
				CallID:    "test-call-id",
				ThreadID:  "test-thread-id",
				AuthToken: "test-auth-token",
			},
			expectedError: "Width value is not valid",
		},
		{
			name: "invalid Height",
			cfg: RecorderConfig{
				SiteURL:   "http://localhost:8065",
				CallID:    "test-call-id",
				ThreadID:  "test-thread-id",
				AuthToken: "test-auth-token",
				Width:     1280,
			},
			expectedError: "Height value is not valid",
		},
		{
			name: "invalid VideoRate",
			cfg: RecorderConfig{
				SiteURL:   "http://localhost:8065",
				CallID:    "test-call-id",
				ThreadID:  "test-thread-id",
				AuthToken: "test-auth-token",
				Width:     1280,
				Height:    720,
			},
			expectedError: "VideoRate value is not valid",
		},
		{
			name: "invalid AudioRate",
			cfg: RecorderConfig{
				SiteURL:   "http://localhost:8065",
				CallID:    "test-call-id",
				ThreadID:  "test-thread-id",
				AuthToken: "test-auth-token",
				Width:     1280,
				Height:    720,
				VideoRate: 1000,
			},
			expectedError: "AudioRate value is not valid",
		},
		{
			name: "invalid FrameRate",
			cfg: RecorderConfig{
				SiteURL:   "http://localhost:8065",
				CallID:    "test-call-id",
				ThreadID:  "test-thread-id",
				AuthToken: "test-auth-token",
				Width:     1280,
				Height:    720,
				VideoRate: 1000,
				AudioRate: 64,
			},
			expectedError: "FrameRate value is not valid",
		},
		{
			name: "invalid format",
			cfg: RecorderConfig{
				SiteURL:   "http://localhost:8065",
				CallID:    "test-call-id",
				ThreadID:  "test-thread-id",
				AuthToken: "test-auth-token",
				Width:     1280,
				Height:    720,
				VideoRate: 1000,
				AudioRate: 64,
				FrameRate: 30,
			},
			expectedError: "OutputFormat value is not valid",
		},
		{
			name: "valid config",
			cfg: RecorderConfig{
				SiteURL:      "http://localhost:8065",
				CallID:       "test-call-id",
				ThreadID:     "test-thread-id",
				AuthToken:    "test-auth-token",
				Width:        1280,
				Height:       720,
				VideoRate:    1000,
				AudioRate:    64,
				FrameRate:    30,
				OutputFormat: AVFormatMP4,
			},
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.cfg.IsValid()
			if tc.expectedError == "" {
				require.NoError(t, err)
			} else {
				require.EqualError(t, err, tc.expectedError)
			}
		})
	}
}

func TestConfigSetDefaults(t *testing.T) {
	t.Run("empty input config", func(t *testing.T) {
		var cfg RecorderConfig
		cfg.SetDefaults()
		require.Equal(t, RecorderConfig{
			Width:        VideoWidthDefault,
			Height:       VideoHeightDefault,
			VideoRate:    VideoRateDefault,
			AudioRate:    AudioRateDefault,
			FrameRate:    FrameRateDefault,
			OutputFormat: OutputFormatDefault,
		}, cfg)
	})

	t.Run("no overrides", func(t *testing.T) {
		cfg := RecorderConfig{
			Width:  1280,
			Height: 720,
		}
		cfg.SetDefaults()
		require.Equal(t, RecorderConfig{
			Width:        1280,
			Height:       720,
			VideoRate:    VideoRateDefault,
			AudioRate:    AudioRateDefault,
			FrameRate:    FrameRateDefault,
			OutputFormat: OutputFormatDefault,
		}, cfg)
	})
}

func TestLoadConfig(t *testing.T) {
	t.Run("no env set", func(t *testing.T) {
		cfg, err := loadConfig()
		require.NoError(t, err)
		require.Empty(t, cfg)
	})

	t.Run("parsing failure", func(t *testing.T) {
		os.Setenv("WIDTH", "invalid")
		cfg, err := loadConfig()
		require.Empty(t, cfg)
		require.EqualError(t, err, `failed to parse Width: strconv.ParseInt: parsing "invalid": invalid syntax`)
		os.Unsetenv("WIDTH")

		os.Setenv("HEIGHT", "invalid")
		cfg, err = loadConfig()
		require.Empty(t, cfg)
		require.EqualError(t, err, `failed to parse Height: strconv.ParseInt: parsing "invalid": invalid syntax`)
		os.Unsetenv("HEIGHT")

		os.Setenv("VIDEO_RATE", "invalid")
		cfg, err = loadConfig()
		require.Empty(t, cfg)
		require.EqualError(t, err, `failed to parse VideoRate: strconv.ParseInt: parsing "invalid": invalid syntax`)
		os.Unsetenv("VIDEO_RATE")

		os.Setenv("AUDIO_RATE", "invalid")
		cfg, err = loadConfig()
		require.Empty(t, cfg)
		require.EqualError(t, err, `failed to parse AudioRate: strconv.ParseInt: parsing "invalid": invalid syntax`)
		os.Unsetenv("AUDIO_RATE")

		os.Setenv("FRAME_RATE", "invalid")
		cfg, err = loadConfig()
		require.Empty(t, cfg)
		require.EqualError(t, err, `failed to parse FrameRate: strconv.ParseInt: parsing "invalid": invalid syntax`)
		os.Unsetenv("FRAME_RATE")
	})

	t.Run("valid config", func(t *testing.T) {
		os.Setenv("SITE_URL", "http://localhost:8065/")
		defer os.Unsetenv("SITE_URL")
		os.Setenv("CALL_ID", "test-call-id")
		defer os.Unsetenv("CALL_ID")
		os.Setenv("THREAD_ID", "test-thread-id")
		defer os.Unsetenv("THREAD_ID")
		os.Setenv("AUTH_TOKEN", "test-auth-token")
		defer os.Unsetenv("AUTH_TOKEN")
		os.Setenv("WIDTH", "1920")
		defer os.Unsetenv("WIDTH")
		os.Setenv("HEIGHT", "1080")
		defer os.Unsetenv("HEIGHT")
		os.Setenv("VIDEO_RATE", "1000")
		defer os.Unsetenv("VIDEO_RATE")
		os.Setenv("AUDIO_RATE", "64")
		defer os.Unsetenv("AUDIO_RATE")
		os.Setenv("FRAME_RATE", "30")
		defer os.Unsetenv("FRAME_RATE")
		cfg, err := loadConfig()
		require.NoError(t, err)
		require.NotEmpty(t, cfg)
		require.Equal(t, RecorderConfig{
			SiteURL:   "http://localhost:8065/",
			CallID:    "test-call-id",
			ThreadID:  "test-thread-id",
			AuthToken: "test-auth-token",
			Width:     1920,
			Height:    1080,
			VideoRate: 1000,
			AudioRate: 64,
			FrameRate: 30,
		}, cfg)
	})
}
