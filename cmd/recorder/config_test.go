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
			name: "invalid SiteURL schema",
			cfg: RecorderConfig{
				SiteURL: "invalid://localhost",
			},
			expectedError: "SiteURL parsing failed: invalid scheme \"invalid\"",
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
				SiteURL:   "http://localhost:8065",
				CallID:    "8w8jorhr7j83uqr6y1st894hqe",
				AuthToken: "qj75unbsef83ik9p7ueypb6iyw",
			},
			expectedError: "ThreadID cannot be empty",
		},
		{
			name: "missing AuthToken",
			cfg: RecorderConfig{
				SiteURL:  "http://localhost:8065",
				CallID:   "8w8jorhr7j83uqr6y1st894hqe",
				ThreadID: "udzdsg7dwidbzcidx5khrf8nee",
			},
			expectedError: "AuthToken cannot be empty",
		},
		{
			name: "invalid Width",
			cfg: RecorderConfig{
				SiteURL:   "http://localhost:8065",
				CallID:    "8w8jorhr7j83uqr6y1st894hqe",
				ThreadID:  "udzdsg7dwidbzcidx5khrf8nee",
				AuthToken: "qj75unbsef83ik9p7ueypb6iyw",
			},
			expectedError: "Width value is not valid",
		},
		{
			name: "invalid Height",
			cfg: RecorderConfig{
				SiteURL:   "http://localhost:8065",
				CallID:    "8w8jorhr7j83uqr6y1st894hqe",
				ThreadID:  "udzdsg7dwidbzcidx5khrf8nee",
				AuthToken: "qj75unbsef83ik9p7ueypb6iyw",
				Width:     1280,
			},
			expectedError: "Height value is not valid",
		},
		{
			name: "invalid VideoRate",
			cfg: RecorderConfig{
				SiteURL:   "http://localhost:8065",
				CallID:    "8w8jorhr7j83uqr6y1st894hqe",
				ThreadID:  "udzdsg7dwidbzcidx5khrf8nee",
				AuthToken: "qj75unbsef83ik9p7ueypb6iyw",
				Width:     1280,
				Height:    720,
			},
			expectedError: "VideoRate value is not valid",
		},
		{
			name: "invalid AudioRate",
			cfg: RecorderConfig{
				SiteURL:   "http://localhost:8065",
				CallID:    "8w8jorhr7j83uqr6y1st894hqe",
				ThreadID:  "udzdsg7dwidbzcidx5khrf8nee",
				AuthToken: "qj75unbsef83ik9p7ueypb6iyw",
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
				CallID:    "8w8jorhr7j83uqr6y1st894hqe",
				ThreadID:  "udzdsg7dwidbzcidx5khrf8nee",
				AuthToken: "qj75unbsef83ik9p7ueypb6iyw",
				Width:     1280,
				Height:    720,
				VideoRate: 1000,
				AudioRate: 64,
			},
			expectedError: "FrameRate value is not valid",
		},
		{
			name: "invalid video preset",
			cfg: RecorderConfig{
				SiteURL:      "http://localhost:8065",
				CallID:       "8w8jorhr7j83uqr6y1st894hqe",
				ThreadID:     "udzdsg7dwidbzcidx5khrf8nee",
				AuthToken:    "qj75unbsef83ik9p7ueypb6iyw",
				Width:        1280,
				Height:       720,
				VideoRate:    1000,
				AudioRate:    64,
				FrameRate:    30,
				OutputFormat: AVFormatMP4,
			},
			expectedError: "VideoPreset value is not valid",
		},
		{
			name: "invalid format",
			cfg: RecorderConfig{
				SiteURL:   "http://localhost:8065",
				CallID:    "8w8jorhr7j83uqr6y1st894hqe",
				ThreadID:  "udzdsg7dwidbzcidx5khrf8nee",
				AuthToken: "qj75unbsef83ik9p7ueypb6iyw",
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
				CallID:       "8w8jorhr7j83uqr6y1st894hqe",
				ThreadID:     "udzdsg7dwidbzcidx5khrf8nee",
				AuthToken:    "qj75unbsef83ik9p7ueypb6iyw",
				Width:        1280,
				Height:       720,
				VideoRate:    1000,
				AudioRate:    64,
				FrameRate:    30,
				VideoPreset:  "medium",
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
			VideoPreset:  VideoPresetDefault,
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
			VideoPreset:  VideoPresetDefault,
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
		os.Setenv("CALL_ID", "8w8jorhr7j83uqr6y1st894hqe")
		defer os.Unsetenv("CALL_ID")
		os.Setenv("THREAD_ID", "udzdsg7dwidbzcidx5khrf8nee")
		defer os.Unsetenv("THREAD_ID")
		os.Setenv("AUTH_TOKEN", "qj75unbsef83ik9p7ueypb6iyw")
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
		os.Setenv("VIDEO_PRESET", "medium")
		defer os.Unsetenv("VIDEO_PRESET")
		cfg, err := loadConfig()
		require.NoError(t, err)
		require.NotEmpty(t, cfg)
		require.Equal(t, RecorderConfig{
			SiteURL:     "http://localhost:8065",
			CallID:      "8w8jorhr7j83uqr6y1st894hqe",
			ThreadID:    "udzdsg7dwidbzcidx5khrf8nee",
			AuthToken:   "qj75unbsef83ik9p7ueypb6iyw",
			Width:       1920,
			Height:      1080,
			VideoRate:   1000,
			AudioRate:   64,
			FrameRate:   30,
			VideoPreset: H264PresetMedium,
		}, cfg)
	})
}

func TestRecorderConfigToEnv(t *testing.T) {
	var cfg RecorderConfig
	cfg.SiteURL = "http://localhost:8065"
	cfg.CallID = "8w8jorhr7j83uqr6y1st894hqe"
	cfg.AuthToken = "qj75unbsef83ik9p7ueypb6iyw"
	cfg.ThreadID = "udzdsg7dwidbzcidx5khrf8nee"
	cfg.SetDefaults()
	require.Equal(t, []string{
		"SITE_URL=http://localhost:8065",
		"CALL_ID=8w8jorhr7j83uqr6y1st894hqe",
		"THREAD_ID=udzdsg7dwidbzcidx5khrf8nee",
		"AUTH_TOKEN=qj75unbsef83ik9p7ueypb6iyw",
		"WIDTH=1920",
		"HEIGHT=1080",
		"VIDEO_RATE=1500",
		"AUDIO_RATE=64",
		"FRAME_RATE=30",
		"VIDEO_PRESET=fast",
		"OUTPUT_FORMAT=mp4",
	}, cfg.ToEnv())
}

func TestRecorderConfigMap(t *testing.T) {
	var cfg RecorderConfig
	cfg.SiteURL = "http://localhost:8065"
	cfg.CallID = "8w8jorhr7j83uqr6y1st894hqe"
	cfg.AuthToken = "qj75unbsef83ik9p7ueypb6iyw"
	cfg.ThreadID = "udzdsg7dwidbzcidx5khrf8nee"
	cfg.SetDefaults()

	t.Run("default config", func(t *testing.T) {
		var c RecorderConfig
		err := c.FromMap(cfg.ToMap()).IsValid()
		require.NoError(t, err)
	})
}
