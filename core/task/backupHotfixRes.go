package task

import (
	"autobuildrobot/core/cdn"

	"autobuildrobot/models"
	"errors"
	"strconv"
	"strings"
)

// BackupHotfixRes uses the same backup path and comparison process as publishing.
func BackupHotfixRes(command *operationCommand) (string, error) {
	if command == nil {
		return "", errors.New("备份参数不能为空")
	}
	err, params := models.AnalysisParam(command.CommandParams, models.CommandType_BackupHotfixRes)
	if err != nil {
		return "", err
	}
	if len(params) == 0 || strings.TrimSpace(params[0]) == "" {
		return "", errors.New("必须指定工程名称")
	}
	err, kind, endpoint, bucket, stamp, key, id, secret, backup, platform, paths, languages := models.GetCdnData(command.ProjectName, params[0])
	if err != nil {
		return "", err
	}
	if backup == "" || len(paths) < 2 || strings.TrimSpace(paths[1]) == "" {
		return "", errors.New("请配置备份目录及正式资源地址")
	}
	err, client := cdn.GetCdnClient(kind, endpoint, bucket, id, secret)
	if err != nil {
		return "", err
	}
	lans := []string{""}
	if len(languages) > 0 {
		lans = nil
		for _, language := range languages {
			lans = append(lans, strconv.Itoa(language))
		}
	}
	var result strings.Builder
	for _, language := range lans {
		message, err := BackRes(stamp, key, backup, paths[1], platform, language, client)
		if err != nil {
			return result.String(), err
		}
		result.WriteString(message + "\n")
	}
	return result.String(), nil
}
