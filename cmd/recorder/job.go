package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/mattermost/mattermost-plugin-calls/server/public"
)

func (rec *Recorder) postJobStatus(status public.JobStatus) error {
	apiURL := fmt.Sprintf("%s/plugins/%s/bot/calls/%s/jobs/%s/status",
		rec.client.URL, pluginID, rec.cfg.CallID, rec.cfg.RecordingID)

	payload, err := json.Marshal(&status)
	if err != nil {
		return fmt.Errorf("failed to marshal: %w", err)
	}

	ctx, cancelCtx := context.WithTimeout(context.Background(), httpRequestTimeout)
	defer cancelCtx()
	resp, err := rec.client.DoAPIRequestBytes(ctx, http.MethodPost, apiURL, payload, "")
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()
	cancelCtx()

	return nil
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
