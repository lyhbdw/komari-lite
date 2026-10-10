// Package boundedexec bounds optional probes and update smoke tests.
package boundedexec

import (
	"bytes"
	"context"
	"errors"
	"os/exec"
	"time"
)

var ErrOutputLimit = errors.New("command output limit exceeded")

type cappedOutput struct {
	buffer   bytes.Buffer
	limit    int
	exceeded bool
	cancel   context.CancelFunc
}

func (b *cappedOutput) Write(p []byte) (int, error) {
	remaining := b.limit - b.buffer.Len()
	if len(p) > remaining {
		_, _ = b.buffer.Write(p[:remaining])
		b.exceeded = true
		b.cancel()
		return remaining, ErrOutputLimit
	}
	return b.buffer.Write(p)
}

func Run(parent context.Context, timeout time.Duration, limit int, name string, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(parent, timeout)
	defer cancel()
	output := &cappedOutput{limit: limit, cancel: cancel}
	command := exec.CommandContext(ctx, name, args...)
	// Do not wait indefinitely for descendants that inherited output pipes.
	command.WaitDelay = 100 * time.Millisecond
	command.Stdout, command.Stderr = output, output
	err := command.Run()
	if output.exceeded {
		err = ErrOutputLimit
	} else if ctx.Err() != nil {
		err = ctx.Err()
	}
	return output.buffer.Bytes(), err
}
