package task

import (
	log "autobuildrobot/mylog"
	"autobuildrobot/taskflow"
	"autobuildrobot/tool"
	"context"
	"errors"
	"fmt"
)

func withCommandNotifications(job taskflow.Task, display string, index, total int, builder, phone string, send SendResultFunc) taskflow.Task {
	if send == nil {
		return job
	}
	return taskflow.Func{Title: job.Name(), Action: func(ctx context.Context, data any) (err error) {
		if err := ctx.Err(); err != nil {
			return err
		}
		send(fmt.Sprintf("builder:%s\ninfo:开始执行%d/%d指令：【%s】...", builder, index, total, display), "")
		defer func() {
			if value := recover(); value != nil {
				send(fmt.Sprintf("builder:%s\n执行【%s】异常：%v", builder, display, value), phone)
				panic(value)
			}
			status := "完毕！"
			if err != nil {
				status = "失败：" + err.Error()
				if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
					status = "已取消：" + err.Error()
				}
			}
			send(fmt.Sprintf("builder:%s\n执行【%s】%s", builder, display, status), phone)
		}()
		err = job.Run(ctx, data)
		if err == nil {
			err = ctx.Err()
		}
		return err
	}}
}

func callbackPanicReporter(webhook, project, executor, phone string) func(taskflow.Result, any, []byte) {
	return func(result taskflow.Result, value any, stack []byte) {
		log.Error(fmt.Sprintf("task=%s 回调 panic: %v\n%s", result.TaskID, value, stack))
		message := fmt.Sprintf("构建回调发生 panic\n任务:%s\n项目:%s\n执行人:%s\n异常:%v", result.TaskID, project, executor, value)
		runes := []rune(message)
		if len(runes) > 2000 {
			message = string(runes[:2000]) + "…"
		}
		// Do not retry the failed business callback. Keep stack traces in local logs.
		if err := tool.SendDingMessage(webhook, message, phone); err != nil {
			log.Error("发送回调异常告警失败", err)
		}
	}
}
