package processrun

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestHelperProcess(t *testing.T) {
	for i, arg := range os.Args {
		if arg != "--processrun-helper" {
			continue
		}
		switch os.Args[i+1] {
		case "output":
			fmt.Fprintln(os.Stdout, "stdout line")
			fmt.Fprint(os.Stderr, "stderr without newline")
			os.Exit(7)
		case "wait":
			for {
				time.Sleep(time.Second)
			}
		case "args":
			fmt.Fprint(os.Stdout, os.Args[i+2])
			os.Exit(0)
		case "child":
			time.Sleep(time.Second)
			_ = os.WriteFile(os.Args[i+2], []byte("child survived cancellation"), 0600)
			for {
				time.Sleep(time.Second)
			}
		case "tree":
			child := exec.Command(os.Args[0], append(helperArgs("child"), os.Args[i+2])...)
			if err := child.Start(); err != nil {
				os.Exit(9)
			}
			fmt.Fprintln(os.Stdout, "child started")
			for {
				time.Sleep(time.Second)
			}
		}
	}
}

func TestCancellationStopsDescendant(t *testing.T) {
	marker := filepath.Join(t.TempDir(), "survivor")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err := Run(ctx, "", os.Args[0], append(helperArgs("tree"), marker), func(line string) {
		if strings.Contains(line, "child started") {
			cancel()
		}
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("cancel: %v", err)
	}
	time.Sleep(1200 * time.Millisecond)
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatal("descendant survived cancellation")
	}
}

func helperArgs(mode string) []string {
	return []string{"-test.run=^TestHelperProcess$", "--", "--processrun-helper", mode}
}

func TestOutputAndExitCode(t *testing.T) {
	var output strings.Builder
	err := Run(context.Background(), "", os.Args[0], helperArgs("output"), func(s string) { output.WriteString(s) })
	var exit *exec.ExitError
	if !errors.As(err, &exit) || exit.ExitCode() != 7 {
		t.Fatalf("exit: %v", err)
	}
	if !strings.Contains(output.String(), "stdout line") || !strings.Contains(output.String(), "stderr without newline") {
		t.Fatalf("lost output: %q", output.String())
	}
}

func TestCancellation(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()
	err := Run(ctx, "", os.Args[0], helperArgs("wait"), nil)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("got %v", err)
	}
}

func TestLiteralArgumentsAndStartFailure(t *testing.T) {
	want := "a path with spaces; $(echo injected) & \"quote\""
	var got strings.Builder
	err := Run(context.Background(), "", os.Args[0], append(helperArgs("args"), want), func(s string) { got.WriteString(s) })
	if err != nil || got.String() != want {
		t.Fatalf("output=%q error=%v", got.String(), err)
	}
	if err := Run(context.Background(), "", "nonexistent-taskflow-test-executable", nil, nil); err == nil {
		t.Fatal("missing start error")
	}
}
