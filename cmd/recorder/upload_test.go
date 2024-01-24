package main

import (
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/mattermost/calls-recorder/cmd/recorder/config"

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

	cfg := config.RecorderConfig{
		SiteURL:     ts.URL,
		CallID:      "8w8jorhr7j83uqr6y1st894hqe",
		PostID:      "udzdsg7dwidbzcidx5khrf8nee",
		RecordingID: "67t5u6cmtfbb7jug739d43xa9e",
		AuthToken:   "qj75unbsef83ik9p7ueypb6iyw",
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

	t.Run("invalid response", func(t *testing.T) {
		middlewares = []middleware{
			func(w http.ResponseWriter, r *http.Request) bool {
				if r.URL.Path == "/plugins/com.mattermost.calls/bot/uploads" && r.Method == http.MethodPost {
					w.WriteHeader(500)
					fmt.Fprintln(w, `Internal Server Error`)
					return true
				}

				return false
			},
		}
		err := rec.uploadRecording()
		require.EqualError(t, err, "failed to create upload: AppErrorFromJSON: model.utils.decode_json.app_error, body: Internal Server Error\n, invalid character 'I' looking for beginning of value")
	})

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
		require.EqualError(t, err, "failed to create upload: : server error")
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
		require.EqualError(t, err, "failed to upload data: : server error")
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
				if r.URL.Path == "/plugins/com.mattermost.calls/bot/calls/8w8jorhr7j83uqr6y1st894hqe/recordings" && r.Method == http.MethodPost {
					w.WriteHeader(400)
					fmt.Fprintln(w, `{"message": "server error"}`)
					return true
				}

				return false
			},
		}
		err := rec.uploadRecording()
		require.EqualError(t, err, "failed to save recording: : server error")
	})

	t.Run("success", func(t *testing.T) {
		middlewares = middlewares[:len(middlewares)-1]
		middlewares = append(middlewares, func(w http.ResponseWriter, r *http.Request) bool {
			if r.URL.Path == "/plugins/com.mattermost.calls/bot/calls/8w8jorhr7j83uqr6y1st894hqe/recordings" && r.Method == http.MethodPost {
				w.WriteHeader(200)
				return true
			}
			return false
		})
		err := rec.uploadRecording()
		require.NoError(t, err)
	})
}

func TestPublishRecording(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		AddSource: true,
		Level:     slog.LevelDebug,
	}))
	slog.SetDefault(logger)

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
	rec, err := NewRecorder(cfg)
	require.NoError(t, err)
	require.NotNil(t, rec)

	recFile, err := os.CreateTemp("", "recording.mp4")
	require.NoError(t, err)
	defer os.Remove(recFile.Name())

	rec.outPath = recFile.Name()

	uploadRetryAttemptWaitTime = time.Second

	t.Run("success after multiple failed attempts", func(t *testing.T) {
		var failures int
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
				if r.URL.Path == "/plugins/com.mattermost.calls/bot/calls/8w8jorhr7j83uqr6y1st894hqe/recordings" && r.Method == http.MethodPost {
					if failures < 5 {
						w.WriteHeader(400)
						fmt.Fprintln(w, `{"message": "server error"}`)
						failures++
					} else {
						w.WriteHeader(200)
					}
					return true
				}
				return false
			},
		}

		err := rec.publishRecording()
		require.NoError(t, err)
	})

	t.Run("failure after maximum attempts reached", func(t *testing.T) {
		uploadRetryAttemptWaitTime = 10 * time.Millisecond

		middlewares[1] = func(w http.ResponseWriter, r *http.Request) bool {
			if r.URL.Path == "/plugins/com.mattermost.calls/bot/uploads/uploadID" && r.Method == http.MethodPost {
				w.WriteHeader(500)
				fmt.Fprintln(w, `{"message": "server error"}`)
				return true
			}

			return false
		}

		err := rec.publishRecording()
		require.EqualError(t, err, "max retry attempts reached, exiting")
	})

	t.Run("success", func(t *testing.T) {
		middlewares[1] = func(w http.ResponseWriter, r *http.Request) bool {
			if r.URL.Path == "/plugins/com.mattermost.calls/bot/uploads/uploadID" && r.Method == http.MethodPost {
				fmt.Fprintln(w, `{"id": "fileID"}`)
				return true
			}
			return false
		}

		middlewares[2] = func(w http.ResponseWriter, r *http.Request) bool {
			if r.URL.Path == "/plugins/com.mattermost.calls/bot/calls/8w8jorhr7j83uqr6y1st894hqe/recordings" && r.Method == http.MethodPost {
				w.WriteHeader(200)
				return true
			}
			return false
		}

		err := rec.publishRecording()
		require.NoError(t, err)
	})
}
