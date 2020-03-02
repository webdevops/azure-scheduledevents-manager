package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
)

type (
	NotificationMessage struct {
		Channel  string                            `json:"channel"`
		Username string                            `json:"username"`
		Text     string                            `json:"text"`
		Blocks   []NotificationMessageBlockContext `json:"blocks"`
	}

	NotificationMessageBlockContext struct {
		Type     string                          `json:"type"`
		Text     *NotificationMessageBlockText   `json:"text,omitempty"`
		Elements []*NotificationMessageBlockText `json:"elements,omitempty"`
	}

	NotificationMessageBlockText struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
)

func notificationMessage(message string, args ...interface{}) {
	if opts.NotificationSlackUrl == "" {
		return
	}

	message = fmt.Sprintf(message, args...)
	message = fmt.Sprintf(opts.NotificationMsgTemplate, message)

	payloadBlocks := []NotificationMessageBlockContext{}
	payloadBlocks = append(payloadBlocks, NotificationMessageBlockContext{
		Type: "section",
		Text: &NotificationMessageBlockText{
			Type: "plain_text",
			Text: message,
		},
	})

	payload := NotificationMessage{
		Username: "Azure ScheduledEvents manager",
		Text:     message,
		Blocks:   payloadBlocks,
	}
	payloadJson, _ := json.Marshal(payload)

	client := http.Client{}
	req, err := http.NewRequest("POST", opts.NotificationSlackUrl, bytes.NewBuffer(payloadJson))
	defer req.Body.Close()
	if err != nil {
		ErrorLogger.Error("Failed to send slack notification: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	_, err = client.Do(req)
	if err != nil {
		ErrorLogger.Error("Failed to send slack notification: %v", err)
	}
}


