package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/mattermost/mattermost-plugin-calls/server/public"
)

const (
	postJobStatusMaxRetries = 2
	postJobStatusRetryDelay = 2 * time.Second
)

func (rec *Recorder) postJobStatus(status public.JobStatus) error {
	apiURL := fmt.Sprintf("%s/plugins/%s/bot/calls/%s/jobs/%s/status",
		rec.client.URL, pluginID, rec.cfg.CallID, rec.cfg.RecordingID)

	payload, err := json.Marshal(&status)
	if err != nil {
		return fmt.Errorf("failed to marshal: %w", err)
	}

	var lastErr error
	for attempt := 0; attempt <= postJobStatusMaxRetries; attempt++ {
		if attempt > 0 {
			slog.Warn("postJobStatus failed, retrying", slog.Int("attempt", attempt), slog.String("err", lastErr.Error()))
			time.Sleep(postJobStatusRetryDelay)
		}
		ctx, cancelCtx := context.WithTimeout(context.Background(), httpRequestTimeout)
		resp, err := rec.client.DoAPIRequestBytes(ctx, http.MethodPost, apiURL, payload, "")
		cancelCtx()
		if err == nil {
			resp.Body.Close()
			return nil
		}
		lastErr = err
	}

	return fmt.Errorf("request failed: %w", lastErr)
}

func (rec *Recorder) ReportJobFailure(errMsg string) error {
	return rec.postJobStatus(public.JobStatus{
		JobType: public.JobTypeRecording,
		Status:  public.JobStatusTypeFailed,
		Error:   errMsg,
	})
}

func (rec *Recorder) ReportJobStarted() error {
	return rec.postJobStatus(public.JobStatus{
		JobType: public.JobTypeRecording,
		Status:  public.JobStatusTypeStarted,
	})
}
