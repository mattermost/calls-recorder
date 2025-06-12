package main

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/mattermost/calls-recorder/cmd/recorder/config"
)

func main() {
	recID := os.Getenv("RECORDING_ID")

	dataPath := getDataDir(recID)

	logFile, err := os.Create(filepath.Join(dataPath, "recorder.log"))
	if err != nil {
		slog.Error("failed to create log file", slog.String("err", err.Error()))
		os.Exit(1)
	}
	defer logFile.Close()

	// This lets us write logs simultaneously to console and file.
	logWriter := io.MultiWriter(os.Stdout, logFile)

	logger := slog.New(slog.NewTextHandler(logWriter, &slog.HandlerOptions{
		AddSource:   true,
		Level:       slog.LevelDebug,
		ReplaceAttr: slogReplaceAttr,
	})).With("recID", recID)
	slog.SetDefault(logger)

	pid := os.Getpid()
	if err := os.WriteFile("/tmp/recorder.pid", []byte(fmt.Sprintf("%d", pid)), 0666); err != nil {
		slog.Error("failed to write pid file", slog.String("err", err.Error()))
		os.Exit(1)
	}

	cfg, err := config.LoadFromEnv()
	if err != nil {
		slog.Error("failed to load config", slog.String("err", err.Error()))
		os.Exit(1)
	}
	cfg.SetDefaults()

	slog.SetDefault(logger.With("jobID", cfg.RecordingID))

	recorder, err := NewRecorder(cfg, dataPath)
	if err != nil {
		slog.Error("failed to create recorder", slog.String("err", err.Error()))
		os.Exit(1)
	}

	slog.Info("starting recording")

	if err := recorder.Start(); err != nil {
		slog.Error("failed to start recording", slog.String("err", err.Error()))
		if err := recorder.ReportJobFailure(err.Error()); err != nil {
			slog.Error("failed to report job failure", slog.String("err", err.Error()))
		}

		// cleaning up
		if err := recorder.Stop(); err != nil {
			slog.Error("failed to stop recorder", slog.String("err", err.Error()))
		}

		// Although an error case, if we fail to start we are not losing any
		// recording data so the associated resources (e.g. container, volume) can be safely deleted.
		// This is signaled to the calling layer (calls-offloader) by exiting with
		// a success code.
		os.Exit(0)
	}

	slog.Info("recording has started")

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)
	<-sig

	slog.Info("received SIGTERM, stopping recording")

	if err := recorder.Stop(); err != nil {
		slog.Error("failed to stop recording", slog.String("err", err.Error()))
		os.Exit(1)
	}

	slog.Info("recording has finished, exiting")
}
