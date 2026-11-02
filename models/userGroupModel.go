package models

import (
	"autobuildrobot/tool"
	"errors"
	"fmt"
	"strings"
	"sync"
)

// 用户组
type UserGroupModel struct {
	GroupName          string   `json:"GroupName"`          //组名
	CommandPermissions []string `json:"CommandPermissions"` //指令权限数组
	ProjectPermissions []string `json:"ProjectPermissions"` //项目权限数组
}

var userGroupFileName = "userGroupData.gob"
var userGroupMap map[string]*UserGroupModel
var userGroupDataLock sync.Mutex

func init() {
	userGroupMap = make(map[string]*UserGroupModel)
	tool.ReadGobFile(userGroupFileName, &userGroupMap)
}

// 有就更新，没有则添加
func UpdateUserGroup(groupConfig string) (result string, err error) {
	userGroupDataLock.Lock()
	defer userGroupDataLock.Unlock()

	//解析数据
	userGroupArr := strings.Split(groupConfig, ";")
	for _, _groupConfig := range userGroupArr {
		if _groupConfig == "" {
			continue
		}
		newGroupModel := new(UserGroupModel)
		newGroupModel.CommandPermissions = make([]string, 0)
		newGroupModel.ProjectPermissions = make([]string, 0)
		err = tool.UnmarshJson([]byte(_groupConfig), &newGroupModel)
		if nil != err {
			errMsg := "配置用户组异常，请检查：" + err.Error()
			result += errMsg + "\n"
			continue
		}

		//编码并存储
		if strings.Contains(newGroupModel.GroupName, "-") {
			//表示删除
			groupName := strings.Replace(newGroupModel.GroupName, "-", "", 1)
			delete(userGroupMap, groupName)
		} else {
			//增加或修改
			if _groupModel, ok := userGroupMap[newGroupModel.GroupName]; ok {
				//已存在，如果数据为空则用老数据
				if newGroupModel.GroupName == "" {
					newGroupModel.GroupName = _groupModel.GroupName
				}
				if len(newGroupModel.ProjectPermissions) <= 0 {
					newGroupModel.ProjectPermissions = _groupModel.ProjectPermissions
				}
				if len(newGroupModel.CommandPermissions) <= 0 {
					newGroupModel.CommandPermissions = _groupModel.CommandPermissions
				}
			}

			//新增或更新
			userGroupMap[newGroupModel.GroupName] = newGroupModel
		}
	}

	tool.SaveGobFile(userGroupFileName, userGroupMap)
	if result != "" {
		return
	}
	result = "更新用户组成功"
	return
}

// 查找数据
func QueryUserGroupDatas(searchValue string) (result string) {
	userGroupDataLock.Lock()
	defer userGroupDataLock.Unlock()

	if len(userGroupMap) <= 0 {
		return "当前没有任何用户组，请添加," + GetUserGroupConfigHelp()
	}

	tpl := UserGroupModel{}
	for _, v := range userGroupMap {
		if !JudgeIsSearchAllParam(searchValue) && !strings.Contains(v.GroupName, searchValue) {
			continue
		}
		tpl.GroupName = v.GroupName
		tpl.CommandPermissions = v.CommandPermissions
		tpl.ProjectPermissions = v.ProjectPermissions
		result += fmt.Sprintln(tool.MarshalJson(tpl) + "\n")
	}
	if result == "" {
		return "当前没有符合条件的用户组配置信息，请配置：\n" + GetUserGroupConfigHelp()
	} else {
		return "\n***********************以下是用户组配置数据***********************\n" + result
	}
}

// 获取用户组帮助信息
func GetUserGroupConfigHelp() (result string) {
	group := new(UserGroupModel)
	group.GroupName = "用户组名"
	group.CommandPermissions = make([]string, 0)
	group.ProjectPermissions = make([]string, 0)
	group.ProjectPermissions = append(group.ProjectPermissions, "拥有权限的项目名1")
	group.ProjectPermissions = append(group.ProjectPermissions, "拥有权限的项目名2")
	return fmt.Sprintf("配置用户组权限\n例：\n【%s：%s】\nCommandPermissions为拥有权限的指令索引数组\n如多个配置用分号分割",
		commandName[CommandType_UserGroup], tool.MarshalJson(group))
}

// 获取指令权限
func GetUserGroupPermissions(groupName string) (error, []string, []string) {
	userGroupDataLock.Lock()
	defer userGroupDataLock.Unlock()
	if project, ok := userGroupMap[groupName]; ok {
		return nil, project.CommandPermissions, project.ProjectPermissions
	}
	return errors.New("用户组不存在,请配置！！！"), make([]string, 0), make([]string, 0)
}

// 获取所有权限描述
func GetAllPermissionDesc(groupName string) (err error, commandPermission, projectPermission string) {
	userGroupDataLock.Lock()
	defer userGroupDataLock.Unlock()
	if project, ok := userGroupMap[groupName]; ok {
		for _, v := range project.CommandPermissions {
			commandPermission += v + "|"
		}
		for _, v := range project.ProjectPermissions {
			projectPermission += v + "|"
		}
		return
	}
	return errors.New("用户组不存在,请配置！！！"), "", ""
}

// 判断用户组是否存在
func JudgeUserGroupIsExist(groupName string) bool {
	userGroupDataLock.Lock()
	defer userGroupDataLock.Unlock()
	if _, ok := userGroupMap[groupName]; ok {
		return true
	}
	return false
}
