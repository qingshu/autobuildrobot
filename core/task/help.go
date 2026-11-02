package task

import (
	"autobuildrobot/models"
	"autobuildrobot/taskflow"
	"context"
	"fmt"
)

func getHelpTask(param *Param) (taskflow.Task, error) {
	unOpenCommandList := models.GetUnopenCommandList(param.ProjectName)
	help := "指令如下：\n"
	for i := 0; i < models.CommandType_Max; i++ {
		isUnOpen := false
		for _, unOpenCommand := range unOpenCommandList {
			if unOpenCommand == models.GetCommandNameByType(i) {
				isUnOpen = true
				break
			}
		}
		if isUnOpen {
			continue
		}
		help += fmt.Sprintf("%d:%s\n", i, models.GetCommandNameByType(i))
	}
	help += fmt.Sprintf("\n输入指令名称或者编号选择操作，指令后加冒号和参数如【%s：%s】\n如果不清楚参数则输入帮助或者help会输出详细帮助提示如【%s：帮助】",
		models.GetCommandNameByType(models.CommandType_UpdateTable), "研发表格", models.GetCommandNameByType(models.CommandType_UpdateTable))
	help += "\n指令如果不带动词，表示配置型指令，配置型指令参数为空则会输出所有已有数据或者输入查询条件筛选出对应数据\n"
	help += fmt.Sprintf("如果要执行多条指令，则指令间用->连接，如【%s：研发表格->%s：正式表格】", models.GetCommandNameByType(models.CommandType_UpdateTable), models.GetCommandNameByType(models.CommandType_UpdateTable))

	return taskflow.Func{
		Title: "帮助",
		Action: func(ctx context.Context, a any) error {
			param.SendResult(help, "")
			return nil
		},
	}, nil
}
