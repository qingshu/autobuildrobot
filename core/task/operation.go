package task

import (
	"autobuildrobot/core/cdn"
	"autobuildrobot/models"
	log "autobuildrobot/mylog"
	"autobuildrobot/taskflow"
	"autobuildrobot/tool"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/astaxie/beego"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

type operationCommand struct {
	CommandType                               int
	Name, CommandParams, ProjectName, WebHook string
	SendResult                                SendResultFunc
}

// registerOperation registers task-layer SVN and CDN operations during init.
func registerOperation(command int, run func(*operationCommand) (string, error)) {
	buildFuncs[command] = func(param *Param) (taskflow.Task, error) {
		if param == nil || len(param.CmdParams) == 0 || strings.TrimSpace(param.CmdParams[0]) == "" {
			return nil, errors.New("必须指定 SVN 工程名称")
		}
		if command == models.CommandType_PrintHotfixResList || command == models.CommandType_UploadHotfixRes2Test {
			if len(param.CmdParams) < 2 || strings.TrimSpace(param.CmdParams[1]) == "" {
				return nil, errors.New("必须指定语言")
			}
		}
		err, root, _, _ := models.GetSvnProjectInfo(param.ProjectName, param.CmdParams[0])
		if err != nil {
			return nil, err
		}
		if strings.TrimSpace(root) == "" {
			return nil, errors.New("工程路径不能为空")
		}
		p := *param
		p.CmdParams = append([]string(nil), param.CmdParams...)
		var job taskflow.Task = taskflow.Func{Title: models.GetCommandNameByType(command), Action: func(ctx context.Context, _ any) error {
			if err := ctx.Err(); err != nil {
				return err
			}
			// Checkout may need a new directory. Create and lock it only during execution.
			if command == models.CommandType_CheckOutSvnProject {
				if err := os.MkdirAll(root, 0755); err != nil {
					return err
				}
			}
			operation := taskflow.Func{Title: models.GetCommandNameByType(command), Action: func(ctx context.Context, _ any) error {
				if err := ctx.Err(); err != nil {
					return err
				}
				send := func(msg, phone string) {
					if p.SendResult != nil {
						p.SendResult(msg, phone)
					}
				}
				result, err := run(&operationCommand{CommandType: command, Name: models.GetCommandNameByType(command), CommandParams: strings.Join(p.CmdParams, ","), ProjectName: p.ProjectName, WebHook: p.WebHook, SendResult: send})
				if err != nil {
					return err
				}
				if err := ctx.Err(); err != nil {
					return err
				}
				send(result, "")
				return nil
			}}
			locked, err := taskflow.TaskDirLocks.Wrap(root, operation)
			if err != nil {
				return err
			}
			return locked.Run(ctx, nil)
		}}
		return job, nil
	}
}

func UploadHotfixRes2Test(command *operationCommand) (string, error) {
	//获取参数
	err, params := models.AnalysisParam(command.CommandParams, command.CommandType)
	if nil != err {
		return "", err
	}
	if len(params) < 2 {
		return "", errors.New("参数不足，必须指定项目名和语言！")
	}

	//获取项目信息
	svnProjectName := params[0]
	err, projectPath, _, _ := models.GetSvnProjectInfo(command.ProjectName, svnProjectName)
	if err != nil {
		return "", err
	}

	//获取cdn配置
	err, cdnType, urlOfBucket, bucketName, zzzTimeStamp, _, accessKeyID, accessKeySecret, _, platform, resPaths, lans := models.GetCdnData(
		command.ProjectName, svnProjectName)
	if err != nil {
		return "", err
	}

	//获取热更语言
	language := ""
	if len(params) > 1 {
		language = params[1]
		_, err = strconv.Atoi(language)
		if err != nil {
			return "", errors.New("转换语言失败,只能是数字：" + err.Error())
		}
	} else {
		return "", errors.New("请指定语言！！")
	}

	//从缓存中取，如果没有则提示重新获取更新列表
	models.ClientHotFixedDataLock.Lock()
	defer models.ClientHotFixedDataLock.Unlock()
	var updateData *models.UpdateDataOfProject
	ok := false
	updateKey := models.GetHotFixedMapKey(svnProjectName, platform, language)
	if updateData, ok = models.ClientHotFixedFileDataTempMap[updateKey]; !ok {
		return "", errors.New(fmt.Sprintf("获取热更缓存数据失败，请重新执行【%s】指令！", models.GetCommandNameByType(models.CommandType_PrintHotfixResList)))
	}
	if nil == updateData || len(updateData.HotFiexedDatas) <= 0 {
		return "", errors.New(fmt.Sprintf("缓存热更数据为空，请重新执行【%s】指令试试！", models.GetCommandNameByType(models.CommandType_PrintHotfixResList)))
	}

	fileName := tool.DealZZZZZName(models.CLIENTFILESNAME, zzzTimeStamp) + ".txt"
	originalFileName := tool.DealZZZZZName(models.OriginalFILESNAME, zzzTimeStamp) + ".txt"

	//获取cdn对象
	cdnErr, cdnClient := cdn.GetCdnClient(cdnType, urlOfBucket, bucketName, accessKeyID, accessKeySecret)
	if nil != cdnErr {
		return "", cdnErr
	}

	//获取混淆文件隐射表
	result := ""
	zzzMapping, err := tool.GetOriginalAndZZZName(path.Join(projectPath, "zzzzzName.txt"), zzzTimeStamp)
	if err != nil {
		result += err.Error()
	}

	//上传一个文件
	count := 0
	successCount := 0
	needHotResList := make([]string, 0)
	temLan := language
	if nil == lans || len(lans) <= 0 {
		temLan = ""
	}
	hotfixedPath := models.GetHotFixedResPath(projectPath, zzzTimeStamp, platform, temLan)
	lastTime := time.Now().Unix()
	uploadFile := func(testPath, _platform, lan, uploadFileName, _originalName, msg string) (error, string) {
		localFilesPath := path.Join(hotfixedPath, uploadFileName)
		remoteFilePath := path.Join(testPath, _platform, lan, uploadFileName)
		fixFileName := uploadFileName
		if _originalName != "" {
			fixFileName = fmt.Sprintf("%s:%s", _originalName, uploadFileName)
		}
		err := cdnClient.UploadFile(localFilesPath, remoteFilePath)
		if nil == err {
			count++
			successCount++
			uploadResult := fmt.Sprintf("上传%d、%s成功\n", successCount, fixFileName)
			msg += uploadResult
			log.Info(uploadResult)
		} else {
			return errors.New(fmt.Sprintf("上传%s失败，原因%s\n", fixFileName, err.Error())), ""
		}

		//避免消息过长，钉钉截掉
		timeNow := time.Now().Unix()
		if count >= lineInOneMes || (timeNow-lastTime) > int64(maxIntervalTimeBetween2Msg) {
			command.SendResult(msg, "")
			msg = ""
			count = 0
			lastTime = timeNow
		}
		return nil, msg
	}

	//先上传所有热更文件
	testPath := resPaths[0]
	uploadSuccessMsg := fmt.Sprintf("开始上传语言【%s】测试热更资源：\n", language)
	for _, hotfiexedData := range updateData.HotFiexedDatas {
		if v, exist := updateData.UpdateSuccessFiles[hotfiexedData.Name]; v && exist {
			uploadSuccessMsg += fmt.Sprintf("已经上传%s成功，不再上传\n", hotfiexedData.Name)
			continue
		}
		tempName := strings.Split(filepath.Base(hotfiexedData.Name), ".")[0]
		_originalName, _ := zzzMapping[tempName]
		err, uploadSuccessMsg = uploadFile(testPath, platform, temLan, hotfiexedData.Name, _originalName, uploadSuccessMsg)
		if err != nil {
			return "", err
		} else {
			remoteFilePath := path.Join(testPath, platform, temLan, hotfiexedData.Name)
			needHotResList = append(needHotResList, remoteFilePath)
			updateData.UpdateSuccessFiles[hotfiexedData.Name] = true
		}
	}

	//再上传originalfiles.txt并返回md5值
	if zzzTimeStamp != "" {
		err, uploadSuccessMsg = uploadFile(testPath, platform, temLan, originalFileName, models.OriginalFILESNAME, uploadSuccessMsg)
		if err != nil {
			return "", err
		}
	}

	//再上传files.txt并返回md5值
	err, uploadSuccessMsg = uploadFile(testPath, platform, temLan, fileName, models.CLIENTFILESNAME, uploadSuccessMsg)
	if err != nil {
		return "", err
	}

	//先输出结果
	updateData.HotFiexedDatas = make([]*models.ClientHotFixedFileData, 0)
	updateData.UpdateSuccessFiles = make(map[string]bool)
	updateData.PreHotRes = needHotResList
	result += uploadSuccessMsg

	return result, nil
}

func CheckOutSvnProject(command *operationCommand) (string, error) {
	//解析指令
	err, params := models.AnalysisParam(command.CommandParams, command.CommandType)
	if nil != err {
		return "", err
	}

	//获取svn工程配置
	err, projectPath, svnUrl, _ := models.GetSvnProjectInfo(command.ProjectName, params[0])
	if nil != err {
		return "", err
	}

	executable := "sh"
	if runtime.GOOS == "windows" {
		executable = winGitPath
	}

	//如果路径存在，则输出svn信息
	isExist, err := tool.PathExists(filepath.Join(projectPath, ".svn"))
	if nil != err {
		return "", err
	}
	if isExist {
		svnInfoCommand := fmt.Sprintf("cd %s;svn info", projectPath)
		result, err := tool.ExecSimpleCmd(executable, svnInfoCommand)
		return fmt.Sprintf("已存在【%s】svn工程!!\n%s", params[0], result), err
	}

	//创建文件夹
	dirErr := tool.CreateDir(projectPath)
	if nil != dirErr {
		return "", dirErr
	}

	//检出svn
	count := 0
	temp := ""
	lastTime := time.Now().Unix()
	svnCheckOutCommand := fmt.Sprintf("cd %s;svn co %s .", projectPath, svnUrl)
	errCmd := tool.ExecComplexCmd(executable, svnCheckOutCommand, func(resultLine string) {
		timeNow := time.Now().Unix()
		if (timeNow - lastTime) < int64(maxIntervalTimeBetween2Msg) {
			//过滤掉一些消息，要不消息太恐怖了
			return
		}

		//每隔80行发送一条构建消息
		count++
		temp += resultLine
		if count >= lineInOneMes {
			command.SendResult(temp, "")
			temp = ""
			count = 0
			lastTime = timeNow
		}
	})
	if nil != errCmd {
		return "", errCmd
	}
	return temp + "\n检出完毕（避免消息爆炸，有过滤一些svn检出日志）！！", nil
}

func UploadHotfixRes2Release(command *operationCommand) (string, error) {
	//获取分支名称
	err, params := models.AnalysisParam(command.CommandParams, command.CommandType)
	if nil != err {
		return "", err
	}
	svnProjectName := params[0]

	//获取cdn配置
	err, cdnType, urlOfBucket, bucketName, zzzTimeStamp, fileDecodeKey, accessKeyID, accessKeySecret, backUpPath, platform, resPaths, lans := models.GetCdnData(
		command.ProjectName, svnProjectName)
	if nil != err {
		return "", err
	}

	//如果没有配置语言，那就是空字符
	strLans := make([]string, 0)
	for _, v := range lans {
		strLans = append(strLans, strconv.Itoa(v))
	}
	if len(strLans) <= 0 {
		//没有配置语言，则插入一条空字符，以便统一循环执行
		strLans = append(strLans, "")
	}

	if len(resPaths) < 2 {
		return "", errors.New("没有配置正式资源地址，上传正式资源失败！")
	}
	fileName := tool.DealZZZZZName(models.CLIENTFILESNAME, zzzTimeStamp) + ".txt"

	//获取cdn对象
	cdnErr, cdnClient := cdn.GetCdnClient(cdnType, urlOfBucket, bucketName, accessKeyID, accessKeySecret)
	if nil != cdnErr {
		return "", cdnErr
	}

	//获取服务器资源地址files.txt列表
	resFileList := make(map[string]string)
	for _, _path := range resPaths {
		if _path == "" {
			continue
		}
		for _, lan := range strLans {
			remoteHotfixedFilesPath := path.Join(_path, platform, lan, fileName)
			fileErr, files := cdnClient.DownFile(remoteHotfixedFilesPath)
			if nil != fileErr {
				return "", errors.New(fmt.Sprintf("获取files.txt文件失败，资源路径：%s,错误原因：%s", remoteHotfixedFilesPath, fileErr.Error()))
			}
			//解密
			files = tool.XOREncryptionData(files, fileDecodeKey)
			resFileList[remoteHotfixedFilesPath] = string(files)
		}
	}

	//先备份(就取第一个正式地址)
	for _, lan := range strLans {
		backResult, err := BackRes(zzzTimeStamp, fileDecodeKey, backUpPath, resPaths[1], platform, lan, cdnClient)
		if err != nil {
			return "", errors.New("备份异常：" + err.Error())
		}
		command.SendResult(backResult, "")
	}

	//拷贝cdn上文件
	lastTime := time.Now().Unix()
	copyFunc := func(testPath, resPath, fileName, copyResult string, count int) (error, string) {
		testFilesPath := path.Join(testPath, fileName)
		targetFilePath := path.Join(resPath, fileName)
		err := cdnClient.CopyFile(testFilesPath, targetFilePath)
		if nil == err {
			count++
			cs := fmt.Sprintf("拷贝%s成功\n", fileName)
			copyResult += cs
			log.Info(cs)
		} else {
			return errors.New(fmt.Sprintf("拷贝%s失败，原因%s\n", fileName, err.Error())), copyResult
		}

		//避免消息过长，钉钉截掉
		timeNow := time.Now().Unix()
		if count >= lineInOneMes || (timeNow-lastTime) > int64(maxIntervalTimeBetween2Msg) {
			command.SendResult(copyResult, "")
			copyResult = ""
			count = 0
			lastTime = timeNow
		}
		return nil, copyResult
	}

	//修改包配置
	packageConfigIds, packageConfigSvrs := models.GetPackageConfigSvr(command.ProjectName, svnProjectName)
	modifyPackageConfig := func(request string) string {
		_error, _result := tool.Http("POST", request, "")
		if _error != nil {
			return "请求失败：" + _error.Error()
		}
		if string(_result) == "" {
			return "结果为空"
		}

		// 解析包配置返回结果
		var PackageResult struct {
			Result int    `json:"Result"` //返回结果
			Error  string `json:"Error"`
		}
		err := json.Unmarshal(_result, &PackageResult)
		if err != nil {
			return "解析结果失败：" + err.Error()
		}
		if PackageResult.Error != "" {
			return "返回失败:" + PackageResult.Error
		} else {
			return ""
		}
	}

	//从测试地址拷贝资源到正式地址
	result := ""
	filesMd5 := ""
	testPath := resPaths[0]
	for _, lan := range strLans {
		testHotfixedPath := path.Join(testPath, platform, lan)
		testFilesPath := path.Join(testHotfixedPath, fileName)
		testFiles := resFileList[testFilesPath]
		tempMd5 := tool.CalcMd5(testFiles)

		//将测试地址资源依次拷贝到正式地址
		for i := 1; i < len(resPaths); i++ {
			//拷贝文件
			count := 0
			resPath := resPaths[i]
			configLen := len(packageConfigIds)
			packageConfigSvr := ""
			packageConfigId := ""
			if configLen > 0 {
				packageIndex := i - 1
				if packageIndex >= configLen {
					packageIndex = configLen - 1
				}
				packageConfigSvr = packageConfigSvrs[packageIndex]
				packageConfigId = packageConfigIds[packageIndex]
			}
			isNeedUpdatePackageConfig := packageConfigSvr != "" && packageConfigId != ""
			resHotfixedPath := path.Join(resPath, platform, lan)
			_SuccessResult := fmt.Sprintf("开始从%s拷贝资源到%s\n", testHotfixedPath, resHotfixedPath)

			//比对出不一样的资源并拷贝
			resFilesPath := path.Join(resHotfixedPath, fileName)
			err, needUpdateFiles := models.GetNeedUpdateDatas(testFiles, resFileList[resFilesPath])
			if err != nil {
				log.Error(fmt.Sprintf("testHotfixedPath:%s,resHotfixedPath:%s", testHotfixedPath, resHotfixedPath))
				return "", err
			}
			if len(needUpdateFiles) <= 0 {
				result += resHotfixedPath + "没有需要更新的资源\n"
				continue
			}

			//走到这里表示有更新，先关闭热更（要不虽然最后上传files文件，在上传资源期间老玩家不会热更，但是因为更新过，所以新玩家一定还会热更，此时热更的资源如果更新了比对md5不通过导致下载异常）
			if isNeedUpdatePackageConfig {
				packageConfigReq := fmt.Sprintf("%s/versioncontrol?action=%s&id=%s&isOpen=0", packageConfigSvr, "modify_updateflag", packageConfigId)
				updateRes := modifyPackageConfig(packageConfigReq)
				if updateRes != "" {
					_SuccessResult += "关闭更新失败：" + updateRes + "\n"
				} else {
					_SuccessResult += "临时关闭更新成功" + "\n"
				}
			}

			//拷贝资源
			for fileName, _ := range needUpdateFiles {
				err, _SuccessResult = copyFunc(testHotfixedPath, resHotfixedPath, fileName, _SuccessResult, count)
				if err != nil {
					return "", err
				}
			}

			//拷贝files.txt
			err, _SuccessResult = copyFunc(testHotfixedPath, resHotfixedPath, fileName, _SuccessResult, count)
			if err != nil {
				return "", err
			}

			//更新包配置md5
			if isNeedUpdatePackageConfig {
				packageConfigReq := fmt.Sprintf("%s/versioncontrol?action=%s&id=%s&filesMd5=%s", packageConfigSvr, "modify_filesmd5", packageConfigId, tempMd5)
				updateRes := modifyPackageConfig(packageConfigReq)
				if updateRes != "" {
					result += "更新包配置filesmd5失败：" + updateRes
				} else {
					result += "更新包配置filesmd5成功.更新已恢复"
				}
			}

			_SuccessResult += fmt.Sprintf("从%s拷贝资源到%s结束,files md5:%s\n", testHotfixedPath, resPath, tempMd5)
			_SuccessResult += "***************************************************************************************\n"
			command.SendResult(_SuccessResult, "")
		}

		if len(strLans) <= 1 {
			//老包就一个字段赋值md5
			filesMd5 = tempMd5
		} else {
			//兼容老包，所以用一个字段记录多个语言的md5
			filesMd5 += fmt.Sprintf("%s%s:%s|", platform, lan, tempMd5)
		}

	}

	result += "上传正式热更资源结束。"
	return result, nil
}

func ListAllSvnLog(command *operationCommand) (string, error) {
	//获取项目配置
	err, params := models.AnalysisParam(command.CommandParams, command.CommandType)
	if nil != err {
		return "", err
	}
	svnProjectName := params[0]
	err, _, svnPath, _ := models.GetSvnProjectInfo(command.ProjectName, svnProjectName)
	if nil != err {
		return "", err
	}

	//判断是否有时间参数
	startTimeStamp := models.GetSvnLogTime(command.ProjectName, svnProjectName)
	endTimeStamp := int64(0)
	if len(params) > 1 {
		timeStr := params[1]
		timeStrs := strings.Split(timeStr, "-")
		if len(timeStrs) >= 2 {
			startTimeStamp, _ = strconv.ParseInt(timeStrs[0], 10, 64)
			endTimeStamp, _ = strconv.ParseInt(timeStrs[1], 10, 64)
		} else {
			startTimeStamp, _ = strconv.ParseInt(timeStr, 10, 64)
		}
	}

	//获取svn日志的开始和截止日期参数
	var _startTime time.Time
	if startTimeStamp <= 0 {
		//默认10天前
		currentTime := time.Now()
		_startTime = currentTime.AddDate(0, 0, -10)
	} else {
		_startTime = time.Unix(int64(startTimeStamp), 0)
	}
	startDateStr := _startTime.Format("2006-01-02 15:04:05")
	endDateStr := "HEAD"
	if endTimeStamp > 0 {
		endDateStr = fmt.Sprintf("{'%s'}", time.Unix(int64(endTimeStamp), 0).Format("2006-01-02 15:04:05"))
	}

	executable := "sh"
	if runtime.GOOS == "windows" {
		executable = winGitPath
	}

	//获取svn路径日志
	commandTxt := fmt.Sprintf("svn log -r {'%s'}:%s %s", startDateStr, endDateStr, svnPath)
	log.Info(commandTxt)
	commandResult, _ := tool.ExecSimpleCmd(executable, commandTxt)

	//解析该svn路径所有日志
	defaultLogType := "优化"
	defaultSysType := "unknowSystem"
	logFilters := strings.Split(beego.AppConfig.DefaultString("logFilter", ""), "|")
	ratLog := make(map[string]*models.SvnLog)
	svnLogs := strings.Split(commandResult, "------------------------------------------------------------------------")
	for _, v := range svnLogs {
		if v == "" {
			continue
		}

		//原始格式：版本、提交者、提交日期、内容以|分割
		logItem := strings.Split(v, "|")
		if len(logItem) < 4 {
			continue
		}
		//version := logItem[0]
		author := logItem[1]
		//date := logItem[2]

		//解析日志内容，原始格式：行数\r\n日志内容
		logItem[3] = strings.ReplaceAll(logItem[3], "\r", "")
		content := strings.Split(logItem[3], "\n")
		if len(content) < 4 {
			continue
		}

		//处理日志
		for i := 2; i < len(content); i++ {
			_logContent := content[i]
			if _logContent == "" {
				continue
			}

			//过滤掉不需要的日志
			isFilter := false
			for _, logFil := range logFilters {
				if strings.Contains(_logContent, logFil) {
					isFilter = true
					break
				}
			}
			if isFilter {
				continue
			}

			//获取日志的类型和系统
			temps := strings.Split(_logContent, "#")
			logType := defaultLogType
			logSysType := defaultSysType
			oneLineLog := _logContent
			if len(temps) > 2 {
				logType = temps[0]
				logSysType = temps[1]
				oneLineLog = ""
				for i := 2; i < len(temps); i++ {
					//#前添加转义字符，分割后这一行就是转移字符，所以替换回#号
					if temps[i] == "\\" {
						oneLineLog += "#"
					} else {
						oneLineLog += temps[i]
					}
				}
			}

			//初始化svn日志数据结构
			logType = strings.ToLower(logType)
			if _, ok := ratLog[logType]; !ok {
				//不存在该提交类型
				ratLog[logType] = new(models.SvnLog)
				ratLog[logType].LogType = logType
				ratLog[logType].Logs = make(map[string]map[string][]string)
			}
			if _, ok := ratLog[logType].Logs[logSysType]; !ok {
				//不存在该系统类型
				ratLog[logType].Logs[logSysType] = make(map[string][]string)
			}
			if _, ok := ratLog[logType].Logs[logSysType][author]; !ok {
				//不存在该提交作者
				ratLog[logType].Logs[logSysType][author] = make([]string, 0)
			}

			//添加日志内容
			ratLog[logType].Logs[logSysType][author] = append(ratLog[logType].Logs[logSysType][author], oneLineLog)
		}
	}

	//将该svn路径日志以一定的格式构建群
	svnLog := ""
	for _, logOfOneType := range ratLog {
		//按日志类型整理日志
		index := 0
		svnLog += fmt.Sprintf("%s:\n", logOfOneType.LogType)
		for logSysType, LogMapOfSysType := range logOfOneType.Logs {
			//按系统模块整理日志
			for author, logMapOfAuthor := range LogMapOfSysType {
				//按每个提交者整理日志
				for _, log := range logMapOfAuthor {
					index++
					svnLog += fmt.Sprintf("  %d、%s(%s by %s)\n", index, log, logSysType, author)
				}
			}
		}
	}
	svnLog += "\n"
	result := fmt.Sprintf("从%s到%s的svn日志:\n%s", startDateStr, endDateStr, svnLog)

	//保存svn截止日期（后面命令没有参数就是默认从这个时间到最新日志）
	if endTimeStamp <= 0 {
		endTimeStamp = time.Now().Unix()
	}
	models.SaveSvnLogTime(command.ProjectName, svnProjectName, endTimeStamp)
	return result, nil
}

func PrintHotfixResList(command *operationCommand) (string, error) {
	//获取参数
	err, params := models.AnalysisParam(command.CommandParams, command.CommandType)
	if nil != err {
		return "", err
	}
	if len(params) < 1 {
		return "", errors.New("参数不足，必须指定项目名称!")
	}

	//获取项目信息
	svnProjectName := params[0]
	err, projectPath, _, _ := models.GetSvnProjectInfo(command.ProjectName, svnProjectName)
	if err != nil {
		return "", err
	}

	//再获取cdn配置
	err, cdnType, urlOfBucket, bucketName, zzzTimeStamp, fileDecodeKey, accessKeyID, accessKeySecret, _, platform, resPaths, lans := models.GetCdnData(
		command.ProjectName, svnProjectName)
	if nil != err {
		return "", err
	}

	//获取热更语言
	language := ""
	if len(params) > 1 {
		language = params[1]
		_, err = strconv.Atoi(language)
		if err != nil {
			return "", errors.New("转换语言失败,只能是数字：" + err.Error())
		}
	} else {
		return "", errors.New("请指定语言！！")
	}

	fileName := tool.DealZZZZZName(models.CLIENTFILESNAME, zzzTimeStamp) + ".txt"
	originalFileName := tool.DealZZZZZName(models.OriginalFILESNAME, zzzTimeStamp) + ".txt"

	//获取cdn对象
	cdnErr, cdnClient := cdn.GetCdnClient(cdnType, urlOfBucket, bucketName, accessKeyID, accessKeySecret)
	if nil != cdnErr {
		return "", cdnErr
	}

	//获取热更数据对象
	var updateData *models.UpdateDataOfProject
	ok := false
	models.ClientHotFixedDataLock.Lock()
	defer models.ClientHotFixedDataLock.Unlock()
	hotDataKey := models.GetHotFixedMapKey(svnProjectName, platform, language)
	if updateData, ok = models.ClientHotFixedFileDataTempMap[hotDataKey]; !ok {
		updateData = new(models.UpdateDataOfProject)
		updateData.UpdateSuccessFiles = make(map[string]bool)
	}

	//判断cdn服务器上目标files.txt是否存在
	isNeedUpdate := false
	result := ""

	//上传files
	temLan := language
	if nil == lans || len(lans) <= 0 {
		temLan = ""
	}
	hotfixedPath := models.GetHotFixedResPath(projectPath, zzzTimeStamp, platform, temLan)
	upLoadFiles := func(_platform, _lan, _fileName string) (string, error) {
		//判断本地files.txt是否存在
		localFilesPath := path.Join(hotfixedPath, _fileName)
		if !tool.CheckFileIsExist(localFilesPath) {
			log.Info("本地不存在files文件：" + localFilesPath)
			return "", nil
		}
		for _, resPath := range resPaths {
			//判断服务器files文件
			remoteHotfixedFilesPath := path.Join(resPath, _platform, _lan, _fileName)
			isExist, fileExistErr := cdnClient.IsExistFile(remoteHotfixedFilesPath)
			if nil != fileExistErr {
				log.Error(fmt.Sprintf("判断测试files.txt文件是否存在错误，资源路径：%s,错误原因：%s", remoteHotfixedFilesPath, fileExistErr.Error()))
				return "", fileExistErr
			}

			//如果存在则后面要比对更新
			if isExist {
				isNeedUpdate = true
				continue
			}

			//如果不存在则上传本地files
			uploadFileErr := cdnClient.UploadFile(localFilesPath, remoteHotfixedFilesPath)
			if nil == uploadFileErr {
				result += fmt.Sprintf("cdn服务器不存在%s文件,已上传本地文件到cdn服务器\n", remoteHotfixedFilesPath)
			} else {
				return "", errors.New(fmt.Sprintf("cdn服务器不存在%s文件且上传本地文件失败，错误原因：%s\n", remoteHotfixedFilesPath, uploadFileErr.Error()))
			}
		}
		return "", nil
	}
	_, err = upLoadFiles(platform, temLan, fileName)
	if err != nil {
		return "", err
	}
	if zzzTimeStamp != "" {
		_, err = upLoadFiles(platform, temLan, originalFileName)
		if err != nil {
			return "", err
		}
	}

	if !isNeedUpdate {
		//不存在files.txt，都是最新的，没有需要热更的东西
		return result, nil
	}

	//获取本地files.txt文件
	localFilesPath := path.Join(hotfixedPath, fileName)
	localFile, err := os.Open(localFilesPath)
	if err != nil {
		fmt.Println("读取项目本地file.txt失败 os.Open:", err)
		return "", errors.New("读取项目本地file.txt失败 os.Open:" + err.Error())
	}
	localFileByts, err := ioutil.ReadAll(localFile)
	localFile.Close()
	if err != nil {
		return "", errors.New("读取file.txt失败 ioutil.ReadAll:" + err.Error())
	}

	//获取服务器测试热更files.txt /
	remoteTestHotfixedFilesPath := path.Join(resPaths[0], platform, temLan, fileName)
	testFileErr, testFiles := cdnClient.DownFile(remoteTestHotfixedFilesPath)
	if nil != testFileErr {
		return "", errors.New(fmt.Sprintf("获取测试files.txt文件失败，资源路径：%s,错误原因：%s", remoteTestHotfixedFilesPath, testFileErr.Error()))
	}

	//解密文件
	localFileByts = tool.XOREncryptionData(localFileByts, fileDecodeKey)
	testFiles = tool.XOREncryptionData(testFiles, fileDecodeKey)

	//根据本地和服务器files。txt文件比对获取需要更新的数据
	err, needUpdateHotfixedFilesDataMap := models.GetNeedUpdateDatas(string(localFileByts), string(testFiles))
	if err != nil {
		return "", err
	}

	//获取混淆文件隐射表
	zzzMapping, err := tool.GetOriginalAndZZZName(path.Join(projectPath, "zzzzzName.txt"), zzzTimeStamp)
	if err != nil {
		result += err.Error()
	}

	//缓存数据并返回结果给钉钉群
	updateData.HotFiexedDatas = make([]*models.ClientHotFixedFileData, 0)
	temp := "需要更新的文件有：\n"
	count := 0
	totalSize := 0
	for k, v := range needUpdateHotfixedFilesDataMap {
		updateData.HotFiexedDatas = append(updateData.HotFiexedDatas, v)
		count++
		fileSize, _ := strconv.Atoi(v.Size)
		totalSize += fileSize
		fixFileName := k
		tempName := strings.Split(fixFileName, ".")[0]
		if _originalName, ok := zzzMapping[tempName]; ok {
			fixFileName = _originalName
		}
		temp += fmt.Sprintf("%s≈%dKB\n", fixFileName, (fileSize / 1024))

		//避免消息过长，钉钉截掉
		if count >= lineInOneMes {
			command.SendResult(temp, "")
			temp = ""
			count = 0
		}
	}
	result += temp
	models.ClientHotFixedFileDataTempMap[hotDataKey] = updateData

	//拷贝所有热更资源到项目临时文件夹
	_, hotResTempFolderName := models.GetHotFixedResTempFolderName(command.ProjectName, svnProjectName)
	if hotResTempFolderName != "" && len(updateData.HotFiexedDatas) > 0 {
		tempFolder := path.Join(projectPath, hotResTempFolderName)
		if exist, _ := tool.PathExists(tempFolder); exist {
			os.RemoveAll(tempFolder)
			tool.CreateDir(tempFolder)
		}

		for _, v := range updateData.HotFiexedDatas {
			resPath := path.Join(hotfixedPath, v.Name)
			tarPath := path.Join(tempFolder, v.Name)
			parentPath := filepath.Dir(tarPath)
			isExist, _ := tool.PathExists(parentPath)
			if !isExist {
				dirErr := tool.CreateDir(parentPath)
				if nil != dirErr {
					result += fmt.Sprintf("热更资源拷贝到临时文件夹%s失败:%s！\n", resPath, dirErr.Error())
					continue
				}
			}
			_, err := tool.CopyFile(resPath, tarPath)
			if err != nil {
				result += fmt.Sprintf("拷贝资源到临时目录失败：%s", v.Name)
			}
		}

		//最后拷贝files.txt
		tool.CopyFile(path.Join(hotfixedPath, fileName), path.Join(tempFolder, fileName))
		if zzzTimeStamp != "" {
			tool.CopyFile(path.Join(hotfixedPath, originalFileName), path.Join(tempFolder, originalFileName))
		}

		result += fmt.Sprintf("拷贝热更文件到%s完毕：", tempFolder)
	}

	//最终结果
	result += fmt.Sprintf("\ntotalsize:%dMB\ntotaCount:%d\nfiles.txtmd5：%s", totalSize/1048576, len(needUpdateHotfixedFilesDataMap),
		tool.CalcMd5(string(localFileByts)))
	return result, nil
}

func BackRes(zzzTimeStamp, fileDecodeKey, backupPath, remotePath, platform, language string, cdnClient cdn.IcdnClient) (string, error) {
	//如果本地不存在备份文件夹则创建
	if backupPath == "" {
		log.Error("备份文件夹为空，不备份！")
		return "", nil
	}
	backupPath = path.Join(backupPath, platform, language)
	remotePath = path.Join(remotePath, platform, language)
	isExist, err := tool.PathExists(backupPath)
	if nil != err {
		return "", err
	}
	if !isExist {
		dirErr := tool.CreateDir(backupPath)
		if nil != dirErr {
			return "", dirErr
		}
	}

	//先更新本地备份文件
	//result := ""
	//executable := "sh"
	//if runtime.GOOS == "windows" {
	//	executable = winGitPath
	//}
	//svnUpdateCommand := fmt.Sprintf("cd %s;svn up .;", backupPath)
	//svnUpdateResult, _ := tool.ExecSimpleCmd(executable, svnUpdateCommand)

	//判断服务器有没有正式资源，没有则不备份
	fileName := tool.DealZZZZZName(models.CLIENTFILESNAME, zzzTimeStamp) + ".txt"
	remoteFilesPath := path.Join(remotePath, fileName)
	isRemoteExist, err := cdnClient.IsExistFile(remoteFilesPath)
	if err != nil {
		return "", err
	}
	if !isRemoteExist {
		return "", errors.New("服务器不存在files文件，备份终止！")
	}

	//判断本地是否有files文件，没有直接下载最新
	backupFilesPath := path.Join(backupPath, fileName)
	if !tool.CheckFileIsExist(backupFilesPath) {
		//不存在则下载最新文件
		err = cdnClient.DownFile2Local(remoteFilesPath, backupFilesPath)
		if err != nil {
			return "", errors.New("下载远端files失败，原因：" + err.Error())
		} else {
			return "备份本地没有最新files,已下载最新", nil
		}
	}

	//获取本地files.txt文件
	backupFilesFile, err := os.Open(backupFilesPath)
	if err != nil {
		fmt.Println("读取备份file.txt失败 os.Open:", err)
		return "", errors.New("读取备份file.txt失败 os.Open:" + err.Error())
	}
	backupFilesFileByts, err := ioutil.ReadAll(backupFilesFile)
	backupFilesFile.Close()
	if err != nil {
		return "", errors.New("读取备份file.txt失败 ioutil.ReadAll:" + err.Error())
	}

	//获取服务器正式热更files.txt
	remoteFileErr, remoteFiles := cdnClient.DownFile(remoteFilesPath)
	if nil != remoteFileErr {
		return "", errors.New(fmt.Sprintf("获取正式files.txt文件失败，资源路径：%s,错误原因：%s", remoteFilesPath, remoteFileErr.Error()))
	}

	//解密
	backupFilesFileByts = tool.XOREncryptionData(backupFilesFileByts, fileDecodeKey)
	remoteFiles = tool.XOREncryptionData(remoteFiles, fileDecodeKey)

	//根据本地和服务器files。txt文件比对获取需要下载的数据
	err, needDownloadHotfixedFilesDataMap := models.GetNeedUpdateDatas(string(backupFilesFileByts), string(remoteFiles))
	if err != nil {
		return "", err
	}
	if len(needDownloadHotfixedFilesDataMap) <= 0 {
		return "本地已是最新，不需要备份", nil
	}

	//下载跟本地不一样的文件
	result := fmt.Sprintf("备份开始，本次共%d个文件需要更新：\n", len(needDownloadHotfixedFilesDataMap))
	for k, _ := range needDownloadHotfixedFilesDataMap {
		remoteFilePath := path.Join(remotePath, k)
		downloadedFileName := path.Join(backupPath, k)
		parentPath := filepath.Dir(downloadedFileName)
		isExist, _ := tool.PathExists(parentPath)
		if !isExist {
			dirErr := tool.CreateDir(parentPath)
			if nil != dirErr {
				result += fmt.Sprintf("备份%s/%s失败:%s！\n", language, k, dirErr.Error())
				continue
			}
		}
		err := cdnClient.DownFile2Local(remoteFilePath, downloadedFileName)
		if err == nil {
			result += fmt.Sprintf("备份%s/%s成功！\n", language, k)
		} else {
			result += fmt.Sprintf("备份%s/%s失败！\n", language, k)
		}
	}

	//再下载files.txt
	err = cdnClient.DownFile2Local(remoteFilesPath, backupFilesPath)
	if err == nil {
		result += "备份files成功！\n"
	} else {
		result += "备份files失败！\n"
	}

	////上传svn
	//svnLog := fmt.Sprintf("md5:%s \n", tool.CalcFileMd5(backupFilesPath))
	//requestParams := strings.Split(command.CommandParams, ",")
	//if len(requestParams) > 2 {
	//	svnLog += requestParams[1]
	//}
	//commitCommand := fmt.Sprintf("cd %s;chmod +x svnCommit.sh;./svnCommit.sh %s %s", shellPath, backupPath, svnLog)
	//commitResult, _ := tool.ExecSimpleCmd(executable, commitCommand)
	//result += commitResult
	//result += "备份完毕！"
	return result, nil
}
