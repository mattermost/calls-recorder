package main

import (
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/mattermost/calls-recorder/cmd/recorder/config"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		AddSource:   true,
		Level:       slog.LevelDebug,
		ReplaceAttr: slogReplaceAttr,
	}))
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

	recorder, err := NewRecorder(cfg)
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

		os.Exit(1)
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
