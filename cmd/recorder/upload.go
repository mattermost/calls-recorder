package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/mattermost/mattermost/server/public/model"
)

const (
	httpRequestTimeout = 10 * time.Second
	httpUploadTimeout  = 5 * time.Minute
)

func (rec *Recorder) uploadRecording() error {
	file, err := os.Open(rec.outPath)
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		return fmt.Errorf("failed to stat file: %w", err)
	}

	client := model.NewAPIv4Client(rec.cfg.SiteURL)
	client.SetToken(rec.cfg.AuthToken)
	apiURL := fmt.Sprintf("%s/plugins/%s/bot", client.URL, pluginID)

	us := &model.UploadSession{
		ChannelId: rec.cfg.CallID,
		Filename:  filepath.Base(rec.outPath),
		FileSize:  info.Size(),
	}

	payload, err := json.Marshal(us)
	if err != nil {
		return fmt.Errorf("failed to encode payload: %w", err)
	}

	ctx, cancelCtx := context.WithTimeout(context.Background(), httpRequestTimeout)
	defer cancelCtx()
	resp, err := client.DoAPIRequestBytes(ctx, http.MethodPost, apiURL+"/uploads", payload, "")
	defer resp.Body.Close()
	if err != nil {
		return fmt.Errorf("failed to create upload (%d): %w", resp.StatusCode, err)
	}
	cancelCtx()

	if err := json.NewDecoder(resp.Body).Decode(&us); err != nil {
		return fmt.Errorf("failed to decode response body: %w", err)
	}

	ctx, cancelCtx = context.WithTimeout(context.Background(), httpUploadTimeout)
	defer cancelCtx()
	resp, err = client.DoAPIRequestReader(ctx, http.MethodPost, apiURL+"/uploads/"+us.Id, file, nil)
	defer resp.Body.Close()
	if err != nil {
		return fmt.Errorf("failed to upload data (%d): %w", resp.StatusCode, err)
	}
	cancelCtx()

	// TODO (MM-48545): handle upload resumption.

	var fi model.FileInfo
	if err := json.NewDecoder(resp.Body).Decode(&fi); err != nil {
		return fmt.Errorf("failed to decode response body: %w", err)
	}

	payload, err = json.Marshal(map[string]string{
		"file_id":   fi.Id,
		"thread_id": rec.cfg.ThreadID,
	})
	if err != nil {
		return fmt.Errorf("failed to encode payload: %w", err)
	}

	url := fmt.Sprintf("%s/calls/%s/recordings", apiURL, rec.cfg.CallID)
	ctx, cancelCtx = context.WithTimeout(context.Background(), httpRequestTimeout)
	defer cancelCtx()
	resp, err = client.DoAPIRequestBytes(ctx, http.MethodPost, url, payload, "")
	defer resp.Body.Close()
	if err != nil {
		return fmt.Errorf("failed to save recording (%d): %w", resp.StatusCode, err)
	}

	return nil
}
