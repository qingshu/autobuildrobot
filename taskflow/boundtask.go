package taskflow

import (
	"context"
	"errors"
)

type BoundTask struct {
	task Task
	data any
}

func (bt BoundTask) Name() string {
	return bt.task.Name()
}

// 这里用自身的参数，不用源头传的参数
func (bt BoundTask) Run(ctx context.Context, _ any) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if bt.task == nil {
		return errors.New("bound task cannot be nil")
	}
	return bt.task.Run(ctx, bt.data)
}

func BindTask(task Task, data any) Task {
	return BoundTask{task: task, data: data}
}
