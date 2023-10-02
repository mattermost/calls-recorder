package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/mattermost/calls-recorder/cmd/recorder/config"
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)

	pid := os.Getpid()
	if err := os.WriteFile("/tmp/recorder.pid", []byte(fmt.Sprintf("%d", pid)), 0666); err != nil {
		log.Fatalf("failed to write pid file: %s", err)
	}

	cfg, err := config.LoadFromEnv()
	if err != nil {
		log.Fatalf("failed to load config: %s", err)
	}
	cfg.SetDefaults()

	recorder, err := NewRecorder(cfg)
	if err != nil {
		log.Fatalf("failed to create recorder: %s", err)
	}

	log.Printf("starting recordinig")

	if err := recorder.Start(); err != nil {
		if err := recorder.ReportJobFailure(err.Error()); err != nil {
			log.Printf("failed to report job failure: %s", err.Error())
		}
		log.Fatalf("failed to start recording: %s", err)
	}

	log.Printf("recording has started")

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)
	<-sig

	log.Printf("received SIGTERM, stopping recording")

	if err := recorder.Stop(); err != nil {
		log.Fatalf("failed to stop recording: %s", err)
	}

	log.Printf("recording has finished, exiting")
}
