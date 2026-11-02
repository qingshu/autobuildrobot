package task

import (
	"autobuildrobot/models"
	"autobuildrobot/taskflow"
	"autobuildrobot/tool"
	"context"
	"errors"
	"github.com/astaxie/beego"
	"strings"
	"time"
)

func getBackupSvnTask(p *Param) (taskflow.Task, error) {
	if p == nil || p.CmdType != models.CommandType_BackupSvn {
		return nil, errors.New("无效的 SVN 备份指令")
	}
	send := p.SendResult
	return taskflow.Func{Title: models.GetCommandNameByType(models.CommandType_BackupSvn), Action: func(ctx context.Context, _ any) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		account := beego.AppConfig.DefaultString("svnMachineAccount", "")
		password := beego.AppConfig.DefaultString("svnMachinePwd", "")
		address := beego.AppConfig.DefaultString("svnMachineAddr", "")
		if strings.TrimSpace(account) == "" || strings.TrimSpace(address) == "" {
			return errors.New("请配置 svnMachineAccount 和 svnMachineAddr")
		}
		logs := taskflow.NewLogAggregator(time.Duration(maxIntervalTimeBetween2Msg*float64(time.Second)), lineInOneMes, 16*1024, func(msg string) {
			if send != nil {
				send(msg, "")
			}
		})
		defer logs.Stop()
		err := tool.RemoteShell("python backupSvn.py", account, password, address, func(line string) { logs.Write(line) })
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return err
	}}, nil
}
