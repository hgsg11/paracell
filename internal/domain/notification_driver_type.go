package domain

import "fmt"

type NotificationDriverType string

const (
	NoNotification   NotificationDriverType = "none"
	TmuxNotification NotificationDriverType = "tmux"
)

func NewNotificationDriverType(value string) (NotificationDriverType, error) {
	driver := NotificationDriverType(value)
	if driver == "" {
		return NoNotification, nil
	}
	switch driver {
	case NoNotification, TmuxNotification:
		return driver, nil
	default:
		return driver, fmt.Errorf("invalid notification driver type %q", driver)
	}
}
