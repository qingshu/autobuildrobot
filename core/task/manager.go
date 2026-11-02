package task

import (
	"autobuildrobot/models"
	"autobuildrobot/taskflow"
	"context"
	"errors"
	"fmt"
	"runtime/debug"
	"strconv"
	"strings"
	"sync"

	"github.com/astaxie/beego"
)

var winGitPath string
var lineInOneMes = 200

const (
	shellPath                  = "shell"    //shell脚本地址
	maxIntervalTimeBetween2Msg = float64(5) //一条消息间隔最长时间
	seriesCommandSeparator     = "->"       // 多条指令分隔符
)

type SendResultFunc func(msg, executorPhoneNum string)
type Param struct {
	CmdType     int            //指令类型
	CmdParams   []string       //指令参数
	RawParams   string         //冒号统一后的原始参数，不做逗号替换或拆分
	ProjectName string         //项目名称（群标题，一个群一个项目）
	WebHook     string         //回调地址
	SendResult  SendResultFunc //发送处理结果
}

type GetBuildFunc func(params *Param) (taskflow.Task, error)

var buildFuncs [models.CommandType_Max]GetBuildFunc
var engine *taskflow.Engine

func init() {
	// Beego 1.x GetConfig has a broken integer-default fallback when a key is absent.
	workers := beego.AppConfig.DefaultInt("worksCount", 3)
	queueSize := beego.AppConfig.DefaultInt("workChanLen", 10)
	var err error
	config := taskflow.Config{Workers: workers, QueueSize: queueSize}
	engine, err = taskflow.New(config)
	if err != nil {
		panic(fmt.Errorf("构建 engine 失败: %w", err))
	}

	temp, _ := beego.GetConfig("String", "winGitPath", "")
	winGitPath = temp.(string)
	temp, err = beego.GetConfig("Int", "lineInOneMes", 200)
	if err == nil {
		if n, ok := temp.(int); ok && n > 0 {
			lineInOneMes = n
		}
	}

}

func ExecCommand(projectName, executor, rawCmd, webHook string, sendResultFunc SendResultFunc) <-chan struct{} {
	done := make(chan struct{})
	phoneNum := models.GetUserPhone(projectName, executor)
	reportPanic := callbackPanicReporter(webHook, projectName, executor, phoneNum)
	var sendMu sync.Mutex
	deliver := func(msg, phone string) {
		sendMu.Lock()
		defer sendMu.Unlock()
		if sendResultFunc != nil {
			sendResultFunc(msg, phone)
		}
	}
	send := func(msg, phone string) {
		defer func() {
			if value := recover(); value != nil {
				reportPanic(taskflow.Result{Name: "构建通知"}, value, debug.Stack())
			}
		}()
		deliver(msg, phone)
	}
	accepted := make(chan struct{})
	_task, err := GetTaskForUser(projectName, executor, webHook, rawCmd, func(msg, phone string) {
		<-accepted // Per-command output follows the submission notification.
		send(msg, phone)
	})
	if err != nil {
		send(fmt.Sprintf("builder:%s\ninfo:执行异常%s\n,cmd:%s", executor, err.Error(), rawCmd), phoneNum)
		close(done)
		return done
	}
	taskId, err := engine.SubmitWithCallback(_task, nil, taskflow.WithCallbackPanicHandler(func(result taskflow.Result) {
		defer close(done)
		<-accepted // Keep even very fast completions after the acceptance notice.
		status := "成功"
		if result.Err != nil {
			status = "失败: " + result.Err.Error()
			if errors.Is(result.Err, context.Canceled) || errors.Is(result.Err, context.DeadlineExceeded) {
				status = "已取消: " + result.Err.Error()
			}
		}
		deliver(fmt.Sprintf("builder:%s\ntask:%s\n结果:%s\n耗时:%s\n指令:%s", executor, result.TaskID, status, result.Duration, rawCmd), phoneNum)
	}, reportPanic))
	if err != nil {
		send(fmt.Sprintf("builder:%s\ninfo:执行异常%s\n,cmd:%s", executor, err.Error(), rawCmd), phoneNum)
		close(done)
		return done
	}
	send(fmt.Sprintf("builder:%s\n已提交，执行id:%s\n原始指令:%s", executor, taskId, rawCmd), phoneNum)
	close(accepted)
	return done
}

// GetTaskForUser validates all commands before constructing or submitting work.
func GetTaskForUser(projectName, executor, webHook, rawCmd string, sendResult SendResultFunc) (taskflow.Task, error) {
	return getTask(projectName, executor, webHook, rawCmd, sendResult, true)
}

func getTask(projectName, executor, webHook, rawCmd string, sendResult SendResultFunc, authorize bool) (taskflow.Task, error) {
	err, cmdParms := getTaskParams(projectName, rawCmd)
	if err != nil {
		return nil, err
	}

	//这里统一用seqTask,懒得判断单个
	if len(cmdParms) == 0 {
		return nil, errors.New("没有可执行的指令")
	}
	if authorize {
		if strings.TrimSpace(executor) == "" {
			return nil, errors.New("执行用户不能为空")
		}
		for _, p := range cmdParms {
			if ok, reason := models.JudgeIsHadPermission(p.CmdType, projectName, executor, p.RawParams); !ok {
				return nil, fmt.Errorf("指令 %s 无权限: %s", models.GetCommandNameByType(p.CmdType), reason)
			}
		}
	}
	seqTask := taskflow.Sequence{Title: rawCmd}
	seqTask.Children = make([]taskflow.Task, len(cmdParms))
	phone := ""
	if sendResult != nil {
		phone = models.GetUserPhone(projectName, executor)
	}
	for i, cmdParm := range cmdParms {
		//补全参数
		cmdParm.WebHook = webHook
		cmdParm.SendResult = sendResult

		//获取task
		taskFunc := buildFuncs[cmdParm.CmdType]
		if taskFunc == nil {
			return nil, fmt.Errorf("不存在构建方法：%d", cmdParm.CmdType)
		}
		task, err := taskFunc(cmdParm)
		if err != nil {
			return nil, fmt.Errorf("构建类型【%d】异常: %w", cmdParm.CmdType, err)
		}
		display := models.GetCommandNameByType(cmdParm.CmdType)
		if cmdParm.RawParams != "" {
			display += ":" + cmdParm.RawParams
		}
		seqTask.Children[i] = withCommandNotifications(task, display, i+1, len(cmdParms), executor, phone, sendResult)
	}
	return seqTask, nil
}

// 获取任务参数
func getTaskParams(projectName, rawCmd string) (err error, cmdParms []*Param) {
	cmdParms = make([]*Param, 0)

	if strings.Contains(rawCmd, "{") && hasUnquotedCommandSeparator(rawCmd) {
		return errors.New("不支持配置型指令拼接！"), cmdParms
	}

	//有“{”表示修改配置
	if strings.Contains(rawCmd, "{") {
		cmdParm := analysisOneCommand(projectName, rawCmd)
		if nil == cmdParm {
			return fmt.Errorf("解析指令【%s】异常，请检查是否有该指令,或者输入【帮助】获取所有指令帮助信息！", rawCmd), cmdParms
		}
		cmdParms = append(cmdParms, cmdParm)
		return
	}

	//解析拼接指令
	cmdParms, err = expandCommands(projectName, rawCmd, models.GetTemplateCmd, make(map[string]bool), 0)
	return err, cmdParms
}

// JSON string values may contain a sequence of commands (template Cmd).
// Only separators outside strings join the configuration to another command.
func hasUnquotedCommandSeparator(raw string) bool {
	quoted, escaped := false, false
	for i := 0; i < len(raw); i++ {
		if quoted {
			if escaped {
				escaped = false
			} else if raw[i] == '\\' {
				escaped = true
			} else if raw[i] == '"' {
				quoted = false
			}
			continue
		}
		if raw[i] == '"' {
			quoted = true
		} else if strings.HasPrefix(raw[i:], seriesCommandSeparator) {
			return true
		}
	}
	return false
}

func expandCommands(projectName, rawCmd string, lookup func(string, string) string, visiting map[string]bool, depth int) ([]*Param, error) {
	if depth > 32 {
		return nil, errors.New("模板嵌套超过32层")
	}
	var result []*Param
	rawCmds := strings.Split(rawCmd, seriesCommandSeparator)
	for _, _rawCmd := range rawCmds {
		_rawCmd = strings.TrimSpace(_rawCmd)
		if _rawCmd == "" {
			return nil, errors.New("指令不能为空")
		}
		//指令中又包含模板指令
		if !strings.ContainsAny(_rawCmd, ":：") {
			//没有冒号，基本除了帮助就可能是模板指令，简单处理吧
			tplCmd := lookup(projectName, _rawCmd)
			if tplCmd != "" {
				if visiting[_rawCmd] {
					return nil, fmt.Errorf("模板循环引用: %s", _rawCmd)
				}
				visiting[_rawCmd] = true
				nested, err := expandCommands(projectName, tplCmd, lookup, visiting, depth+1)
				delete(visiting, _rawCmd)
				if err != nil {
					return nil, err
				}
				result = append(result, nested...)
				continue
			}
		}

		//不是模板指令
		cmdParm := analysisOneCommand(projectName, _rawCmd)
		if nil == cmdParm {
			return nil, fmt.Errorf("不存在指令，检查原始指令：%s", _rawCmd)
		}
		result = append(result, cmdParm)
	}
	return result, nil
}

func analysisOneCommand(projectName, rawCommand string) (cmdParams *Param) {
	//约定指令格式为指令名称+冒号+指令参数，所以用冒号分割
	// 统一在此处全局替换冒号，包括配置内容（JSON 值）中的中文冒号也允许改为英文冒号。
	rawCommand = strings.ReplaceAll(rawCommand, "：", ":")
	commands := strings.SplitN(rawCommand, ":", 2)

	//参数去掉空格和换行
	requestCommand := commands[0]
	requestCommand = strings.Replace(requestCommand, " ", "", -1)
	params := make([]string, 0)
	if len(commands) > 1 {
		requestParam := strings.TrimSpace(commands[1])
		requestParam = strings.Replace(requestParam, "\n", "", -1)
		requestParam = strings.ReplaceAll(requestParam, "，", ",")
		requestParam = strings.ReplaceAll(requestParam, "；", ";")
		params = strings.Split(requestParam, ",")
	}

	for commandType := 0; commandType < models.CommandType_Max; commandType++ {
		name := models.GetCommandNameByType(commandType)
		if requestCommand == name || requestCommand == strconv.Itoa(commandType) {
			cmdParams = &Param{}
			cmdParams.CmdType = commandType
			cmdParams.ProjectName = projectName
			cmdParams.CmdParams = params
			if len(commands) > 1 {
				cmdParams.RawParams = strings.TrimSpace(commands[1])
			}
			if commandType == models.CommandType_SvnMerge {
				if err, mergeParams := models.AnalysisParam(cmdParams.RawParams, commandType); err == nil {
					cmdParams.CmdParams = mergeParams
				}
			}
			return
		}
	}
	return
}
