package main

import (
	"fmt"
	"github.com/containrrr/shoutrrr"
)

func notificationMessage(message string, args ...interface{}) {
	message = fmt.Sprintf(message, args...)
	message = fmt.Sprintf(opts.NotificationMsgTemplate, message)

	for _, url := range opts.Notification {
		if err := shoutrrr.Send(url, message); err != nil {
			Logger.Error("Unable to send shoutrrr notification", err)
		}
	}
}
