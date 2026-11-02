package taskflow

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
)

var TaskDirLocks = &DirectoryLocks{} // 多个 Engine 也应共享此实例。

type directoryEntry struct {
	gate chan struct{}
	refs int // holder plus waiting acquisitions
}

// DirectoryLocks coordinates exact workspace roots within this process.
// Share one instance across engines. The zero value is ready to use.
// Parent/child directories are different resources; use one workspace root.
type DirectoryLocks struct {
	mu      sync.Mutex
	entries map[string]*directoryEntry
}

// Wrap resolves an existing directory and protects the entire task, including
// nested steps. Do not wrap its children with the same directory lock again.
// The directory and its aliases must remain stable during task execution.
func (l *DirectoryLocks) Wrap(directory string, task Task) (Task, error) {
	if task == nil {
		return nil, fmt.Errorf("directory lock: nil task")
	}
	if strings.TrimSpace(directory) == "" {
		return nil, fmt.Errorf("directory lock: empty directory")
	}
	abs, err := filepath.Abs(directory)
	if err != nil {
		return nil, err
	}
	resolved, err := filepath.EvalSymlinks(abs)
	if err != nil {
		return nil, fmt.Errorf("resolve directory: %w", err)
	}
	info, err := os.Stat(resolved)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("not a directory: %s", directory)
	}
	key := filepath.Clean(resolved)
	// Conservative serialization on Windows, including case-sensitive folders.
	if runtime.GOOS == "windows" {
		key = strings.ToLower(key)
	}
	return directoryTask{locks: l, key: key, task: task}, nil
}

func (l *DirectoryLocks) acquire(ctx context.Context, key string) (func(), error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	l.mu.Lock()
	if l.entries == nil {
		l.entries = make(map[string]*directoryEntry)
	}
	entry := l.entries[key]
	if entry == nil {
		entry = &directoryEntry{gate: make(chan struct{}, 1)}
		l.entries[key] = entry
	}
	entry.refs++
	l.mu.Unlock()

	select {
	case entry.gate <- struct{}{}:
		if err := ctx.Err(); err != nil {
			<-entry.gate
			l.drop(key, entry)
			return nil, err
		}
		var once sync.Once
		return func() {
			once.Do(func() {
				<-entry.gate
				l.drop(key, entry)
			})
		}, nil
	case <-ctx.Done():
		l.drop(key, entry)
		return nil, ctx.Err()
	}
}

func (l *DirectoryLocks) drop(key string, entry *directoryEntry) {
	l.mu.Lock()
	defer l.mu.Unlock()
	entry.refs--
	if entry.refs == 0 {
		delete(l.entries, key)
	}
}

type directoryTask struct {
	locks *DirectoryLocks
	key   string
	task  Task
}

func (t directoryTask) Name() string { return t.task.Name() }
func (t directoryTask) Run(ctx context.Context, data any) error {
	release, err := t.locks.acquire(ctx, t.key)
	if err != nil {
		return err
	}
	defer release() // Also release during panic unwinding.
	return t.task.Run(ctx, data)
}
