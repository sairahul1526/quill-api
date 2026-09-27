package model

import "time"

type TaskPriority string

const (
	PriorityLow    TaskPriority = "low"
	PriorityNormal TaskPriority = "normal"
	PriorityHigh   TaskPriority = "high"
)

func (p TaskPriority) Valid() bool {
	return p == PriorityLow || p == PriorityNormal || p == PriorityHigh
}

type RetryPolicy struct {
	MaxAttempts    int `json:"max_attempts"`
	BackoffSeconds int `json:"backoff_seconds"`
}

type CreateRetryPolicy struct {
	MaxAttempts    *int `json:"max_attempts"`
	BackoffSeconds *int `json:"backoff_seconds"`
}

type Task struct {
	ID          string            `json:"id"`
	Queue       string            `json:"queue"`
	Payload     map[string]string `json:"payload"`
	Priority    TaskPriority      `json:"priority"`
	State       string            `json:"state"`
	Attempts    int               `json:"attempts"`
	Retry       RetryPolicy       `json:"retry"`
	TTLSeconds  *int              `json:"ttl_seconds,omitempty"`
	CreatedAt   time.Time         `json:"createdAt"`
	ScheduledAt *time.Time        `json:"scheduledAt,omitempty"`
}

type Queue struct {
	Name           string `json:"name"`
	Concurrency    int    `json:"concurrency"`
	MaxConcurrency *int   `json:"max_concurrency,omitempty"`
	MaxAttempts    int    `json:"maxAttempts"`
	Retries        int    `json:"retries"`
	Paused         bool   `json:"paused"`
}

type Schedule struct {
	ID        string    `json:"id"`
	Queue     string    `json:"queue"`
	Interval  string    `json:"interval"`
	TimeZone  string    `json:"timeZone"`
	NextRunAt time.Time `json:"nextRunAt"`
}

type CreateTaskRequest struct {
	Queue       string             `json:"queue"`
	Payload     map[string]string  `json:"payload"`
	Priority    *TaskPriority      `json:"priority,omitempty"`
	Retry       *CreateRetryPolicy `json:"retry,omitempty"`
	ScheduledAt *time.Time         `json:"scheduledAt,omitempty"`
	TTLSeconds  *int               `json:"ttl_seconds,omitempty"`
}
type CreateQueueRequest struct {
	Name           string `json:"name"`
	Concurrency    int    `json:"concurrency"`
	MaxConcurrency *int   `json:"max_concurrency,omitempty"`
	MaxAttempts    int    `json:"maxAttempts"`
	Retries        *int   `json:"retries,omitempty"`
}
type CreateScheduleRequest struct {
	Queue    string `json:"queue"`
	Interval string `json:"interval"`
	TimeZone string `json:"timeZone"`
}
