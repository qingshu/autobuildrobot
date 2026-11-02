package models

import (
	log "autobuildrobot/mylog"
	"autobuildrobot/tool"
	"errors"
	"fmt"
	"strings"
	"sync"
)

// 游戏服务器主机配置
type SvrMachineModel struct {
	MachineName string `json:"MachineName"` //服务器主机名
	Platform    string `json:"Platform"`    //构建目标平台
	Ip          string `json:"Ip"`          //ip
	Port        string `json:"Port"`        //端口
	Account     string `json:"Account"`     //账号
	Psd         string `json:"Psd"`         //密码
	SvrRootPath string `json:"SvrRootPath"` //服务器根目录
}

var lastSvrMachineConfigFileName string             //上一次的服务器主机配置数据文件名（基本一个项目一个文件）
var svrMachineConfigMap map[string]*SvrMachineModel //服务器主机配置字典，key 服务器主机名 value:服务器主机配置
var svrMachineDataLock sync.Mutex

// 有就更新，没有则添加
func UpdateSvrMachineData(projectName, svrConfig string) (result string) {
	svrMachineDataLock.Lock()
	defer svrMachineDataLock.Unlock()

	//先获取数据
	var fileName string
	fileName, svrMachineConfigMap = getProjectSvrMachineData(projectName)

	//再更新数据
	svrArr := strings.Split(svrConfig, ";")
	tempSvr := ""
	for _, svr := range svrArr {
		if svr == "" {
			continue
		}
		if !strings.HasSuffix(svr, "}") {
			//处理配置中有特殊分号字符
			tempSvr += svr + ";"
			continue
		}
		svr = tempSvr + svr
		tempSvr = ""
		svrModel := new(SvrMachineModel)
		tool.UnmarshJson([]byte(svr), &svrModel)
		if svrModel.MachineName == "" {
			errMsg := "svr主机名不能为空：" + svr
			log.Error(errMsg)
			result += (errMsg + "\n")
			continue
		}

		//删除配置
		if strings.Contains(svrModel.MachineName, "-") {
			//负号作为删除标记吧
			delMachine := strings.ReplaceAll(svrModel.MachineName, "-", "")
			if strings.Contains(delMachine, "*") {
				delMachine = strings.ReplaceAll(delMachine, "*", "")
				delKeys := make([]string, 0)
				for k, _ := range svrMachineConfigMap {
					if strings.Contains(k, delMachine) {
						delKeys = append(delKeys, k)
					}
				}
				for _, k := range delKeys {
					delete(svrMachineConfigMap, k)
				}
			} else {
				delete(svrMachineConfigMap, delMachine)
			}
			continue
		}

		//判断svn工程是否存在
		if !JudgeSvnProjectIsExist(projectName, svrModel.MachineName) {
			result += fmt.Sprintf("不存在%s工程，请先用【%s】指令添加！\n", svrModel.MachineName, commandName[CommandType_SvnProjectConfig])
			continue
		}

		//增加或修改
		if _svrModel, ok := svrMachineConfigMap[svrModel.MachineName]; ok {
			//已存在，如果数据为空则用老数据
			if svrModel.Ip == "" {
				svrModel.Ip = _svrModel.Ip
			}
			if svrModel.Platform == "" {
				svrModel.Platform = _svrModel.Platform
			}
			if svrModel.Account == "" {
				svrModel.Account = _svrModel.Account
			}
			if svrModel.Psd == "" {
				svrModel.Psd = _svrModel.Psd
			}
			if svrModel.Port == "" {
				svrModel.Port = _svrModel.Port
			}
			if svrModel.SvrRootPath == "" {
				svrModel.SvrRootPath = _svrModel.SvrRootPath
			}
		}
		svrMachineConfigMap[svrModel.MachineName] = svrModel
	}

	//编码并存储
	tool.SaveGobFile(fileName, svrMachineConfigMap)
	if result != "" {
		return
	}
	return "更新主机配置成功"
}

// 获取一个项目所有服务器主机配置信息
func QuerySvrMachineDataOfOneProject(projectName, searchValue string) (result string) {
	svrMachineDataLock.Lock()
	defer svrMachineDataLock.Unlock()
	_, svrMachineConfigMap = getProjectSvrMachineData(projectName)
	if len(svrMachineConfigMap) <= 0 {
		return "当前没有主机配置信息，请配置：\n" + GetSvrMachineConfigHelp()
	}

	tpl := SvrMachineModel{}
	for _, v := range svrMachineConfigMap {
		if !JudgeIsSearchAllParam(searchValue) && !strings.Contains(v.MachineName, searchValue) {
			continue
		}
		tpl.MachineName = v.MachineName
		tpl.Ip = v.Ip
		tpl.Platform = v.Platform
		tpl.Account = v.Account
		tpl.Psd = v.Psd
		tpl.Port = v.Port
		tpl.SvrRootPath = v.SvrRootPath
		result += fmt.Sprintln(tool.MarshalJson(tpl) + "\n")
	}

	if result == "" {
		return "当前没有符合条件的主机配置信息，请配置：\n" + GetSvrMachineConfigHelp()
	} else {
		return "\n***********************以下是主机配置数据***********************\n" + result
	}
}

// 获取服务器主机配置帮助提示
func GetSvrMachineConfigHelp() string {
	tpl := SvrMachineModel{
		MachineName: "指令【" + commandName[CommandType_SvnProjectConfig] + "】为了节省关联字段，所以这里跟其一样名称关联",
		Platform:    "构建目标平台，如linux windows darwin等",
		Ip:          "ip",
		Port:        "端口",
		Psd:         "密码",
		Account:     "账号",
		SvrRootPath: "服务器根目录",
	}
	return fmt.Sprintf("服务器主机配置可理解为更新服务器需要的svn工程配置的补充字段吧\n配置例子：\n【%s：%s】\n如果多个配置用分号分割",
		commandName[CommandType_SvrMachineConfig], tool.MarshalJson(tpl))
}

// 获取服务器主机配置数据
func GetSvrMachineData(projectName, svrMachineName string) (err error, ip, port, account, psd, platform, svrRootPath string) {
	svrMachineDataLock.Lock()
	defer svrMachineDataLock.Unlock()
	if svrMachineName == "" {
		err = errors.New("获取主机配置失败，主机名不能为空！")
		return
	}
	_, svrMachineConfigMap = getProjectSvrMachineData(projectName)
	if _svrModel, ok := svrMachineConfigMap[svrMachineName]; ok {
		return nil, _svrModel.Ip, _svrModel.Port, _svrModel.Account, _svrModel.Psd, _svrModel.Platform, _svrModel.SvrRootPath
	} else {
		err = errors.New(svrMachineName + "主机配置不存在，请添加！")
		return
	}
}

// 根据项目名获取svr文件名和数据
func getProjectSvrMachineData(projectName string) (string, map[string]*SvrMachineModel) {
	svrDataFileName := "svrMachine.gob"
	fileName := tool.CalcMd5(projectName) + svrDataFileName
	if fileName == lastSvrMachineConfigFileName {
		return fileName, svrMachineConfigMap
	}
	svrMachineConfigMap = make(map[string]*SvrMachineModel)
	tool.ReadGobFile(fileName, &svrMachineConfigMap)
	lastSvrMachineConfigFileName = fileName
	return fileName, svrMachineConfigMap
}
