package task

import (
	"autobuildrobot/taskflow"
	"context"
	"errors"
	"strings"
)

func getStopTask(param *Param) (taskflow.Task, error) {
	if param == nil || len(param.CmdParams) <= 0 {
		return nil, errors.New("param is empty,要传入需要停止的构建id")
	}
	taskIds := param.CmdParams
	return taskflow.Func{Title: "停止构建",
		Action: func(ctx context.Context, a any) error {
			for _, taskId := range taskIds {
				taskId = strings.TrimSpace(taskId)
				if taskId == "" {
					continue
				}
				if err := engine.Cancel(taskId); err != nil {
					return err
				}
			}
			return nil
		},
	}, nil
}
