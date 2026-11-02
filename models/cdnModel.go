package models

import (
	"autobuildrobot/tool"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
)

// CDN配置
type CdnModel struct {
	CdnName                   string   `json:"CdnName"`                   //工程名称（对应SvnProjectModel工程名）
	CdnType                   string   `json:"CdnType"`                   //cdn类型,对应cdn.CdnType类型
	Res2ZZZTimestamp          string   `json:"Res2ZZZTimestamp"`          //混淆资源名时间戳
	FilesDecodeKey            string   `json:"FilesDecodeKey"`            //files文件解密密钥
	EndpointOfBucket          string   `json:"EndpointOfBucket"`          //用户Bucket所在数据中心的访问域名
	BucketName                string   `json:"BucketName"`                //Bucket名称
	AccessKeyID               string   `json:"AccessKeyID"`               //访问id
	AccessKeySecret           string   `json:"AccessKeySecret"`           //访问密钥
	AutoBuildClientMethodList []string `json:"AutoBuildClientMethodList"` //客户端构建方法列表
	PackageConfigSvr          string   `json:"PackageConfigSvr"`          //包配置服务器地址
	PackageConfigId           string   `json:"PackageConfigId"`           //包配置id
	BackupPath                string   `json:"BackupPath"`                //热更备份地址
	HotFixedResTempFolderName string   `json:"HotFixedResTempFolderName"` //热更资源临时文件夹
	ResPaths                  []string `json:"ResPaths"`                  //资源地址（相对bucket根目录），默认第一个为测试地址
	Languages                 []int    `json:"Languages"`                 //支持切换的语言数组
	Platform                  string   `json:"Platform"`                  //平台
}

var lastProjectCdnFileName string     //上一次的CDN数据文件名（基本一个项目一个文件）
var cdnConfigMap map[string]*CdnModel //项目CDN配置字典，key CDN名 value:项目CDN配置
var cdnDataLock sync.Mutex

// 有就更新，没有则添加
func UpdateCdn(projectName, cdnConfig string) (result string) {
	cdnDataLock.Lock()
	defer cdnDataLock.Unlock()

	//先获取数据
	var fileName string
	fileName, cdnConfigMap = getProjectCdnsData(projectName)

	//再更新数据
	cdnArr := strings.Split(cdnConfig, ";")
	for _, cdn := range cdnArr {
		if cdn == "" {
			continue
		}
		cdnModel := new(CdnModel)
		tool.UnmarshJson([]byte(cdn), &cdnModel)
		if cdnModel.CdnName == "" {
			errMsg := "cdn配置工程名不能为空：" + cdn
			result += (errMsg + "\n")
			continue
		}

		//删除配置
		if strings.Contains(cdnModel.CdnName, "-") {
			//负号作为删除标记吧
			delBranch := strings.ReplaceAll(cdnModel.CdnName, "-", "")
			if strings.Contains(delBranch, "*") {
				delBranch = strings.ReplaceAll(delBranch, "*", "")
				delKeys := make([]string, 0)
				for k, _ := range cdnConfigMap {
					if strings.Contains(k, delBranch) {
						delKeys = append(delKeys, k)
					}
				}
				for _, k := range delKeys {
					delete(cdnConfigMap, k)
				}
			} else {
				delete(cdnConfigMap, delBranch)
			}
			continue
		}

		//判断svn工程是否存在
		if !JudgeSvnProjectIsExist(projectName, cdnModel.CdnName) {
			result += fmt.Sprintf("不存在%s工程，请先用【%s】指令添加！\n", cdnModel.CdnName, commandName[CommandType_SvnProjectConfig])
			continue
		}

		//增加或修改
		if _cdnModel, ok := cdnConfigMap[cdnModel.CdnName]; ok {
			//已存在，如果数据为空则用老数据
			if cdnModel.CdnType == "" {
				cdnModel.CdnType = _cdnModel.CdnType
			}
			if cdnModel.Res2ZZZTimestamp == "" {
				cdnModel.Res2ZZZTimestamp = _cdnModel.Res2ZZZTimestamp
			}
			if cdnModel.FilesDecodeKey == "" {
				cdnModel.FilesDecodeKey = _cdnModel.FilesDecodeKey
			}
			if cdnModel.EndpointOfBucket == "" {
				cdnModel.EndpointOfBucket = _cdnModel.EndpointOfBucket
			}
			if cdnModel.BucketName == "" {
				cdnModel.BucketName = _cdnModel.BucketName
			}
			if cdnModel.BackupPath == "" {
				cdnModel.BackupPath = _cdnModel.BackupPath
			}
			if "" == cdnModel.PackageConfigSvr {
				//如果没有设置，则用老的
				cdnModel.PackageConfigSvr = _cdnModel.PackageConfigSvr
			} else {
				//负号作为删除标记,才能真正删除
				if strings.Contains(cdnModel.PackageConfigSvr, "-") {
					cdnModel.PackageConfigSvr = ""
				}
			}
			if "" == cdnModel.HotFixedResTempFolderName {
				//如果没有设置，则用老的
				cdnModel.HotFixedResTempFolderName = _cdnModel.HotFixedResTempFolderName
			} else {
				//负号作为删除标记,才能真正删除
				if strings.Contains(cdnModel.HotFixedResTempFolderName, "-") {
					cdnModel.HotFixedResTempFolderName = ""
				}
			}
			if len(cdnModel.AutoBuildClientMethodList) <= 0 {
				cdnModel.AutoBuildClientMethodList = _cdnModel.AutoBuildClientMethodList
			}
			if cdnModel.AccessKeyID == "" {
				cdnModel.AccessKeyID = _cdnModel.AccessKeyID
			}
			if cdnModel.AccessKeySecret == "" {
				cdnModel.AccessKeySecret = _cdnModel.AccessKeySecret
			}
			if cdnModel.PackageConfigId == "" {
				cdnModel.PackageConfigId = _cdnModel.PackageConfigId
			}
			if len(cdnModel.ResPaths) <= 0 {
				cdnModel.ResPaths = _cdnModel.ResPaths
			}
			if len(cdnModel.Languages) <= 0 {
				cdnModel.Languages = _cdnModel.Languages
			}
			if cdnModel.Platform == "" {
				cdnModel.Platform = _cdnModel.Platform
			}
		}
		cdnConfigMap[cdnModel.CdnName] = cdnModel
	}

	//编码并存储
	tool.SaveGobFile(fileName, cdnConfigMap)
	if result != "" {
		return
	}
	return "更新cdn配置成功"
}

// 获取一个项目所有CDN配置信息
func QueryCdnDataOfOneProject(projectName, searchValue string) (result string) {
	cdnDataLock.Lock()
	defer cdnDataLock.Unlock()
	_, cdnConfigMap = getProjectCdnsData(projectName)
	if len(cdnConfigMap) <= 0 {
		return "当前没有cdn配置信息，请配置：\n" + GetCdnConfigHelp()
	}

	tpl := CdnModel{}
	for _, v := range cdnConfigMap {
		if !JudgeIsSearchAllParam(searchValue) && !strings.Contains(v.CdnName, searchValue) {
			continue
		}
		tpl.CdnName = v.CdnName
		tpl.CdnType = v.CdnType
		tpl.Res2ZZZTimestamp = v.Res2ZZZTimestamp
		tpl.FilesDecodeKey = v.FilesDecodeKey
		tpl.EndpointOfBucket = v.EndpointOfBucket
		tpl.BucketName = v.BucketName
		tpl.PackageConfigSvr = v.PackageConfigSvr
		tpl.HotFixedResTempFolderName = v.HotFixedResTempFolderName
		tpl.AutoBuildClientMethodList = v.AutoBuildClientMethodList
		tpl.BackupPath = v.BackupPath
		tpl.ResPaths = v.ResPaths
		tpl.Languages = v.Languages
		tpl.PackageConfigId = v.PackageConfigId
		tpl.AccessKeyID = v.AccessKeyID
		tpl.AccessKeySecret = v.AccessKeySecret
		tpl.Platform = v.Platform
		result += fmt.Sprintln(tool.MarshalJson(tpl) + "\n")
	}
	if result == "" {
		return "当前没有符合条件的cdn配置信息，请配置：\n" + GetCdnConfigHelp()
	} else {
		return "\n***********************以下是cdn配置数据***********************\n" + result
	}
}

// 获取cdn配置帮助提示
func GetCdnConfigHelp() string {
	tpl := CdnModel{
		CdnName:                   "指令【" + commandName[CommandType_SvnProjectConfig] + "】为了节省关联字段，所以用名称一样关联",
		CdnType:                   "0:阿里云，1：华为云,2:亚马逊，3：腾讯云,4:ucloud",
		Res2ZZZTimestamp:          "客户端AppConst.GetZKey方法返回的值",
		FilesDecodeKey:            "客户端AppConst.ShenQiZhiMenR64属性返回的值",
		PackageConfigId:           "热更对应的服务器包配置id",
		PackageConfigSvr:          "包配置服务地址,如果要空字符串，加-标识",
		HotFixedResTempFolderName: "输出热更资源时拷贝热更资源到项目临时文件夹(相对项目根目录)，一般不要配置，用于一些没有接入cdn sdk的项目方便获取热更资源",
		ResPaths:                  []string{"第一个默认为测试地址，地址都为Bucket下得相对路径，且不能有反斜杠用/", "路径2开始为正式地址1，多个地址后面追加"},
		Platform:                  "平台",
	}
	return fmt.Sprintf("cdn配置理解为热更需要的svn工程配置的补充字段吧\n其中Languages为该热更包支持切换的所有语言枚举数组，但是如果所有语言都打进一个包，即不同语言资源不区分则不要配置！！语言枚举跟各自项目走，一般0简体 1繁体 2英文 3韩文 4法文 5德文 6泰文 7越南\n例：\n【%s：%s】 \n如果多个配置用分号分割", commandName[CommandType_CdnConfig], tool.MarshalJson(tpl))
}

// 获取测试热更地址
func GetFullTestHotfixPath(projectName, cdnName string) (err error, testResPath string) {
	if cdnName == "" {
		err = errors.New(cdnName + "获取cdn地址失败，cdn配置名称不能为空！")
		return
	}
	cdnDataLock.Lock()
	defer cdnDataLock.Unlock()
	_, cdnConfigMap = getProjectCdnsData(projectName)
	if _cdnModel, ok := cdnConfigMap[cdnName]; ok {
		if len(_cdnModel.ResPaths) <= 0 {
			err = errors.New(cdnName + "cdn热更地址不存在，请配置！")
			return
		}

		return nil, fmt.Sprintf("http://%s.%s/%s", _cdnModel.BucketName, _cdnModel.EndpointOfBucket, _cdnModel.ResPaths[0])
	} else {
		err = errors.New(cdnName + "cdn配置不存在，请添加！")
		return
	}
}

// 根据参数获取构建方法
func GetBuildMethod(projectName, cdnName, buildMethodParam string) (error, string, int) {
	if cdnName == "" {
		return errors.New("获取构建方法，cdn配置获取失败，名称不能为空！"), "", -1
	}
	cdnDataLock.Lock()
	defer cdnDataLock.Unlock()
	_, cdnConfigMap = getProjectCdnsData(projectName)
	intParam, err2Int := strconv.Atoi(buildMethodParam)
	if err2Int != nil {
		intParam = -1
	}
	if cdnConfig, ok := cdnConfigMap[cdnName]; ok {
		for k, v := range cdnConfig.AutoBuildClientMethodList {
			if k == intParam || v == buildMethodParam {
				return nil, v, intParam
			}
		}
	}

	return errors.New("获取构建方法失败：" + buildMethodParam), "", -1
}

// 获取热更资源临时文件夹路径
func GetHotFixedResTempFolderName(projectName, cdnName string) (error, string) {
	if cdnName == "" {
		return errors.New("获取热更资源临时文件夹路径，cdn配置获取失败，名称不能为空！"), ""
	}
	cdnDataLock.Lock()
	defer cdnDataLock.Unlock()
	_, cdnConfigMap = getProjectCdnsData(projectName)
	if cdnConfig, ok := cdnConfigMap[cdnName]; ok {
		return nil, cdnConfig.HotFixedResTempFolderName
	}
	return errors.New("获取项目热更文件临时文件夹名称失败，cdn配置不存在,请添加！！！"), ""
}

// 获取包配置地址
func GetPackageConfigSvr(projectName, cdnName string) (packageIds []string, packageSvrs []string) {
	if cdnName == "" {
		return
	}
	cdnDataLock.Lock()
	defer cdnDataLock.Unlock()
	_, cdnConfigMap = getProjectCdnsData(projectName)
	if cdnConfig, ok := cdnConfigMap[cdnName]; ok {
		return strings.Split(cdnConfig.PackageConfigId, "|"), strings.Split(cdnConfig.PackageConfigSvr, "|")
	}
	return
}

// 获取CDN配置数据
func GetCdnData(projectName, cdnName string) (
	err error, cdnType, urlOfBucket, bucketName, zzzTimeStamp, fileDecodeKey, accessKeyID,
	accessKeySecret, backupPath, platform string, resPaths []string, languages []int) {
	if cdnName == "" {
		err = errors.New("获取cdn地址失败，名称不能为空！")
		return
	}
	cdnDataLock.Lock()
	defer cdnDataLock.Unlock()
	_, cdnConfigMap = getProjectCdnsData(projectName)
	if _cdnModel, ok := cdnConfigMap[cdnName]; ok {
		return nil, _cdnModel.CdnType, _cdnModel.EndpointOfBucket, _cdnModel.BucketName, _cdnModel.Res2ZZZTimestamp, _cdnModel.FilesDecodeKey,
			_cdnModel.AccessKeyID, _cdnModel.AccessKeySecret, _cdnModel.BackupPath, _cdnModel.Platform, _cdnModel.ResPaths, _cdnModel.Languages
	} else {
		err = errors.New("cdn配置不存在，请添加！")
		return
	}
}

// 获取支持的语言
func GetLanguages(projectName, cdnName string) (err error, lans []int) {
	cdnDataLock.Lock()
	defer cdnDataLock.Unlock()
	if cdnName == "" {
		err = errors.New("cdn名不能为空！")
		return
	}
	_, cdnConfigMap = getProjectCdnsData(projectName)
	if _cdnModel, ok := cdnConfigMap[cdnName]; ok {
		return nil, _cdnModel.Languages
	} else {
		err = errors.New("cdn配置不存在，请添加！")
		return
	}
}

// 获取包配置地址
func GetPlatform(projectName, cdnName string) string {
	if cdnName == "" {
		return ""
	}
	cdnDataLock.Lock()
	defer cdnDataLock.Unlock()
	_, cdnConfigMap = getProjectCdnsData(projectName)
	if cdnConfig, ok := cdnConfigMap[cdnName]; ok {
		return cdnConfig.Platform
	}
	return ""
}

// 根据项目名获取cdn文件名和数据
func getProjectCdnsData(projectName string) (string, map[string]*CdnModel) {
	cdnDataFileName := "cdn.gob"
	fileName := tool.CalcMd5(projectName) + cdnDataFileName
	if fileName == lastProjectCdnFileName {
		return fileName, cdnConfigMap
	}
	cdnConfigMap = make(map[string]*CdnModel)
	tool.ReadGobFile(fileName, &cdnConfigMap)
	lastProjectCdnFileName = fileName
	return fileName, cdnConfigMap
}
