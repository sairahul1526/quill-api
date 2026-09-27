package store

import (
	"errors"
	"testing"
	"time"

	"github.com/sairahul1526/quill-api/internal/model"
)

func TestClaimTaskUsesPriorityAndDefaultsToNormal(t *testing.T) {
	memory := NewMemory()
	if _, err := memory.CreateQueue(model.CreateQueueRequest{Name: "emails", Concurrency: 1}); err != nil {
		t.Fatalf("create queue: %v", err)
	}

	lowPriority := model.PriorityLow
	highPriority := model.PriorityHigh
	low := memory.CreateTask(model.CreateTaskRequest{Queue: "emails", Payload: map[string]string{"id": "low"}, Priority: &lowPriority})
	normal := memory.CreateTask(model.CreateTaskRequest{Queue: "emails", Payload: map[string]string{"id": "normal"}})
	high := memory.CreateTask(model.CreateTaskRequest{Queue: "emails", Payload: map[string]string{"id": "high"}, Priority: &highPriority})
	if normal.Priority != model.PriorityNormal {
		t.Fatalf("default priority=%q, want %q", normal.Priority, model.PriorityNormal)
	}

	for _, want := range []model.Task{high, normal, low} {
		got, err := memory.ClaimTask("emails")
		if err != nil {
			t.Fatalf("claim task: %v", err)
		}
		if got.ID != want.ID {
			t.Fatalf("claimed task priority=%q id=%q, want priority=%q id=%q", got.Priority, got.ID, want.Priority, want.ID)
		}
	}
}

func TestClaimTaskEnforcesQueueMaxConcurrencyUntilTaskCompletes(t *testing.T) {
	memory := NewMemory()
	limit := 1
	if _, err := memory.CreateQueue(model.CreateQueueRequest{Name: "emails", Concurrency: 4, MaxConcurrency: &limit}); err != nil {
		t.Fatalf("create queue: %v", err)
	}
	memory.CreateTask(model.CreateTaskRequest{Queue: "emails", Payload: map[string]string{"id": "first"}})
	memory.CreateTask(model.CreateTaskRequest{Queue: "emails", Payload: map[string]string{"id": "second"}})

	running, err := memory.ClaimTask("emails")
	if err != nil {
		t.Fatalf("claim first task: %v", err)
	}
	if _, err := memory.ClaimTask("emails"); !errors.Is(err, ErrQueueConcurrencyLimit) {
		t.Fatalf("claim at concurrency limit error=%v, want ErrQueueConcurrencyLimit", err)
	}
	if _, err := memory.CompleteTask(running.ID); err != nil {
		t.Fatalf("complete first task: %v", err)
	}
	if _, err := memory.ClaimTask("emails"); err != nil {
		t.Fatalf("claim after a running task completes: %v", err)
	}
}

func TestClaimTaskAllowsUnlimitedConcurrencyWhenUnset(t *testing.T) {
	memory := NewMemory()
	if _, err := memory.CreateQueue(model.CreateQueueRequest{Name: "emails", Concurrency: 4}); err != nil {
		t.Fatalf("create queue: %v", err)
	}
	memory.CreateTask(model.CreateTaskRequest{Queue: "emails", Payload: map[string]string{"id": "first"}})
	memory.CreateTask(model.CreateTaskRequest{Queue: "emails", Payload: map[string]string{"id": "second"}})

	for range 2 {
		if _, err := memory.ClaimTask("emails"); err != nil {
			t.Fatalf("claim task with unset max_concurrency: %v", err)
		}
	}
}

func TestExpiredQueuedTaskIsMarkedExpiredAndNeverClaimed(t *testing.T) {
	memory := NewMemory()
	if _, err := memory.CreateQueue(model.CreateQueueRequest{Name: "emails", Concurrency: 1}); err != nil {
		t.Fatalf("create queue: %v", err)
	}
	ttlSeconds := 1
	task := memory.CreateTask(model.CreateTaskRequest{Queue: "emails", Payload: map[string]string{}, TTLSeconds: &ttlSeconds})

	memory.mu.Lock()
	stored := memory.tasks[task.ID]
	stored.CreatedAt = time.Now().UTC().Add(-2 * time.Second)
	memory.tasks[task.ID] = stored
	memory.mu.Unlock()

	if _, err := memory.ClaimTask("emails"); !errors.Is(err, ErrNoTaskAvailable) {
		t.Fatalf("claim expired task error=%v, want ErrNoTaskAvailable", err)
	}
	tasks := memory.ListTasks()
	if len(tasks) != 1 || tasks[0].State != "expired" || tasks[0].Attempts != 0 {
		t.Fatalf("expired task=%+v, want state expired and zero attempts", tasks)
	}
}

func TestTaskWithoutTTLDoesNotExpire(t *testing.T) {
	memory := NewMemory()
	if _, err := memory.CreateQueue(model.CreateQueueRequest{Name: "emails", Concurrency: 1}); err != nil {
		t.Fatalf("create queue: %v", err)
	}
	task := memory.CreateTask(model.CreateTaskRequest{Queue: "emails", Payload: map[string]string{}})

	memory.mu.Lock()
	stored := memory.tasks[task.ID]
	stored.CreatedAt = time.Now().UTC().Add(-24 * time.Hour)
	memory.tasks[task.ID] = stored
	memory.mu.Unlock()

	claimed, err := memory.ClaimTask("emails")
	if err != nil {
		t.Fatalf("claim task without TTL: %v", err)
	}
	if claimed.State != "running" {
		t.Fatalf("task state=%q, want running", claimed.State)
	}
}
