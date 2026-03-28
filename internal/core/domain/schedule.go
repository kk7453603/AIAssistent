package domain

import "time"

type ScheduledTask struct {
	ID         string     `json:"id"`
	UserID     string     `json:"user_id"`
	CronExpr   string     `json:"cron_expr"`
	Prompt     string     `json:"prompt"`
	Condition  string     `json:"condition,omitempty"`
	WebhookURL string     `json:"webhook_url,omitempty"`
	Enabled    bool       `json:"enabled"`
	LastRunAt  *time.Time `json:"last_run_at,omitempty"`
	LastResult string     `json:"last_result,omitempty"`
	LastStatus string     `json:"last_status,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
}
