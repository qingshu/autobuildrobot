package models

import (
	"autobuildrobot/tool"
	"fmt"
	"strings"
	"sync"
)

// 模板指令
type CmdTemplateModel struct {
	Name     string `json:"Name"`     //模板名称
	Cmd      string `json:"Cmd"`      //指令
	Describe string `json:"Describe"` //描述
}

var lastCmdTempFileName string              //上一个项目的模板指令数据文件名（基本一个项目一个模板指令数据文件）
var cmdTempMap map[string]*CmdTemplateModel //模板指令配置字典，key 模板名 value:指令
var cmdTempDataLock sync.Mutex

// 有就更新，没有则添加
func UpdateCmdTemp(projectName, cmdTempConfig string) (result string) {
	cmdTempDataLock.Lock()
	defer cmdTempDataLock.Unlock()

	//先获取数据
	var fileName string
	fileName, cmdTempMap = getCmdTempsDataByProjectName(projectName)

	//再更新数据
	cmdTempArr := strings.Split(cmdTempConfig, ";")
	for _, cmdTemp := range cmdTempArr {
		if cmdTemp == "" {
			continue
		}
		cmdTempModel := new(CmdTemplateModel)
		tool.UnmarshJson([]byte(cmdTemp), &cmdTempModel)
		if strings.Contains(cmdTempModel.Name, "-") {
			//负号作为删除标记吧
			delCmdTemp := strings.ReplaceAll(cmdTempModel.Name, "-", "")
			if strings.Contains(delCmdTemp, "*") {
				delCmdTemp = strings.ReplaceAll(delCmdTemp, "*", "")
				delKeys := make([]string, 0)
				for k, _ := range cmdTempMap {
					if strings.Contains(k, delCmdTemp) {
						delKeys = append(delKeys, k)
					}
				}
				for _, k := range delKeys {
					delete(cmdTempMap, k)
				}
			} else {
				delete(cmdTempMap, delCmdTemp)
			}
			continue
		}
		if cmdTempModel.Name == "" || cmdTempModel.Cmd == "" {
			return "模板名称或者指令不能为空：" + cmdTemp
		}

		cmdTempMap[cmdTempModel.Name] = cmdTempModel
	}

	//编码并存储
	tool.SaveGobFile(fileName, cmdTempMap)
	result = "更新模板指令配置成功"
	return
}

// 获取一个项目所有模板指令配置信息
func QueryCmdTempsDataByProject(projectName, searchValue string) (result string) {
	cmdTempDataLock.Lock()
	defer cmdTempDataLock.Unlock()

	_, cmdTempMap = getCmdTempsDataByProjectName(projectName)
	if len(cmdTempMap) <= 0 {
		return "当前没有模板指令信息，请配置：\n" + GetCmdTempConfigHelp()
	}

	for _, v := range cmdTempMap {
		if !JudgeIsSearchAllParam(searchValue) && !strings.Contains(v.Name, searchValue) {
			continue
		}
		result += fmt.Sprintln(tool.MarshalJson(v) + "\n")
	}
	if result == "" {
		return "当前没有符合条件的模板指令配置信息，请配置：\n" + GetCmdTempConfigHelp()
	} else {
		return "\n***********************以下是模板指令配置数据***********************\n" + result
	}
}

// 获取模板指令配置帮助提示
func GetCmdTempConfigHelp() string {
	tpl := CmdTemplateModel{
		Name:     "预定义指令名称，如：合并到测试",
		Cmd:      "预定义指令，如：分支合并：开发分支合并到策划分支->分支合并：策划分支合并到测试分支",
		Describe: "预定义指令描述信息",
	}
	jsonTpl := tool.MarshalJson(tpl)
	jsonTpl = strings.ReplaceAll(jsonTpl, "》", ">")
	return fmt.Sprintf("模板指令就是预定义一些指令,直接输入指令名称，机器人就会执行该预定义指令。\n配置例子：\n【%s：%s】 \n如多条指令用英文分号拼接",
		commandName[CommandType_TemplateCmd], jsonTpl)
}

// 获取模板指令
func GetTemplateCmd(projectName, cmdTempName string) string {
	//获取模板指令
	cmdTempDataLock.Lock()
	defer cmdTempDataLock.Unlock()
	svnProjectModel := getCmdTempData(projectName, cmdTempName)
	if nil == svnProjectModel {
		return ""
	}
	return svnProjectModel.Cmd
}

// 获取工程配置
func getCmdTempData(projectName, cmdTempName string) *CmdTemplateModel {
	if projectName == "" || cmdTempName == "" {
		return nil
	}
	_, cmdTempMap = getCmdTempsDataByProjectName(projectName)
	cmdTempName = strings.TrimSpace(cmdTempName)
	if _svnProjectModel, ok := cmdTempMap[cmdTempName]; ok {
		return _svnProjectModel
	} else {
		//log.Error(cmdTempName, "模板指令配置不存在，请添加")
		return nil
	}
}

// 根据webHook获取该项目模板指令数据文件名和数据
func getCmdTempsDataByProjectName(projectName string) (string, map[string]*CmdTemplateModel) {
	cmdTempDataFileName := "cmdTemp.gob"
	fileName := tool.CalcMd5(projectName) + cmdTempDataFileName
	if fileName == lastCmdTempFileName {
		return fileName, cmdTempMap
	}
	cmdTempMap = make(map[string]*CmdTemplateModel)
	tool.ReadGobFile(fileName, &cmdTempMap)
	lastCmdTempFileName = fileName
	return fileName, cmdTempMap
}
