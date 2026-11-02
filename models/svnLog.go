package models

type SvnLog struct {
	LogType string                         //日志类型（bug、优化、新功能）
	Logs    map[string]map[string][]string //第一个key:系统名称 第二个key:作者 第二个字典数据：日志切片
}
