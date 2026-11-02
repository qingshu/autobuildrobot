package models

import (
	"autobuildrobot/tool"
	"fmt"
	"github.com/astaxie/beego"
	"strings"
	"sync"
)

var projectUsersMap map[string]*UserModel       //项目用户数据，key：用户昵称 value:用户数据
var companyUsersMap map[string]*UserModel       //公司用户（默认）
var lastProjectUserFileName = ""                //上一次的项目用户数据文件名（基本一个项目一个文件）
var companyUserFileName = "companyUserData.gob" //公司用户数据文件名
var superUser string
var userDataLock sync.Mutex

// 用户数据
type UserModel struct {
	UserName      string `json:"UserName"`      //用户名
	GroupName     string `json:"GroupName"`     //用户组
	PhoneNum      string `json:"PhoneNum"`      //电话
	IsProjectUser bool   `json:"IsProjectUser"` //是否是项目用户，如果否则是公司用户
}

func init() {
	temp, _ := beego.GetConfig("String", "superUser", "")
	superUser = temp.(string)

	companyUsersMap = make(map[string]*UserModel)
	tool.ReadGobFile(companyUserFileName, &companyUsersMap)
}

// 获取项目管理员电话
func GetProjectManagerPhone(projectName string) string {
	userDataLock.Lock()
	defer userDataLock.Unlock()

	manager := GetProjectManager(projectName)
	if manager == "" {
		return ""
	}
	phone := ""
	managers := strings.Split(manager, "|")

	//先从公司用户中获取
	notExistManagers := make([]string, 0)
	for _, _manager := range managers {
		if user, ok := companyUsersMap[_manager]; ok {
			phone += user.PhoneNum + ","
		} else {
			notExistManagers = append(notExistManagers, _manager)
		}
	}

	//再从项目中获取
	if len(notExistManagers) <= 0 {
		return phone
	}
	_, users := getProjectUsersData(projectName)
	for _, _manager := range notExistManagers {
		if user, ok := users[_manager]; ok {
			phone += user.PhoneNum + ","
		}
	}
	return phone
}

// 判断是否有权限
func JudgeIsHadPermission(commandType int, projectName, useName, commandParams string) (bool, string) {
	if strings.TrimSpace(useName) == "" {
		return false, "执行用户不能为空"
	}
	//管理员有所有权限
	isSuperUser := false
	for _, name := range strings.FieldsFunc(superUser, func(r rune) bool { return r == '|' || r == ',' || r == ';' || r == '，' || r == '；' }) {
		if strings.TrimSpace(name) == useName {
			isSuperUser = true
			break
		}
	}
	if isSuperUser || (!JudgeIsOnlySuperUserCmd(commandType) && JudgeIsManager(projectName, useName)) {
		return true, ""
	}

	//解析参数，获取第一个svn工程配置名
	err, params := AnalysisParam(commandParams, commandType)
	if nil != err {
		return false, err.Error()
	}
	svnProject := ""
	if commandType == CommandType_SvnMerge {
		//合并主要看有没有目标分支权限,看第二个参数
		svnProject = params[1]
	} else if len(params) > 0 {
		svnProject = params[0]
	}

	//判断指令是否被禁止了
	commandName := GetCommandNameByType(commandType)
	if JudgeCommandIsBan(projectName, useName, commandName, svnProject) {
		return false, "指令或svn工程已被管理员禁止，联系管理员！"
	}

	//获取权限
	userDataLock.Lock()
	defer userDataLock.Unlock()

	//判断是否有权限
	judgeUserPermission := func(permission []string, projectPermissions []string) (isHadPermission bool, err string) {
		//判断是否有操作权限
		isHadCommandPermission := false
		for _, v := range permission {
			if v == commandName {
				isHadCommandPermission = true
				break
			}
		}
		if !isHadCommandPermission {
			return false, "没有指令权限，请联系管理员！"
		}

		//不用判断是否有项目权限
		if !JudgeIsNeedProjectPermission(commandType) {
			return true, ""
		}

		//判断是否有项目权限
		if nil == projectPermissions || len(projectPermissions) <= 0 {
			return false, "没有项目权限，请联系管理员！"
		}
		for _, v := range projectPermissions {
			if v == svnProject {
				return true, ""
			}
		}
		return false, "没有项目权限，请联系管理员！"
	}

	//先判断是否有项目用户，有则判断权限
	_, users := getProjectUsersData(projectName)
	if user, ok := users[useName]; ok {
		err, permission, projectPermissions := GetUserGroupPermissions(user.GroupName)
		if nil != err {
			return false, err.Error()
		}
		return judgeUserPermission(permission, projectPermissions)
	}

	//再判断公司用户
	if user, ok := companyUsersMap[useName]; ok {
		err, permission, projectPermissions := GetUserGroupPermissions(user.GroupName)
		if nil != err {
			return false, err.Error()
		}
		return judgeUserPermission(permission, projectPermissions)
	}

	return false, "不存在用户，请联系管理员！"
}

// 获取用户电话
func GetUserPhone(projectName, _senderNick string) (phoneNum string) {
	userDataLock.Lock()
	defer userDataLock.Unlock()

	//先判断公司用户
	if user, ok := companyUsersMap[_senderNick]; ok {
		return user.PhoneNum
	}

	//再判断项目用户
	_, users := getProjectUsersData(projectName)
	if user, ok := users[_senderNick]; ok {
		return user.PhoneNum
	}
	return
}

// 获取用户信息
func QueryUsersDatas(projectName, searchParams string) string {
	if JudgeIsSearchAllParam(searchParams) || searchParams == "公司" || searchParams == "项目" {
		return GetAllUserInfo(projectName, searchParams == "公司" || searchParams != "项目")
	} else {
		return GetUserInfoByName(projectName, searchParams)
	}
}

// 获取所有用户信息
func GetAllUserInfo(projectName string, isCompany bool) string {
	userDataLock.Lock()
	defer userDataLock.Unlock()
	users := make(map[string]*UserModel)
	if isCompany {
		users = companyUsersMap
	} else {
		_, users = getProjectUsersData(projectName)
	}
	if len(users) <= 0 {
		return "当前没有任何用户，请添加," + GetUserConfigHelp()
	}
	result := "\n***********************以下是所有的用户配置***********************\n"
	for k, v := range users {
		result += fmt.Sprintf("%s,所在分组：%s\n", k, v.GroupName)
	}
	return result
}

// 根据用户名获取用户信息
func GetUserInfoByName(projectName, userName string) (userInfo string) {
	userDataLock.Lock()
	defer userDataLock.Unlock()

	//获取公司用户
	for k, v := range companyUsersMap {
		if k == userName {
			userInfo += "\n***********************公司用户：***********************\n"
			err, permissions, projectPermission := GetAllPermissionDesc(v.GroupName)
			if nil != err {
				userInfo += fmt.Sprintf("%s,获取权限异常：%s \n", k, err.Error())
				continue
			}
			userInfo += fmt.Sprintf("%s,所在分组：%s,拥有指令权限：%s\n拥有项目权限：%s\n", k, v.GroupName, permissions, projectPermission)
		}
	}

	//再看项目中是否有该用户
	_, projectUsers := getProjectUsersData(projectName)
	for k, v := range projectUsers {
		if k == userName {
			userInfo += "\n***********************项目用户：***********************\n"
			err, permissions, projectPermission := GetAllPermissionDesc(v.GroupName)
			if nil != err {
				userInfo += fmt.Sprintf("%s,获取权限异常：%s \n", k, err.Error())
				continue
			}
			userInfo += fmt.Sprintf("%s,所在分组：%s,拥有指令权限：%s\n拥有项目权限：%s\n", k, v.GroupName, permissions, projectPermission)
		}
	}
	if userInfo == "" {
		userInfo = "查询地用户不存在!!"
	}
	return userInfo
}

// 获取更新用户帮助
func GetUserConfigHelp() string {
	userModel := new(UserModel)
	userModel.UserName = "用户名"
	userModel.GroupName = "所在用户组"
	userModel.PhoneNum = "电话号码"
	userModel.IsProjectUser = false
	return fmt.Sprintf("配置用户信息（如名称 所属用户组等）\n例：\n【%s：%s】\n其中：IsProjectUser如果true则表示该用户属于项目，否则默认的false属于公司用户\n多个配置用分号分割",
		commandName[CommandType_User], tool.MarshalJson(userModel))
}

// 更新用户数据
func UpdateUserInfo(projectName, userInfo string) (result string) {
	userDataLock.Lock()
	defer userDataLock.Unlock()
	var fileName string
	fileName, projectUsersMap = getProjectUsersData(projectName)

	//更新一个玩家数据
	updateOneUser := func(newUserModel *UserModel, userMap map[string]*UserModel) {
		//删除配置
		if strings.Contains(newUserModel.UserName, "-") {
			//负号作为删除标记吧
			delUser := strings.ReplaceAll(newUserModel.UserName, "-", "")
			delete(userMap, delUser)
			return
		}

		//增加或修改
		if _userModel, ok := userMap[newUserModel.UserName]; ok {
			//已存在，如果数据为空则用老数据
			if newUserModel.GroupName == "" {
				newUserModel.GroupName = _userModel.GroupName
			}
			if newUserModel.PhoneNum == "" {
				newUserModel.PhoneNum = _userModel.PhoneNum
			}
		}
		userMap[newUserModel.UserName] = newUserModel
	}

	//再更新数据
	isUpdateProjectUser := false
	isUpdateCompanyUser := false
	userArr := strings.Split(userInfo, ";")
	for _, user := range userArr {
		if user == "" {
			continue
		}
		userModel := new(UserModel)
		tool.UnmarshJson([]byte(user), &userModel)
		if userModel.UserName == "" {
			errMsg := "配置用户，用户名不能为空：" + user
			result += (errMsg + "\n")
			continue
		}

		if userModel.GroupName != "" && !JudgeUserGroupIsExist(userModel.GroupName) {
			errMsg := "配置用户，用户组不存在，请检查：" + user
			result += (errMsg + "\n")
			continue
		}

		if userModel.IsProjectUser {
			updateOneUser(userModel, projectUsersMap)
			isUpdateProjectUser = true
		} else {
			updateOneUser(userModel, companyUsersMap)
			isUpdateCompanyUser = true
		}
	}

	//编码并存储
	if isUpdateCompanyUser {
		tool.SaveGobFile(companyUserFileName, companyUsersMap)
	}
	if isUpdateProjectUser {
		tool.SaveGobFile(fileName, projectUsersMap)
	}
	if result != "" {
		return result
	}
	return "更新用户数据成功"
}

// 根据项目名获取项目用户数据
func getProjectUsersData(projectName string) (string, map[string]*UserModel) {
	userDataFileName := "user.gob"
	fileName := tool.CalcMd5(projectName) + userDataFileName
	if fileName == lastProjectUserFileName {
		return fileName, projectUsersMap
	}
	projectUsersMap = make(map[string]*UserModel)
	tool.ReadGobFile(fileName, &projectUsersMap)
	lastProjectUserFileName = fileName
	return fileName, projectUsersMap
}
