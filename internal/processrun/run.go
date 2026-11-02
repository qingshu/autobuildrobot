// Package processrun runs a command with merged streaming output and cancellation.
package processrun

import (
	"context"
	"errors"
	"io"
	"os/exec"
	"strings"
	"sync"
	"time"
)

func Run(ctx context.Context, dir, executable string, args []string, output func(string)) error {
	cmd := exec.CommandContext(ctx, executable, args...)
	cmd.Dir = dir
	configureCancellation(cmd)
	cmd.WaitDelay = 5 * time.Second
	w := &lineWriter{output: output}
	cmd.Stdout, cmd.Stderr = w, w
	err := cmd.Run()
	w.flush()
	if ctx.Err() != nil {
		return errors.Join(ctx.Err(), err)
	}
	return err
}

type lineWriter struct {
	mu      sync.Mutex
	pending string
	output  func(string)
}

func (w *lineWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.pending += string(p)
	for {
		i := strings.IndexByte(w.pending, '\n')
		if i < 0 {
			break
		}
		w.emit(w.pending[:i+1])
		w.pending = w.pending[i+1:]
	}
	// Bound memory for commands emitting extremely long lines.
	if len(w.pending) >= 64*1024 {
		w.emit(w.pending)
		w.pending = ""
	}
	return len(p), nil
}
func (w *lineWriter) emit(s string) {
	if w.output != nil {
		w.output(s)
	}
}
func (w *lineWriter) flush() {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.pending != "" {
		w.emit(w.pending)
		w.pending = ""
	}
}

var _ io.Writer = (*lineWriter)(nil)
