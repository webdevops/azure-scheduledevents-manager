package main

import (
	"fmt"
	"github.com/containrrr/shoutrrr"
	log "github.com/sirupsen/logrus"
)

func notificationMessage(message string, args ...interface{}) {
	message = fmt.Sprintf(message, args...)
	message = fmt.Sprintf(opts.NotificationMsgTemplate, message)

	for _, url := range opts.Notification {
		if err := shoutrrr.Send(url, message); err != nil {
			log.Errorf("unable to send shoutrrr notification: %v", err)
		}
	}
}
