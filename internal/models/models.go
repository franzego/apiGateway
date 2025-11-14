package models

import "time"

type NotificationMessage struct {
	ID           string     `json:"id"`
	Type         string     `json:"type"` // "email" or "push"
	UserID       string     `json:"user_id"`
	TemplateID   string     `json:"template_id"`
	ScheduledFor *time.Time `json:"scheduled_for,omitempty"`
	Timestamp    time.Time  `json:"timestamp"`
}
type ApiResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Error   string `json:"error"`
}
