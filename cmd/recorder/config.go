package main

import (
	"fmt"
	"os"
	"strconv"
)

type AVFormat string

const (
	AVFormatMP4 AVFormat = "mp4"
)

const (
	// defaults
	VideoWidthDefault   = 1920
	VideoHeightDefault  = 1080
	VideoRateDefault    = 1500
	AudioRateDefault    = 64
	FrameRateDefault    = 30
	OutputFormatDefault = AVFormatMP4

	// limits
	VideoWidthMin  = 1280
	VideoWidthMax  = 3840
	VideoHeightMin = 720
	VideoHeightMax = 2160
	VideoRateMin   = 500
	VideoRateMax   = 10000
	AudioRateMin   = 32
	AudioRateMax   = 320
	FrameRateMin   = 10
	FrameRateMax   = 60
)

type RecorderConfig struct {
	// input config
	SiteURL   string
	CallID    string
	ThreadID  string
	AuthToken string

	// output config
	Width        int
	Height       int
	VideoRate    int
	AudioRate    int
	FrameRate    int
	OutputFormat AVFormat
}

func (cfg RecorderConfig) IsValid() error {
	if cfg == (RecorderConfig{}) {
		return fmt.Errorf("config cannot be empty")
	}
	if cfg.SiteURL == "" {
		return fmt.Errorf("SiteURL cannot be empty")
	}
	if cfg.CallID == "" {
		return fmt.Errorf("CallID cannot be empty")
	}
	if cfg.ThreadID == "" {
		return fmt.Errorf("ThreadID cannot be empty")
	}
	if cfg.AuthToken == "" {
		return fmt.Errorf("AuthToken cannot be empty")
	}
	if cfg.Width < VideoWidthMin || cfg.Width > VideoWidthMax {
		return fmt.Errorf("Width value is not valid")
	}
	if cfg.Height < VideoHeightMin || cfg.Height > VideoHeightMax {
		return fmt.Errorf("Height value is not valid")
	}
	if cfg.VideoRate < VideoRateMin || cfg.VideoRate > VideoRateMax {
		return fmt.Errorf("VideoRate value is not valid")
	}
	if cfg.AudioRate < AudioRateMin || cfg.AudioRate > AudioRateMax {
		return fmt.Errorf("AudioRate value is not valid")
	}
	if cfg.FrameRate < FrameRateMin || cfg.FrameRate > FrameRateMax {
		return fmt.Errorf("FrameRate value is not valid")
	}
	if cfg.OutputFormat != AVFormatMP4 {
		return fmt.Errorf("OutputFormat value is not valid")
	}

	return nil
}

func (cfg *RecorderConfig) SetDefaults() {
	if cfg.Width == 0 {
		cfg.Width = VideoWidthDefault
	}

	if cfg.Height == 0 {
		cfg.Height = VideoHeightDefault
	}

	if cfg.VideoRate == 0 {
		cfg.VideoRate = VideoRateDefault
	}

	if cfg.AudioRate == 0 {
		cfg.AudioRate = AudioRateDefault
	}

	if cfg.FrameRate == 0 {
		cfg.FrameRate = FrameRateDefault
	}

	if cfg.OutputFormat == "" {
		cfg.OutputFormat = OutputFormatDefault
	}
}

func loadConfig() (RecorderConfig, error) {
	var cfg RecorderConfig
	cfg.SiteURL = os.Getenv("SITE_URL")
	cfg.CallID = os.Getenv("CALL_ID")
	cfg.ThreadID = os.Getenv("THREAD_ID")
	cfg.AuthToken = os.Getenv("AUTH_TOKEN")

	if val := os.Getenv("WIDTH"); val != "" {
		width, err := strconv.ParseInt(val, 10, 32)
		if err != nil {
			return cfg, fmt.Errorf("failed to parse Width: %w", err)
		}
		cfg.Width = int(width)
	}

	if val := os.Getenv("HEIGHT"); val != "" {
		height, err := strconv.ParseInt(val, 10, 32)
		if err != nil {
			return cfg, fmt.Errorf("failed to parse Height: %w", err)
		}
		cfg.Height = int(height)
	}

	if val := os.Getenv("VIDEO_RATE"); val != "" {
		rate, err := strconv.ParseInt(val, 10, 32)
		if err != nil {
			return cfg, fmt.Errorf("failed to parse VideoRate: %w", err)
		}
		cfg.VideoRate = int(rate)
	}

	if val := os.Getenv("AUDIO_RATE"); val != "" {
		rate, err := strconv.ParseInt(val, 10, 32)
		if err != nil {
			return cfg, fmt.Errorf("failed to parse AudioRate: %w", err)
		}
		cfg.AudioRate = int(rate)
	}

	if val := os.Getenv("FRAME_RATE"); val != "" {
		rate, err := strconv.ParseInt(val, 10, 32)
		if err != nil {
			return cfg, fmt.Errorf("failed to parse FrameRate: %w", err)
		}
		cfg.FrameRate = int(rate)
	}

	return cfg, nil
}
