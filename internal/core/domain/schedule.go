package domain

import "time"

type ScheduledTask struct {
	ID         string
	UserID     string
	CronExpr   string
	Prompt     string
	Condition  string
	WebhookURL string
	Enabled    bool
	LastRunAt  *time.Time
	LastResult string
	LastStatus string
	CreatedAt  time.Time
	UpdatedAt  time.Time
}
