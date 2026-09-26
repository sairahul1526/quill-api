package model

import "time"

type Task struct {
	ID          string            `json:"id"`
	Queue       string            `json:"queue"`
	Payload     map[string]string `json:"payload"`
	State       string            `json:"state"`
	Attempts    int               `json:"attempts"`
	Retries     int               `json:"retries"`
	CreatedAt   time.Time         `json:"createdAt"`
	ScheduledAt *time.Time        `json:"scheduledAt,omitempty"`
}

type Queue struct {
	Name        string `json:"name"`
	Concurrency int    `json:"concurrency"`
	MaxAttempts int    `json:"maxAttempts"`
	Retries     int    `json:"retries"`
}

type Schedule struct {
	ID        string    `json:"id"`
	Queue     string    `json:"queue"`
	Interval  string    `json:"interval"`
	TimeZone  string    `json:"timeZone"`
	NextRunAt time.Time `json:"nextRunAt"`
}

type CreateTaskRequest struct {
	Queue       string            `json:"queue"`
	Payload     map[string]string `json:"payload"`
	ScheduledAt *time.Time        `json:"scheduledAt,omitempty"`
	Retries     *int              `json:"retries,omitempty"`
}
type CreateQueueRequest struct {
	Name        string `json:"name"`
	Concurrency int    `json:"concurrency"`
	MaxAttempts int    `json:"maxAttempts"`
	Retries     *int   `json:"retries,omitempty"`
}
type CreateScheduleRequest struct {
	Queue    string `json:"queue"`
	Interval string `json:"interval"`
	TimeZone string `json:"timeZone"`
}
