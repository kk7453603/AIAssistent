# Schedules Widget on Dashboard — Design Spec

## Goal

Add a Schedules management widget to the Dashboard page with full CRUD: table view, create/edit forms, delete confirmation, and instant enable/disable toggle.

## Architecture

Two new files (API client + widget component), two modified files (types + DashboardPage). Backend CRUD API already exists.

### Files

**New:**
```
ui/src/api/schedules.ts                          — API client (list, create, update, delete)
ui/src/components/dashboard/SchedulesWidget.tsx   — table + inline form + actions
```

**Modified:**
```
ui/src/api/types.ts            — add ScheduledTask type
ui/src/pages/DashboardPage.tsx — add Schedules section
```

## Types

```typescript
export interface ScheduledTask {
  id: string;
  user_id: string;
  cron_expr: string;
  prompt: string;
  condition: string;
  webhook_url: string;
  enabled: boolean;
  last_run_at: string | null;
  last_result: string;
  last_status: string;
  created_at: string;
  updated_at: string;
}
```

## API Client

```typescript
listSchedules(): Promise<ScheduledTask[]>        // GET /v1/schedules
createSchedule(data): Promise<ScheduledTask>      // POST /v1/schedules
updateSchedule(id, data): Promise<ScheduledTask>  // PATCH /v1/schedules/{id}
deleteSchedule(id): Promise<void>                 // DELETE /v1/schedules/{id}
```

## Widget Layout

```
┌─ Scheduled Tasks ──────────────────────────────────────┐
│ [+ Add Schedule]                                [Refresh]│
├────────────────────────────────────────────────────────┤
│ Prompt          │ Cron       │ Enabled │ Last Run │ Act │
│ Check news...   │ 0 9 * * *  │  [✓]    │ 2h ago   │ ✎ 🗑│
│ Backup vault    │ 0 0 * * 0  │  [✓]    │ 5d ago   │ ✎ 🗑│
├────────────────────────────────────────────────────────┤
│ [Create/Edit form - shown inline when active]          │
│ Prompt: [________________________]                     │
│ Cron:   [________]  Condition: [__________] (optional) │
│ Webhook: [_______________________] (optional)          │
│ [x] Enabled          [Save] [Cancel]                   │
└────────────────────────────────────────────────────────┘
```

## Interactions

| Action | Behavior |
|--------|----------|
| Add Schedule | Show inline form below table |
| Edit (pencil) | Show inline form prefilled with task data |
| Delete (trash) | Confirm dialog, then DELETE |
| Toggle enabled | Instant PATCH with toggled enabled field |
| Refresh | Re-fetch list |
| Save | POST (create) or PATCH (edit), close form, refresh |
| Cancel | Close form |

## Empty State

"No scheduled tasks yet. Click 'Add Schedule' to create one."
