package routers

import (
	"autobuildrobot/controllers"
	"github.com/astaxie/beego"
)

func init() {
	beego.Router("/", &controllers.DingDingController{})
	beego.Router("/weChat", &controllers.WeChatAutoBuld{})
	beego.Router("/webCommand", &controllers.WebCommandController{})
}
