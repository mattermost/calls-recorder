package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

type middleware func(w http.ResponseWriter, r *http.Request) bool

func TestUploadRecording(t *testing.T) {
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

	cfg := RecorderConfig{
		SiteURL:   ts.URL,
		CallID:    "test-call-id",
		ThreadID:  "test-thread-id",
		AuthToken: "test-auth-token",
	}
	cfg.SetDefaults()
	rec, err := NewRecorder(cfg)
	require.NoError(t, err)
	require.NotNil(t, rec)

	recFile, err := os.CreateTemp("", "recording.mp4")
	require.NoError(t, err)
	defer os.Remove(recFile.Name())

	t.Run("missing file", func(t *testing.T) {
		err := rec.uploadRecording()
		require.EqualError(t, err, "failed to open file: open : no such file or directory")
	})

	rec.outPath = recFile.Name()

	t.Run("upload creation failure", func(t *testing.T) {
		middlewares = []middleware{
			func(w http.ResponseWriter, r *http.Request) bool {
				if r.URL.Path == "/plugins/com.mattermost.calls/bot/uploads" && r.Method == http.MethodPost {
					w.WriteHeader(400)
					fmt.Fprintln(w, `{"message": "server error"}`)
					return true
				}

				return false
			},
		}
		err := rec.uploadRecording()
		require.EqualError(t, err, "failed to create upload (400): : server error, ")
	})

	t.Run("upload data failure", func(t *testing.T) {
		middlewares = []middleware{
			func(w http.ResponseWriter, r *http.Request) bool {
				if r.URL.Path == "/plugins/com.mattermost.calls/bot/uploads" && r.Method == http.MethodPost {
					fmt.Fprintln(w, `{"id": "uploadID"}`)
					return true
				}

				return false
			},
			func(w http.ResponseWriter, r *http.Request) bool {
				if r.URL.Path == "/plugins/com.mattermost.calls/bot/uploads/uploadID" && r.Method == http.MethodPost {
					w.WriteHeader(400)
					fmt.Fprintln(w, `{"message": "server error"}`)
					return true
				}

				return false
			},
		}
		err := rec.uploadRecording()
		require.EqualError(t, err, "failed to upload data (400): : server error, ")
	})

	t.Run("save recording failure", func(t *testing.T) {
		middlewares = []middleware{
			func(w http.ResponseWriter, r *http.Request) bool {
				if r.URL.Path == "/plugins/com.mattermost.calls/bot/uploads" && r.Method == http.MethodPost {
					fmt.Fprintln(w, `{"id": "uploadID"}`)
					return true
				}

				return false
			},
			func(w http.ResponseWriter, r *http.Request) bool {
				if r.URL.Path == "/plugins/com.mattermost.calls/bot/uploads/uploadID" && r.Method == http.MethodPost {
					fmt.Fprintln(w, `{"id": "fileID"}`)
					return true
				}

				return false
			},
			func(w http.ResponseWriter, r *http.Request) bool {
				if r.URL.Path == "/plugins/com.mattermost.calls/bot/calls/test-call-id/recordings" && r.Method == http.MethodPost {
					w.WriteHeader(400)
					fmt.Fprintln(w, `{"message": "server error"}`)
					return true
				}

				return false
			},
		}
		err := rec.uploadRecording()
		require.EqualError(t, err, "failed to save recording (400): : server error, ")
	})

	t.Run("success", func(t *testing.T) {
		middlewares = middlewares[:len(middlewares)-1]
		middlewares = append(middlewares, func(w http.ResponseWriter, r *http.Request) bool {
			if r.URL.Path == "/plugins/com.mattermost.calls/bot/calls/test-call-id/recordings" && r.Method == http.MethodPost {
				w.WriteHeader(200)
				return true
			}
			return false
		})
		err := rec.uploadRecording()
		require.NoError(t, err)
	})
}
