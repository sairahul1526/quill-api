package store

import (
	"testing"

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
