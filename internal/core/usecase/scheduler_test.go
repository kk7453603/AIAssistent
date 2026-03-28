package usecase

import (
	"testing"
	"time"

	"github.com/robfig/cron/v3"

	"github.com/kirillkom/personal-ai-assistant/internal/core/domain"
)

func newTestScheduler() *SchedulerUseCase {
	return &SchedulerUseCase{
		parser: cron.NewParser(
			cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor,
		),
	}
}

// TestCronShouldRun verifies that a every-minute cron with a 2-minute-old lastRun is due.
func TestCronShouldRun(t *testing.T) {
	s := newTestScheduler()

	lastRun := time.Now().Add(-2 * time.Minute)
	task := domain.ScheduledTask{
		ID:        "test-1",
		CronExpr:  "* * * * *", // every minute
		LastRunAt: &lastRun,
	}

	now := time.Now()
	if !s.isDue(task, now) {
		t.Error("expected task to be due for every-minute cron with 2min-old lastRun, but isDue returned false")
	}
}

// TestCronShouldNotRun verifies that a yearly cron is not due when lastRun is recent.
func TestCronShouldNotRun(t *testing.T) {
	s := newTestScheduler()

	lastRun := time.Now().Add(-1 * time.Minute)
	task := domain.ScheduledTask{
		ID:        "test-2",
		CronExpr:  "0 0 1 1 *", // once a year: midnight on January 1st
		LastRunAt: &lastRun,
	}

	now := time.Now()
	if s.isDue(task, now) {
		t.Error("expected yearly cron to NOT be due one minute after lastRun, but isDue returned true")
	}
}

// TestTruncateScheduleResult verifies that the helper truncates long strings correctly.
func TestTruncateScheduleResult(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		maxLen int
		want   string
	}{
		{
			name:   "short string unchanged",
			input:  "hello",
			maxLen: 10,
			want:   "hello",
		},
		{
			name:   "exact length unchanged",
			input:  "hello",
			maxLen: 5,
			want:   "hello",
		},
		{
			name:   "long string truncated with ellipsis",
			input:  "hello world",
			maxLen: 5,
			want:   "hello...",
		},
		{
			name:   "empty string unchanged",
			input:  "",
			maxLen: 10,
			want:   "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := truncateScheduleResult(tc.input, tc.maxLen)
			if got != tc.want {
				t.Errorf("truncateScheduleResult(%q, %d) = %q; want %q", tc.input, tc.maxLen, got, tc.want)
			}
		})
	}
}
