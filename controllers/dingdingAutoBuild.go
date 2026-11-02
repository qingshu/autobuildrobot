package controllers

import (
	"autobuildrobot/core/task"

	"autobuildrobot/models"
	log "autobuildrobot/mylog"
	"autobuildrobot/tool"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"strconv"

	"time"

	"github.com/astaxie/beego"
)

//参考文档：https://developers.dingtalk.com/document/robots

var dingdingRobotAppSecret = ""             //钉钉机器人密钥
const millisecondOfOneHour = int64(3600000) //一小时毫秒数，用于辅助验证非法调用
func init() {
	temp, _ := beego.GetConfig("String", "dingdingRobotAppSecret", "")
	dingdingRobotAppSecret = temp.(string)
}

// 钉钉自动构建
type DingDingController struct {
	beego.Controller
}

func (this *DingDingController) Post() {
	defer func() {
		this.ServeJSON()
	}()

	//解析钉钉传过来得数据
	var dingDingData models.DingDingData
	err := json.Unmarshal(this.Ctx.Input.RequestBody, &dingDingData)
	if err != nil {
		result := "解析钉钉数据异常:" + err.Error()
		log.Error(result)
		return
	}

	//判断时间戳是不是相差一小时，是则非法
	phoneNum := models.GetUserPhone(dingDingData.ProjectName, dingDingData.SenderNick)
	timeStamp := this.Ctx.Request.Header.Get("timestamp")
	nTimeStamp, _ := strconv.ParseInt(timeStamp, 10, 64)
	nCurrentTime := time.Now().UnixNano() / 1e6
	if (nCurrentTime - nTimeStamp) > millisecondOfOneHour {
		result := "收到钉钉信息，时间不合法"
		log.Error(result)
		phoneNum += "," + models.GetProjectManagerPhone(dingDingData.ProjectName)
		sendDingMsg(dingDingData.SessionWebhook, result, phoneNum)
		return
	}

	//验证签名
	//header中的timestamp + "\n" + 机器人的appSecret 当做签名字符串，使用HmacSHA256算法计算签名，然后进行Base64 encode，得到最终的签名值
	sign := this.Ctx.Request.Header.Get("sign")
	originalStr := timeStamp + "\n" + dingdingRobotAppSecret
	key := []byte(dingdingRobotAppSecret)
	h := hmac.New(sha256.New, key)
	h.Write([]byte(originalStr))
	calcSign := base64.StdEncoding.EncodeToString(h.Sum(nil))
	if sign != calcSign {
		result := "收到钉钉信息，签名验证失败！"
		log.Error(result)
		phoneNum += "," + models.GetProjectManagerPhone(dingDingData.ProjectName)
		sendDingMsg(dingDingData.SessionWebhook, result, phoneNum)
		return
	}

	//获取并解析指令
	content, ok := dingDingData.Msg["content"]
	if !ok {
		result := "获取钉钉消息失败！"
		log.Error(result)
		phoneNum += "," + models.GetProjectManagerPhone(dingDingData.ProjectName)
		sendDingMsg(dingDingData.SessionWebhook, result, phoneNum)
		return
	}
	task.ExecCommand(dingDingData.ProjectName, dingDingData.SenderNick, content, dingDingData.SessionWebhook, func(msg, phoneNum string) {
		sendDingMsg(dingDingData.SessionWebhook, msg, phoneNum)
	})
}

func (c *DingDingController) Get() {
	c.ServeJSON()
}

// 发送结果到钉钉群
func sendDingMsg(webHook, msg, phoneNum string) {
	if err := tool.SendDingMessage(webHook, msg, phoneNum); err != nil {
		log.Error("回调钉钉异常", err)
	}
}
