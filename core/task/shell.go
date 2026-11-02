package task

import (
	"autobuildrobot/models"
	"autobuildrobot/taskflow"
	"autobuildrobot/tool"
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"
)

type shellRequest struct {
	script, root, project string
	args                  []string
	command               int
	send                  SendResultFunc
}

func getShellTask(param *Param) (taskflow.Task, error) {
	if param == nil {
		return nil, errors.New("指令参数不能为空")
	}
	minimum := 1
	switch param.CmdType {
	case models.CommandType_SvnMerge, models.CommandType_UpdateAndRestartSvr, models.CommandType_CloseSvr, models.CommandType_AutoBuildClient:
		minimum = 2
	case models.CommandType_UpdateTable, models.CommandType_BuildPbMsg:
	default:
		return nil, errors.New("不支持的脚本指令")
	}
	if len(param.CmdParams) < minimum {
		return nil, errors.New("参数不足，请通过帮助查看指令格式")
	}
	for _, value := range param.CmdParams[:minimum] {
		if strings.TrimSpace(value) == "" {
			return nil, errors.New("工程名或服务名不能为空")
		}
	}
	params := append([]string(nil), param.CmdParams...)
	err, root, url, _ := models.GetSvnProjectInfo(param.ProjectName, params[0])
	if err != nil {
		return nil, err
	}
	canonical := func(root string) (string, error) {
		if strings.TrimSpace(root) == "" {
			return "", errors.New("工程路径不能为空")
		}
		abs, err := filepath.Abs(root)
		if err != nil {
			return "", err
		}
		return filepath.EvalSymlinks(abs)
	}
	root, err = canonical(root)
	if err != nil {
		return nil, err
	}
	roots := []string{root}
	r := shellRequest{root: root, project: param.ProjectName, command: param.CmdType, send: param.SendResult}
	switch param.CmdType {
	case models.CommandType_AutoBuildClient:
		err, args := getUnityBuildParams(param.ProjectName, param.WebHook, params)
		if err != nil {
			return nil, err
		}
		args[0], err = filepath.Abs(args[0])
		if err != nil {
			return nil, err
		}
		args[1] = root
		r.script, r.args = "autoBuildClient.sh", args
	case models.CommandType_SvnMerge:
		err, target, _, _ := models.GetSvnProjectInfo(param.ProjectName, params[1])
		if err != nil {
			return nil, err
		}
		target, err = canonical(target)
		if err != nil {
			return nil, err
		}
		roots = append(roots, target)
		r.root = target
		policy := "tf"
		if len(params) > 2 && (params[2] == "tf" || params[2] == "mf" || params[2] == "p") {
			policy = params[2]
		}
		message := params[0] + "合并到" + params[1]
		if len(params) > 3 {
			message = params[3]
		}
		r.script, r.args = "svnmerge.sh", []string{target, url, policy, message}
	case models.CommandType_UpdateTable, models.CommandType_BuildPbMsg:
		err, enginePath := models.GetProjectClientEnginePath(param.ProjectName)
		if err != nil {
			return nil, err
		}
		extra := ""
		if len(params) > 1 {
			extra = params[1]
		}
		r.args = []string{root, runtime.GOOS, enginePath, param.WebHook, extra}
		r.script = "ReadExcel.sh"
		if param.CmdType == models.CommandType_BuildPbMsg {
			r.script = "buildproto.sh"
		}
	case models.CommandType_UpdateAndRestartSvr, models.CommandType_CloseSvr:
		err, ip, port, account, password, platform, serverRoot := models.GetSvrMachineData(param.ProjectName, params[0])
		if err != nil {
			return nil, err
		}
		err, folder, dir, zip, files, dirs, skipCompile := models.GetSvrProgressData(param.ProjectName, params[1])
		if err != nil {
			return nil, err
		}
		r.script = "close_svr.py"
		r.args = []string{platform, zip, ip, port, account, password, serverRoot}
		if param.CmdType == models.CommandType_UpdateAndRestartSvr {
			r.script = "auto_update_server.py"
			r.args = []string{filepath.Join(root, folder), dir, platform, zip, dirs, files, ip, port, account, password, serverRoot, strconv.FormatBool(skipCompile)}
		}
	}
	if param.CmdType != models.CommandType_AutoBuildClient {
		if script := models.GetExecToolFileName(param.ProjectName, params, param.CmdType); script != "" {
			r.script = script
		}
	}
	// Lock canonical directory paths in sorted order, including aliases.
	paths := make(map[string]string)
	for _, root := range roots {
		key := root
		if runtime.GOOS == "windows" {
			key = strings.ToLower(key)
		}
		paths[key] = root
	}
	keys := make([]string, 0, len(paths))
	for key := range paths {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	var job taskflow.Task = taskflow.Func{Title: models.GetCommandNameByType(param.CmdType), Action: shellBuild}
	for i := len(keys) - 1; i >= 0; i-- {
		job, err = taskflow.TaskDirLocks.Wrap(paths[keys[i]], job)
		if err != nil {
			return nil, err
		}
	}
	return taskflow.BindTask(job, r), nil
}

func shellBuild(ctx context.Context, data any) error {
	r, ok := data.(shellRequest)
	if !ok || r.script == "" || r.root == "" || len(r.args) == 0 || (r.command == models.CommandType_AutoBuildClient && len(r.args) != 8) {
		return fmt.Errorf("无效的脚本任务参数: %T", data)
	}
	scriptDir, err := filepath.Abs(shellPath)
	if err != nil {
		return err
	}
	var logs *taskflow.LogAggregator
	if r.send != nil {
		logs = taskflow.NewLogAggregator(time.Duration(maxIntervalTimeBetween2Msg*float64(time.Second)), lineInOneMes, 16*1024, func(msg string) { r.send(msg, "") })
		defer logs.Stop()
	}
	failedOutput := false
	output := func(line string) {
		lower := strings.ToLower(line)
		if !strings.Contains(line, "    ") && (strings.Contains(lower, "error") || strings.Contains(lower, "fail") || strings.Contains(line, "失败")) {
			failedOutput = true
		}
		if logs != nil {
			logs.Write(line)
		}
	}
	runScript := func(script string, args []string) error {
		executable := "sh"
		if runtime.GOOS == "windows" {
			executable = winGitPath
		}
		if strings.EqualFold(filepath.Ext(script), ".py") {
			executable = "python2"
		}
		return tool.ExecCommandArgs(ctx, scriptDir, executable, append([]string{script}, args...), output)
	}
	err = runScript(r.script, r.args)
	if ctx.Err() != nil {
		return ctx.Err()
	}
	// Unity reports build failure through its exit status; ordinary log text can
	// contain words such as "error" even on a successful build.
	if r.command == models.CommandType_AutoBuildClient {
		return err
	}
	generated := r.command == models.CommandType_UpdateTable || r.command == models.CommandType_BuildPbMsg
	if r.command == models.CommandType_SvnMerge || generated {
		conflict := false
		statusErr := tool.ExecCommandArgs(ctx, r.root, "svn", []string{"status", "-q"}, func(line string) {
			if (len(line) > 0 && line[0] == 'C') || (len(line) > 1 && line[1] == 'C') || (len(line) > 6 && line[6] == 'C') {
				conflict = true
			}
		})
		err = errors.Join(err, statusErr)
		if conflict {
			if r.send != nil {
				r.send("info:合并冲突或者其他问题，需要手动处理！！", models.GetProjectManagerPhone(r.project))
			}
			err = errors.Join(err, errors.New("SVN 合并冲突"))
		}
	}
	if failedOutput {
		err = errors.Join(err, errors.New("脚本输出包含失败信息"))
	}
	if generated {
		if err != nil {
			revertErr := tool.ExecCommandArgs(ctx, r.root, "svn", []string{"revert", "-R", "."}, output)
			return errors.Join(err, revertErr)
		}
		err = runScript("svnCommit.sh", []string{r.root, "latest!!!"})
		if failedOutput {
			err = errors.Join(err, errors.New("SVN 提交输出包含失败信息"))
		}
		return err
	}
	return err
}

func getUnityBuildParams(projectName, webHook string, params []string) (error, []string) {
	err, enginePath := models.GetProjectClientEnginePath(projectName)
	if nil != err {
		return err, nil
	}

	//获取构建方法
	if len(params) < 2 {
		return errors.New("构建参数不足，请通过帮助传正确参数"), nil
	}
	err, buildMethod, methodIndex := models.GetBuildMethod(projectName, params[0], params[1])
	if nil != err {
		return err, nil
	}
	err, projectPath, _, _ := models.GetSvnProjectInfo(projectName, params[0])
	if nil != err {
		return err, nil
	}

	//处理打资源逻辑（多语言）
	language := ""
	if len(params) > 2 {
		language = params[2]
	}
	platform := models.GetPlatform(projectName, params[0])
	if methodIndex == models.BuildResType_Lua || methodIndex == models.BuildResType_Total {
		_, conLan := models.GetLanguages(projectName, params[0])
		if methodIndex == models.BuildResType_Total && language == "" && nil != conLan && len(conLan) > 0 {
			return errors.New("请指定语言！！！"), nil
		}
	}

	//获取热更地址
	err, svrFilesUrl := models.GetFullTestHotfixPath(projectName, params[0])
	if err != nil {
		return err, nil
	}

	//目前其他参数只有是否开发包
	otherParams := ""
	if len(params) > 3 {
		otherParams = params[3]
	}

	//依次需要客户端引擎路径、工程路径、构建方法、webhook、语言、平台、其他参数
	return nil, []string{enginePath, projectPath, buildMethod, webHook, language, svrFilesUrl, platform, otherParams}
}
