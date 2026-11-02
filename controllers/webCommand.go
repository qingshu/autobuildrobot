package controllers

import (
	"autobuildrobot/core/task"

	"github.com/astaxie/beego"
)

// 钉钉自动构建
type WebCommandController struct {
	beego.Controller
}

func (this *WebCommandController) Post() {
	this.webCommand()
}

func (this *WebCommandController) Get() {
	this.webCommand()
}

func (this *WebCommandController) webCommand() {
	//获取并解析指令
	projectName := this.Input().Get("p")
	nick := this.Input().Get("u")
	content := this.Input().Get("c")
	this.Ctx.ResponseWriter.Header().Set("Content-Type", "text/event-stream;charset=utf-8")
	this.Ctx.ResponseWriter.Header().Set("Cache-Control", "no-cache")
	this.Ctx.ResponseWriter.Header().Set("Connection", "keep-alive")

	<-task.ExecCommand(projectName, nick, content, "", func(msg, phoneNum string) {
		this.Ctx.WriteString("\n" + msg + "\n")
		this.Ctx.ResponseWriter.Flush()
	})
}
