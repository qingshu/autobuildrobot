// Package taskflow provides an in-process task queue and worker pool.
package taskflow

import (
	"context"
	"fmt"
)

type Task interface {
	Name() string
	Run(context.Context, any) error
}

type Func struct {
	Title  string
	Action func(context.Context, any) error
}

func (f Func) Name() string { return f.Title }
func (f Func) Run(ctx context.Context, data any) error {
	if f.Action == nil {
		return fmt.Errorf("task %q has no action", f.Title)
	}
	return f.Action(ctx, data)
}

type Sequence struct {
	Title    string
	Children []Task
}

func (s Sequence) Name() string { return s.Title }
func (s Sequence) Run(ctx context.Context, data any) error {
	for _, child := range s.Children {
		if err := ctx.Err(); err != nil {
			return err
		}
		if child == nil {
			return fmt.Errorf("nil child in %q", s.Title)
		}
		if err := child.Run(ctx, data); err != nil {
			return fmt.Errorf("%s: %w", child.Name(), err)
		}
	}
	return nil
}
