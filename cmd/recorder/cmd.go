package main

import (
	"io"
	"log/slog"
	"os/exec"
	"strings"
)

const (
	logBufferSize = 1024 * 64 // 64KB
)

func runCmd(cmd string, args string) (*exec.Cmd, error) {
	slog.Debug("running cmd", slog.String("cmd", cmd), slog.String("args", args))
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
				slog.Debug("error reading log buffer",
					slog.String("cmd", cmd),
					slog.String("name", name),
					slog.String("err", err.Error()),
				)
				return
			}
			slog.Debug(strings.TrimSuffix(string(buf[:n]), "\n"),
				slog.String("cmd", cmd),
				slog.String("name", name),
			)
		}
	}

	go logOutput(stdout, "stdout")
	go logOutput(stderr, "stderr")

	return c, nil
}
