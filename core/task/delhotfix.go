package task

import (
	"autobuildrobot/models"
	"autobuildrobot/taskflow"
	"context"
	"errors"
)

func getDelHotfixResTask(p *Param) (taskflow.Task, error) {
	if p == nil || p.CmdType != models.CommandType_DelHotfixRes {
		return nil, errors.New("无效的删除资源指令")
	}
	return taskflow.Func{Title: models.GetCommandNameByType(models.CommandType_DelHotfixRes), Action: func(ctx context.Context, _ any) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		return errors.New("谨慎考虑，暂不实现删除资源")
	}}, nil
}
