package domain

import "fmt"

type NotificationDriverType string

const (
	NoNotification               NotificationDriverType = "none"
	TmuxNotification             NotificationDriverType = "tmux"
	TerminalNotifierNotification NotificationDriverType = "terminal-notifier"
)

func NewNotificationDriverType(value string) (NotificationDriverType, error) {
	driver := NotificationDriverType(value)
	if driver == "" {
		return NoNotification, nil
	}
	switch driver {
	case NoNotification, TmuxNotification, TerminalNotifierNotification:
		return driver, nil
	default:
		return driver, fmt.Errorf("invalid notification driver type %q", driver)
	}
}
