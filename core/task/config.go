package task

import (
	"autobuildrobot/models"
	"autobuildrobot/taskflow"
	"context"
	"errors"
	"strings"
)

// Configuration is read or written only when the task runs, after authorization.
func getConfigTask(param *Param) (taskflow.Task, error) {
	if param == nil {
		return nil, errors.New("配置指令参数不能为空")
	}
	var update func(string, string) (string, error)
	var query func(string, string) string
	adapt := func(fn func(string, string) string) func(string, string) (string, error) {
		return func(project, raw string) (string, error) { return fn(project, raw), nil }
	}
	switch param.CmdType {
	case models.CommandType_ProjectConfig:
		update = models.UpdateProject
		query = func(project, _ string) string { return models.GetProjectData(project) }
	case models.CommandType_SvnProjectConfig:
		update, query = adapt(models.UpdateSvnProject), models.QuerySvnProjectsDataByProject
	case models.CommandType_CdnConfig:
		update, query = adapt(models.UpdateCdn), models.QueryCdnDataOfOneProject
	case models.CommandType_SvrProgressConfig:
		update, query = adapt(models.UpdateSvrProgressData), models.QueryProgressDataOfOneProject
	case models.CommandType_SvrMachineConfig:
		update, query = adapt(models.UpdateSvrMachineData), models.QuerySvrMachineDataOfOneProject
	case models.CommandType_User:
		update, query = adapt(models.UpdateUserInfo), models.QueryUsersDatas
	case models.CommandType_UserGroup:
		update = func(_, raw string) (string, error) { return models.UpdateUserGroup(raw) }
		query = func(_, raw string) string { return models.QueryUserGroupDatas(raw) }
	case models.CommandType_TemplateCmd:
		update, query = adapt(models.UpdateCmdTemp), models.QueryCmdTempsDataByProject
	default:
		return nil, errors.New("不支持的配置指令")
	}
	project, raw, send := param.ProjectName, param.RawParams, param.SendResult
	return taskflow.Func{Title: models.GetCommandNameByType(param.CmdType), Action: func(ctx context.Context, _ any) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		var result string
		if strings.Contains(raw, "{") {
			var err error
			result, err = update(project, raw)
			if err != nil {
				return err
			}
		} else {
			result = query(project, raw)
		}
		if send != nil {
			send(result, "")
		}
		return nil
	}}, nil
}
