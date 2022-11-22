package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)

	cfg, err := loadConfig()
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
