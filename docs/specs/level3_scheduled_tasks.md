# SPEC: Scheduled Tasks (Cron-like Scheduler)

## Goal
Enable recurring task execution: periodic knowledge base refresh, scheduled summaries, reminder checks, and custom user-defined cron jobs.

## Current State
- `TaskStore` supports CRUD for user tasks (one-time)
- Obsidian sync runs on interval (`ObsidianDefaultIntervalMinutes`)
- No general-purpose scheduler

## Architecture

### New Package: `internal/infrastructure/scheduler/`

```
internal/infrastructure/scheduler/
  scheduler.go        — cron-like scheduler engine
  job.go              — job definition and registry
  builtin.go          — built-in jobs (obsidian sync, memory cleanup, etc.)
  scheduler_test.go
```

### New Domain Types

```go
// domain/scheduler.go

type ScheduledJob struct {
    ID          string        `json:"id"`
    Name        string        `json:"name"`
    Schedule    string        `json:"schedule"`     // cron expression: "0 */6 * * *"
    Type        JobType       `json:"type"`         // "builtin", "agent_prompt", "webhook"
    Payload     string        `json:"payload"`      // agent prompt or webhook URL
    Enabled     bool          `json:"enabled"`
    LastRun     *time.Time    `json:"last_run,omitempty"`
    NextRun     *time.Time    `json:"next_run,omitempty"`
    UserID      string        `json:"user_id"`
}

type JobType string
const (
    JobTypeBuiltin     JobType = "builtin"
    JobTypeAgentPrompt JobType = "agent_prompt"
    JobTypeWebhook     JobType = "webhook"
)
```

### Scheduler Engine

```go
type Scheduler struct {
    jobs      map[string]*ScheduledJob
    store     SchedulerStore
    executor  JobExecutor
    ticker    *time.Ticker
    mu        sync.RWMutex
    logger    *slog.Logger
}

func (s *Scheduler) Start(ctx context.Context)
func (s *Scheduler) Stop()
func (s *Scheduler) AddJob(job ScheduledJob) error
func (s *Scheduler) RemoveJob(id string) error
func (s *Scheduler) ListJobs() []ScheduledJob
```

Uses `robfig/cron/v3` for cron expression parsing (already available in Go ecosystem, lightweight).

### Built-in Jobs
1. `obsidian_sync` — periodic vault synchronization (replaces current interval logic)
2. `memory_cleanup` — prune old memory summaries
3. `task_reminders` — check due tasks and generate notifications
4. `knowledge_refresh` — re-embed documents with updated models

### Agent Prompt Jobs
Users can create scheduled prompts that run through the agent:
```json
{
  "name": "daily_summary",
  "schedule": "0 20 * * *",
  "type": "agent_prompt",
  "payload": "Summarize today's notes and tasks"
}
```

### Persistence

```sql
CREATE TABLE scheduled_jobs (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL,
    name TEXT NOT NULL,
    schedule TEXT NOT NULL,
    type TEXT NOT NULL,
    payload TEXT DEFAULT '',
    enabled BOOLEAN DEFAULT true,
    last_run TIMESTAMPTZ,
    next_run TIMESTAMPTZ,
    created_at TIMESTAMPTZ DEFAULT now()
);
```

### API Endpoints
- `POST /v1/scheduler/jobs` — create job
- `GET /v1/scheduler/jobs` — list jobs
- `DELETE /v1/scheduler/jobs/{id}` — remove job
- `PATCH /v1/scheduler/jobs/{id}` — update (enable/disable, change schedule)

### Config
```
SCHEDULER_ENABLED=false
SCHEDULER_CHECK_INTERVAL_SECONDS=60
SCHEDULER_MAX_CONCURRENT_JOBS=3
```

## Tests
- Unit: cron parsing, next-run calculation
- Unit: job executor with mock agent
- Integration: schedule a job, verify it fires
