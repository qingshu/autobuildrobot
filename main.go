// 功能：自动构建机器人（以钉钉为例，支持拓展其他办公通讯工具）
// 设计思路：用beego搭建web服务，在钉钉开发者后台设置web接收地址，@钉钉机器人发送得消息会转发到本web服务，通过接收到得消息执行相应得指令
// app.conf 为服务配置，core/task 为指令解析与任务执行逻辑。
// 时间：2020/09/04
// 作者：lyp
package main

import (
	_ "autobuildrobot/routers"

	"github.com/astaxie/beego"
)

func main() {
	beego.Run()
}
