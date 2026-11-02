package models

//钉钉数据
type DingDingData struct {
	Msg            map[string]string `json:"text"`
	CreateAt       int               `json:"createAt"`
	SenderId       string            `json:"senderId"`
	SenderNick     string            `json:"senderNick"`
	SenderCorpId   string            `json:"senderCorpId"`
	SenderStaffId  string            `json:"senderStaffId"`
	ChatbotUserId  string            `json:"chatbotUserId"`
	SessionWebhook string            `json:"sessionWebhook"`
	ProjectName    string            `json:"conversationTitle"` //群组名（一个项目一个群，所以这里也作为项目名）
}
