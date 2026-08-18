package service

import (
	"bytes"
	"fmt"
	"net/http"

	"github.com/QuantumNous/new-api/common"
)

const pushPlusAPIURL = "https://www.pushplus.plus/send/"

type pushPlusRequest struct {
	Token   string `json:"token"`
	Title   string `json:"title"`
	Content string `json:"content"`
	Topic   string `json:"topic,omitempty"` // 群组编码，填写后为群组推送
}

// SendPushPlusNotify sends a PushPlus push message.
// When topic is non-empty it uses group (one-to-many) mode; otherwise one-to-one.
func SendPushPlusNotify(token, topic, title, content string) error {
	payload := pushPlusRequest{
		Token:   token,
		Title:   title,
		Content: content,
		Topic:   topic,
	}

	payloadBytes, err := common.Marshal(payload)
	if err != nil {
		return fmt.Errorf("pushplus: failed to marshal payload: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, pushPlusAPIURL, bytes.NewBuffer(payloadBytes))
	if err != nil {
		return fmt.Errorf("pushplus: failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	req.Header.Set("User-Agent", "NewAPI-PushPlus-Notify/1.0")

	client := GetSSRFProtectedHTTPClient()
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("pushplus: request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("pushplus: unexpected status code: %d", resp.StatusCode)
	}

	return nil
}
