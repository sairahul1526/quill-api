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
var ErrQueuePaused = errors.New("queue is paused")
var ErrQueueConcurrencyLimit = errors.New("queue concurrency limit reached")
var ErrNoTaskAvailable = errors.New("no task available")

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
	priority := model.PriorityNormal
	if input.Priority != nil {
		priority = *input.Priority
	}
	var ttlSeconds *int
	if input.TTLSeconds != nil {
		value := *input.TTLSeconds
		ttlSeconds = &value
	}
	task := model.Task{ID: m.id("task"), Queue: input.Queue, Payload: input.Payload, Priority: priority, State: "queued", Retries: retries, TTLSeconds: ttlSeconds, CreatedAt: time.Now().UTC(), ScheduledAt: input.ScheduledAt}
	m.tasks[task.ID] = task
	return task
}
func (m *Memory) ListTasks() []model.Task {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.expireQueuedTasks(time.Now().UTC())
	out := make([]model.Task, 0, len(m.tasks))
	for _, v := range m.tasks {
		out = append(out, v)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.Before(out[j].CreatedAt) })
	return out
}
func (m *Memory) ClaimTask(queueName string) (model.Task, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.expireQueuedTasks(time.Now().UTC())
	queue, ok := m.queues[queueName]
	if !ok {
		return model.Task{}, ErrNotFound
	}
	if queue.Paused {
		return model.Task{}, ErrQueuePaused
	}
	if queue.MaxConcurrency != nil {
		running := 0
		for _, task := range m.tasks {
			if task.Queue == queueName && task.State == "running" {
				running++
			}
		}
		if running >= *queue.MaxConcurrency {
			return model.Task{}, ErrQueueConcurrencyLimit
		}
	}
	var next model.Task
	for _, task := range m.tasks {
		if task.Queue != queueName || task.State != "queued" {
			continue
		}
		if next.ID == "" || priorityRank(task.Priority) > priorityRank(next.Priority) ||
			(priorityRank(task.Priority) == priorityRank(next.Priority) && task.CreatedAt.Before(next.CreatedAt)) {
			next = task
		}
	}
	if next.ID == "" {
		return model.Task{}, ErrNoTaskAvailable
	}
	next.State = "running"
	next.Attempts++
	m.tasks[next.ID] = next
	return next, nil
}

func (m *Memory) expireQueuedTasks(now time.Time) {
	for id, task := range m.tasks {
		if task.State == "queued" && task.TTLSeconds != nil && now.Sub(task.CreatedAt).Seconds() >= float64(*task.TTLSeconds) {
			task.State = "expired"
			m.tasks[id] = task
		}
	}
}

func priorityRank(priority model.TaskPriority) int {
	switch priority {
	case model.PriorityHigh:
		return 3
	case model.PriorityLow:
		return 1
	default:
		return 2
	}
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
func (m *Memory) CompleteTask(id string) (model.Task, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	task, ok := m.tasks[id]
	if !ok {
		return model.Task{}, ErrNotFound
	}
	if task.State == "completed" || task.State == "cancelled" {
		return model.Task{}, ErrConflict
	}
	task.State = "completed"
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
	var maxConcurrency *int
	if input.MaxConcurrency != nil {
		value := *input.MaxConcurrency
		maxConcurrency = &value
	}
	q := model.Queue{Name: input.Name, Concurrency: input.Concurrency, MaxConcurrency: maxConcurrency, MaxAttempts: retries, Retries: retries}
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
func (m *Memory) PauseQueue(name string) (model.Queue, error) {
	return m.setQueuePaused(name, true)
}
func (m *Memory) ResumeQueue(name string) (model.Queue, error) {
	return m.setQueuePaused(name, false)
}
func (m *Memory) setQueuePaused(name string, paused bool) (model.Queue, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	queue, ok := m.queues[name]
	if !ok {
		return model.Queue{}, ErrNotFound
	}
	queue.Paused = paused
	m.queues[name] = queue
	return queue, nil
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
