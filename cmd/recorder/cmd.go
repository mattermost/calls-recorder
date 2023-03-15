package main

import (
	"io"
	"log"
	"os/exec"
	"strings"
)

const (
	logBufferSize = 1024 * 64 // 64KB
)

func runCmd(cmd string, args string) (*exec.Cmd, error) {
	log.Printf("running %s: %q", cmd, args)
	c := exec.Command(cmd, strings.Split(args, " ")...)

	stdout, err := c.StdoutPipe()
	if err != nil {
		return nil, err
	}
	stderr, err := c.StderrPipe()
	if err != nil {
		return nil, err
	}

	if err := c.Start(); err != nil {
		return nil, err
	}

	logOutput := func(out io.ReadCloser, name string) {
		defer out.Close()
		buf := make([]byte, logBufferSize)
		for {
			n, err := out.Read(buf)
			if err == io.EOF {
				return
			}
			if err != nil {
				log.Printf("%s (%s): error reading: %s", cmd, name, err)
				return
			}
			log.Printf("%s (%s): %s", cmd, name, strings.TrimSuffix(string(buf[:n]), "\n"))
		}
	}

	go logOutput(stdout, "stdout")
	go logOutput(stderr, "stderr")

	return c, nil
}
