package config

import (
	"fmt"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
)

var idRE = regexp.MustCompile(`^[a-z0-9]{26}$`)

type AVFormat string

const (
	AVFormatMP4 AVFormat = "mp4"
)

type H264Preset string

const (
	H264PresetMedium    = "medium"
	H264PresetFast      = "fast"
	H264PresetFaster    = "faster"
	H264PresetVeryFast  = "veryfast"
	H264PresetSuperFast = "superfast"
	H264PresetUltraFast = "ultrafast"
)

const (
	// defaults
	VideoWidthDefault   = 1920
	VideoHeightDefault  = 1080
	VideoRateDefault    = 1500
	AudioRateDefault    = 64
	FrameRateDefault    = 30
	VideoPresetDefault  = H264PresetFast
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
	VideoPreset  H264Preset
	OutputFormat AVFormat
}

func (p H264Preset) IsValid() bool {
	switch p {
	case H264PresetMedium, H264PresetFast, H264PresetFaster, H264PresetVeryFast, H264PresetSuperFast, H264PresetUltraFast:
		return true
	default:
		return false
	}
}

func (cfg RecorderConfig) IsValid() error {
	if cfg == (RecorderConfig{}) {
		return fmt.Errorf("config cannot be empty")
	}
	if cfg.SiteURL == "" {
		return fmt.Errorf("SiteURL cannot be empty")
	}

	u, err := url.Parse(cfg.SiteURL)
	if err != nil {
		return fmt.Errorf("SiteURL parsing failed: %w", err)
	} else if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("SiteURL parsing failed: invalid scheme %q", u.Scheme)
	} else if u.Path != "" {
		return fmt.Errorf("SiteURL parsing failed: invalid path %q", u.Path)
	}

	if cfg.CallID == "" {
		return fmt.Errorf("CallID cannot be empty")
	} else if !idRE.MatchString(cfg.CallID) {
		return fmt.Errorf("CallID parsing failed")
	}

	if cfg.ThreadID == "" {
		return fmt.Errorf("ThreadID cannot be empty")
	} else if !idRE.MatchString(cfg.ThreadID) {
		return fmt.Errorf("ThreadID parsing failed")
	}

	if cfg.AuthToken == "" {
		return fmt.Errorf("AuthToken cannot be empty")
	} else if !idRE.MatchString(cfg.AuthToken) {
		return fmt.Errorf("AuthToken parsing failed")
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
	if !cfg.VideoPreset.IsValid() {
		return fmt.Errorf("VideoPreset value is not valid")
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

	if cfg.VideoPreset == "" {
		cfg.VideoPreset = VideoPresetDefault
	}
}

func (cfg RecorderConfig) ToEnv() []string {
	return []string{
		fmt.Sprintf("SITE_URL=%s", cfg.SiteURL),
		fmt.Sprintf("CALL_ID=%s", cfg.CallID),
		fmt.Sprintf("THREAD_ID=%s", cfg.ThreadID),
		fmt.Sprintf("AUTH_TOKEN=%s", cfg.AuthToken),
		fmt.Sprintf("WIDTH=%d", cfg.Width),
		fmt.Sprintf("HEIGHT=%d", cfg.Height),
		fmt.Sprintf("VIDEO_RATE=%d", cfg.VideoRate),
		fmt.Sprintf("AUDIO_RATE=%d", cfg.AudioRate),
		fmt.Sprintf("FRAME_RATE=%d", cfg.FrameRate),
		fmt.Sprintf("VIDEO_PRESET=%s", cfg.VideoPreset),
		fmt.Sprintf("OUTPUT_FORMAT=%s", cfg.OutputFormat),
	}
}

func (cfg RecorderConfig) ToMap() map[string]any {
	return map[string]any{
		"site_url":      cfg.SiteURL,
		"call_id":       cfg.CallID,
		"thread_id":     cfg.ThreadID,
		"auth_token":    cfg.AuthToken,
		"width":         cfg.Width,
		"height":        cfg.Height,
		"video_rate":    cfg.VideoRate,
		"audio_rate":    cfg.AudioRate,
		"frame_rate":    cfg.FrameRate,
		"video_preset":  cfg.VideoPreset,
		"output_format": cfg.OutputFormat,
	}
}

func (cfg *RecorderConfig) FromMap(m map[string]any) *RecorderConfig {
	cfg.SiteURL, _ = m["site_url"].(string)
	cfg.CallID, _ = m["call_id"].(string)
	cfg.ThreadID, _ = m["thread_id"].(string)
	cfg.AuthToken, _ = m["auth_token"].(string)
	cfg.Width, _ = m["width"].(int)
	cfg.Height, _ = m["height"].(int)
	cfg.VideoRate, _ = m["video_rate"].(int)
	cfg.AudioRate, _ = m["audio_rate"].(int)
	cfg.FrameRate, _ = m["frame_rate"].(int)
	cfg.VideoPreset, _ = m["video_preset"].(H264Preset)
	cfg.OutputFormat, _ = m["output_format"].(AVFormat)
	return cfg
}

func LoadFromEnv() (RecorderConfig, error) {
	var cfg RecorderConfig
	cfg.SiteURL = strings.TrimSuffix(os.Getenv("SITE_URL"), "/")
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

	if val := os.Getenv("VIDEO_PRESET"); val != "" {
		cfg.VideoPreset = H264Preset(val)
	}

	if val := os.Getenv("OUTPUT_FORMAT"); val != "" {
		cfg.OutputFormat = AVFormat(val)
	}

	return cfg, nil
}
