package taskflow

import (
	"context"
	"errors"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestDirectorySerialization(t *testing.T) {
	var locks DirectoryLocks
	root := t.TempDir()
	var active, peak atomic.Int32
	job, err := locks.Wrap(root, Func{Title: "build", Action: func(_ context.Context, data any) error {
		if data != 42 {
			return errors.New("lost execution data")
		}
		n := active.Add(1)
		for old := peak.Load(); n > old; old = peak.Load() {
			if peak.CompareAndSwap(old, n) {
				break
			}
		}
		time.Sleep(time.Millisecond)
		active.Add(-1)
		return nil
	}})
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := job.Run(context.Background(), 42); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	if peak.Load() != 1 {
		t.Fatalf("concurrent executions: %d", peak.Load())
	}
	if len(locks.entries) != 0 {
		t.Fatal("unused locks retained")
	}
}

func TestDirectoryCancellationAndIndependentRoots(t *testing.T) {
	var locks DirectoryLocks
	root := t.TempDir()
	job, err := locks.Wrap(root, Func{Title: "first", Action: func(context.Context, any) error { return nil }})
	if err != nil {
		t.Fatal(err)
	}
	key := job.(directoryTask).key
	release, err := locks.acquire(context.Background(), key)
	if err != nil {
		t.Fatal(err)
	}
	defer release()

	// Another root can execute while the first root is held.
	other, err := locks.Wrap(t.TempDir(), Func{Title: "other", Action: func(context.Context, any) error { return nil }})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := other.Run(ctx, nil); err != nil {
		t.Fatal(err)
	}

	alias, err := locks.Wrap(filepath.Join(root, "."), Func{Title: "must not run", Action: func(context.Context, any) error {
		t.Error("canceled waiter executed")
		return nil
	}})
	if err != nil {
		t.Fatal(err)
	}
	waitCtx, stop := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer stop()
	if err := alias.Run(waitCtx, nil); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("got %v", err)
	}
	release()
	release() // Idempotent; never consume another owner's token.
	if len(locks.entries) != 0 {
		t.Fatal("lock references leaked")
	}
}

func TestDirectoryPanicReleasesLock(t *testing.T) {
	var locks DirectoryLocks
	root := t.TempDir()
	job, err := locks.Wrap(root, Func{Title: "panic", Action: func(context.Context, any) error { panic("test") }})
	if err != nil {
		t.Fatal(err)
	}
	func() {
		defer func() {
			if recover() == nil {
				t.Error("expected panic")
			}
		}()
		_ = job.Run(context.Background(), nil)
	}()
	next, err := locks.Wrap(root, Func{Title: "next", Action: func(context.Context, any) error { return nil }})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := next.Run(ctx, nil); err != nil {
		t.Fatal(err)
	}
}
