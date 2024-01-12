package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/mattermost/mattermost-plugin-calls/server/public"

	"github.com/mattermost/mattermost/server/public/model"
)

const (
	httpRequestTimeout = 10 * time.Second
	httpUploadTimeout  = 5 * time.Minute
)

var (
	maxUploadRetryAttempts     = 20
	uploadRetryAttemptWaitTime = 5 * time.Second
)

func (rec *Recorder) publishRecording() error {
	var attempt int
	for {
		err := rec.uploadRecording()
		if err == nil {
			slog.Info("recording uploaded successfully")
			break
		}

		if attempt == maxUploadRetryAttempts {
			return fmt.Errorf("max retry attempts reached, exiting")
		}

		attempt++
		slog.Info("failed to upload recording", slog.String("err", err.Error()))

		waitTime := uploadRetryAttemptWaitTime * time.Duration(attempt)
		slog.Info("retrying", slog.Duration("wait_time", waitTime))
		time.Sleep(waitTime)
	}

	return nil
}

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

	apiURL := fmt.Sprintf("%s/plugins/%s/bot", rec.client.URL, pluginID)

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
	resp, err := rec.client.DoAPIRequestBytes(ctx, http.MethodPost, apiURL+"/uploads", payload, "")
	if err != nil {
		return fmt.Errorf("failed to create upload: %w", err)
	}
	defer resp.Body.Close()
	cancelCtx()

	if err := json.NewDecoder(resp.Body).Decode(&us); err != nil {
		return fmt.Errorf("failed to decode response body: %w", err)
	}

	ctx, cancelCtx = context.WithTimeout(context.Background(), httpUploadTimeout)
	defer cancelCtx()
	resp, err = rec.client.DoAPIRequestReader(ctx, http.MethodPost, apiURL+"/uploads/"+us.Id, file, nil)
	if err != nil {
		return fmt.Errorf("failed to upload data: %w", err)
	}
	defer resp.Body.Close()
	cancelCtx()

	// TODO (MM-48545): handle upload resumption.

	var fi model.FileInfo
	if err := json.NewDecoder(resp.Body).Decode(&fi); err != nil {
		return fmt.Errorf("failed to decode response body: %w", err)
	}

	payload, err = json.Marshal(public.JobInfo{
		JobID:   rec.cfg.RecordingID,
		FileIDs: []string{fi.Id},
		PostID:  rec.cfg.PostID,
	})
	if err != nil {
		return fmt.Errorf("failed to encode payload: %w", err)
	}

	url := fmt.Sprintf("%s/calls/%s/recordings", apiURL, rec.cfg.CallID)
	ctx, cancelCtx = context.WithTimeout(context.Background(), httpRequestTimeout)
	defer cancelCtx()
	resp, err = rec.client.DoAPIRequestBytes(ctx, http.MethodPost, url, payload, "")
	if err != nil {
		return fmt.Errorf("failed to save recording: %w", err)
	}
	defer resp.Body.Close()

	return nil
}
