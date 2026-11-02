package tool

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// SendDingMessage is independent of user callbacks, so it can report a failed
// callback without invoking that callback a second time. Http bounds its wait.
func SendDingMessage(webhook, message, phoneNumbers string) error {
	if strings.TrimSpace(webhook) == "" {
		return errors.New("钉钉 webhook 为空")
	}
	mobiles := make([]string, 0)
	for _, phone := range strings.Split(phoneNumbers, ",") {
		phone = strings.Trim(strings.TrimSpace(phone), "\"")
		if phone != "" {
			mobiles = append(mobiles, phone)
		}
	}
	payload := struct {
		MsgType string `json:"msgtype"`
		Text    struct {
			Content string `json:"content"`
		} `json:"text"`
		At struct {
			Mobiles []string `json:"atMobiles"`
			All     bool     `json:"isAtAll"`
		} `json:"at"`
	}{MsgType: "text"}
	payload.Text.Content = strings.ReplaceAll(message, "\r\n", "\n")
	payload.At.Mobiles = mobiles
	content, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	err, result := Http("POST", webhook, string(content))
	if err != nil {
		return err
	}
	var response struct {
		Code    int    `json:"errcode"`
		Message string `json:"errmsg"`
	}
	if err := json.Unmarshal(result, &response); err != nil {
		return fmt.Errorf("解析钉钉响应: %w", err)
	}
	if response.Code != 0 {
		return fmt.Errorf("钉钉拒绝消息: %d %s", response.Code, response.Message)
	}
	return nil
}
