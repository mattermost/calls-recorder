package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/mattermost/calls-recorder/cmd/recorder/config"
	"github.com/mattermost/mattermost-plugin-calls/server/public"

	"github.com/stretchr/testify/require"
)

func TestReportJobFailure(t *testing.T) {
	middlewares := []middleware{}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		for _, mw := range middlewares {
			if mw(w, r) {
				return
			}
		}
		http.NotFound(w, r)
	}))
	defer ts.Close()

	cfg := config.RecorderConfig{
		SiteURL:     ts.URL,
		CallID:      "8w8jorhr7j83uqr6y1st894hqe",
		PostID:      "udzdsg7dwidbzcidx5khrf8nee",
		RecordingID: "67t5u6cmtfbb7jug739d43xa9e",
		AuthToken:   "qj75unbsef83ik9p7ueypb6iyw",
	}
	cfg.SetDefaults()
	rec, err := NewRecorder(cfg, getDataDir(""))
	require.NoError(t, err)
	require.NotNil(t, rec)

	t.Run("request failure", func(t *testing.T) {
		middlewares = []middleware{
			func(w http.ResponseWriter, r *http.Request) bool {
				if r.URL.Path != "/plugins/com.mattermost.calls/bot/calls/8w8jorhr7j83uqr6y1st894hqe/jobs/67t5u6cmtfbb7jug739d43xa9e/status" {
					w.WriteHeader(404)
					return true
				}

				w.WriteHeader(400)
				fmt.Fprintln(w, `{"message": "server error"}`)
				return true
			},
		}
		err := rec.ReportJobFailure("")
		require.EqualError(t, err, "request failed: server error")
	})

	t.Run("success", func(t *testing.T) {
		var errMsg string
		middlewares = []middleware{
			func(w http.ResponseWriter, r *http.Request) bool {
				if r.URL.Path != "/plugins/com.mattermost.calls/bot/calls/8w8jorhr7j83uqr6y1st894hqe/jobs/67t5u6cmtfbb7jug739d43xa9e/status" {
					w.WriteHeader(404)
					return true
				}

				var status public.JobStatus
				if err := json.NewDecoder(r.Body).Decode(&status); err != nil {
					w.WriteHeader(400)
					fmt.Fprintf(w, `{"message": %q}`, err.Error())
					return true
				}

				if status.JobType != public.JobTypeRecording {
					w.WriteHeader(400)
					return true
				}

				if status.Status != public.JobStatusTypeFailed {
					w.WriteHeader(400)
					return true
				}

				errMsg = status.Error

				w.WriteHeader(200)
				return true
			},
		}
		err := rec.ReportJobFailure("some error")
		require.Nil(t, err)
		require.Equal(t, "some error", errMsg)
	})
}
