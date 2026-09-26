package store

import (
	"errors"
	"github.com/google/uuid"
	"sort"
	"sync"
	"time"

	"github.com/sairahul1526/quill-api/internal/model"
)

var ErrNotFound = errors.New("resource not found")
var ErrConflict = errors.New("resource already exists")

type Memory struct {
	mu        sync.RWMutex
	tasks     map[string]model.Task
	queues    map[string]model.Queue
	schedules map[string]model.Schedule
}

func NewMemory() *Memory {
	return &Memory{tasks: map[string]model.Task{}, queues: map[string]model.Queue{}, schedules: map[string]model.Schedule{}}
}
func (m *Memory) id(prefix string) string {
	return prefix + "_" + uuid.NewString()
}
func (m *Memory) CreateTask(input model.CreateTaskRequest) model.Task {
	m.mu.Lock()
	defer m.mu.Unlock()
	retries := 5
	if input.Retries != nil {
		retries = *input.Retries
	}
	task := model.Task{ID: m.id("task"), Queue: input.Queue, Payload: input.Payload, State: "queued", Retries: retries, CreatedAt: time.Now().UTC(), ScheduledAt: input.ScheduledAt}
	m.tasks[task.ID] = task
	return task
}
func (m *Memory) ListTasks() []model.Task {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]model.Task, 0, len(m.tasks))
	for _, v := range m.tasks {
		out = append(out, v)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.Before(out[j].CreatedAt) })
	return out
}
func (m *Memory) CancelTask(id string) (model.Task, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	task, ok := m.tasks[id]
	if !ok {
		return model.Task{}, ErrNotFound
	}
	if task.State == "completed" || task.State == "cancelled" {
		return model.Task{}, ErrConflict
	}
	task.State = "cancelled"
	m.tasks[id] = task
	return task, nil
}
func (m *Memory) CreateQueue(input model.CreateQueueRequest) (model.Queue, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.queues[input.Name]; ok {
		return model.Queue{}, ErrConflict
	}
	retries := 5
	if input.Retries != nil {
		retries = *input.Retries
	} else if input.MaxAttempts > 0 {
		retries = input.MaxAttempts
	}
	q := model.Queue{Name: input.Name, Concurrency: input.Concurrency, MaxAttempts: retries, Retries: retries}
	m.queues[q.Name] = q
	return q, nil
}
func (m *Memory) ListQueues() []model.Queue {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]model.Queue, 0, len(m.queues))
	for _, v := range m.queues {
		out = append(out, v)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}
func (m *Memory) CreateSchedule(input model.CreateScheduleRequest) model.Schedule {
	m.mu.Lock()
	defer m.mu.Unlock()
	s := model.Schedule{ID: m.id("schedule"), Queue: input.Queue, Interval: input.Interval, TimeZone: input.TimeZone, NextRunAt: time.Now().UTC().Add(time.Hour)}
	m.schedules[s.ID] = s
	return s
}
func (m *Memory) ListSchedules() []model.Schedule {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]model.Schedule, 0, len(m.schedules))
	for _, v := range m.schedules {
		out = append(out, v)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].NextRunAt.Before(out[j].NextRunAt) })
	return out
}
