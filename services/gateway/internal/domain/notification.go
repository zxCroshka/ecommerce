package domain

import "time"

type Notification struct {
	ID        int64
	Type      string
	Title     string
	Body      string
	CreatedAt time.Time
	ReadAt    *time.Time
}

type NotificationList struct {
	Notifications []*Notification
	Total         int64
	Limit         int32
	Offset        int32
}
