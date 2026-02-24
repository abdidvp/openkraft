package domain

import "time"

// Notification represents a notification sent to a user.
type Notification struct {
	ID        string
	UserID    string
	Title     string
	Body      string
	Channel   string
	SentAt    time.Time
}
