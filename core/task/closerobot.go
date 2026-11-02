package task

import (
	"autobuildrobot/models"
	log "autobuildrobot/mylog"
	"autobuildrobot/taskflow"
	"autobuildrobot/tool"
	"context"
	"errors"
	"github.com/astaxie/beego"
	"path/filepath"
	"runtime"
	"time"
)

// CloseRobot is used by the scheduled shutdown as well as the command task.
func CloseRobot(ctx context.Context) error {
	job, err := getCloseRobotTask(&Param{CmdType: models.CommandType_CloseRobot})
	if err != nil {
		return err
	}
	return job.Run(ctx, nil)
}

func getCloseRobotTask(p *Param) (taskflow.Task, error) {
	if p == nil || p.CmdType != models.CommandType_CloseRobot {
		return nil, errors.New("无效的关闭机器人指令")
	}
	send := p.SendResult
	return taskflow.Func{Title: models.GetCommandNameByType(models.CommandType_CloseRobot), Action: func(ctx context.Context, _ any) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		dir, err := filepath.Abs(shellPath)
		if err != nil {
			return err
		}
		executable := "sh"
		if runtime.GOOS == "windows" {
			executable = winGitPath
		}
		return tool.ExecCommandArgs(ctx, dir, executable, []string{"stopAutoBuildRobot.sh"}, func(line string) {
			if send != nil {
				send(line, "")
			}
		})
	}}, nil
}

func init() {
	closeRobotTime := beego.AppConfig.DefaultInt("closeRobotTime", 3580)
	if closeRobotTime < 0 || runtime.GOOS != "darwin" {
		return
	}
	now := time.Now()
	target := time.Date(now.Year(), now.Month(), now.Day(), closeRobotTime/3600, (closeRobotTime%3600)/60, closeRobotTime%60, 0, now.Location()).AddDate(0, 0, 1)
	time.AfterFunc(time.Until(target), closeAutoBuildByTime)
}

func closeAutoBuildByTime() {
	if err := CloseRobot(context.Background()); err != nil {
		log.Error("定时关闭机器人失败:", err)
	}
	time.AfterFunc(24*time.Hour, closeAutoBuildByTime)
}
