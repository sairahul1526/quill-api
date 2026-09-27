package httpapi

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/sairahul1526/quill-api/internal/model"
	"github.com/sairahul1526/quill-api/internal/store"
)

func testServer() http.Handler { return New(store.NewMemory(), "test-token", "hook-secret") }
func testServerWithStore(memory *store.Memory) http.Handler {
	return New(memory, "test-token", "hook-secret")
}
func TestTaskRequiresBearerToken(t *testing.T) {
	rec := httptest.NewRecorder()
	testServer().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/v1/tasks", nil))
	if rec.Code != 401 {
		t.Fatalf("status=%d, want 401", rec.Code)
	}
}
func TestCreateAndCancelTask(t *testing.T) {
	h := testServer()
	req := httptest.NewRequest(http.MethodPost, "/v1/tasks", bytes.NewBufferString("{\"queue\":\"emails\",\"payload\":{\"recipient\":\"a@example.test\"}}"))
	req.Header.Set("Authorization", "Bearer test-token")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != 201 {
		t.Fatalf("create status=%d body=%s", rec.Code, rec.Body.String())
	}
	var task struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &task); err != nil {
		t.Fatal(err)
	}
	cancel := httptest.NewRequest(http.MethodPost, "/v1/tasks/"+task.ID+"/cancel", nil)
	cancel.Header.Set("Authorization", "Bearer test-token")
	out := httptest.NewRecorder()
	h.ServeHTTP(out, cancel)
	if out.Code != 200 {
		t.Fatalf("cancel status=%d", out.Code)
	}
}
func TestCancelFinishedTaskReturnsConflict(t *testing.T) {
	memory := store.NewMemory()
	task := memory.CreateTask(model.CreateTaskRequest{Queue: "emails", Payload: map[string]string{"recipient": "a@example.test"}})
	if _, err := memory.CompleteTask(task.ID); err != nil {
		t.Fatalf("complete task: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, "/v1/tasks/"+task.ID+"/cancel", nil)
	req.Header.Set("Authorization", "Bearer test-token")
	rec := httptest.NewRecorder()
	testServerWithStore(memory).ServeHTTP(rec, req)
	if rec.Code != http.StatusConflict {
		t.Fatalf("cancel status=%d body=%s, want %d", rec.Code, rec.Body.String(), http.StatusConflict)
	}
}
func TestTaskRetryPolicyDefaultsToFiveAttempts(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/v1/tasks", bytes.NewBufferString(`{"queue":"emails","payload":{"recipient":"a@example.test"}}`))
	req.Header.Set("Authorization", "Bearer test-token")
	rec := httptest.NewRecorder()
	testServer().ServeHTTP(rec, req)
	if rec.Code != 201 {
		t.Fatalf("create status=%d body=%s", rec.Code, rec.Body.String())
	}
	var task model.Task
	if err := json.Unmarshal(rec.Body.Bytes(), &task); err != nil {
		t.Fatal(err)
	}
	if task.Retry.MaxAttempts != 5 || task.Retry.BackoffSeconds != 0 {
		t.Fatalf("retry=%+v, want max_attempts=5 and backoff_seconds=0", task.Retry)
	}
}

func TestCreateTaskAcceptsRetryPolicy(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/v1/tasks", bytes.NewBufferString(`{"queue":"emails","payload":{},"retry":{"max_attempts":3,"backoff_seconds":10}}`))
	req.Header.Set("Authorization", "Bearer test-token")
	rec := httptest.NewRecorder()
	testServer().ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create status=%d body=%s, want %d", rec.Code, rec.Body.String(), http.StatusCreated)
	}
	var task model.Task
	if err := json.Unmarshal(rec.Body.Bytes(), &task); err != nil {
		t.Fatal(err)
	}
	if task.Retry.MaxAttempts != 3 || task.Retry.BackoffSeconds != 10 {
		t.Fatalf("retry=%+v, want max_attempts=3 and backoff_seconds=10", task.Retry)
	}
}

func TestCreateTaskRejectsLegacyRetriesWithMigrationMessage(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/v1/tasks", bytes.NewBufferString(`{"queue":"emails","payload":{},"retries":3}`))
	req.Header.Set("Authorization", "Bearer test-token")
	rec := httptest.NewRecorder()
	testServer().ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("create status=%d body=%s, want %d", rec.Code, rec.Body.String(), http.StatusBadRequest)
	}
	if !strings.Contains(rec.Body.String(), "retry.max_attempts") {
		t.Fatalf("error body=%s, want migration guidance to retry.max_attempts", rec.Body.String())
	}
}

func TestCreateTaskAcceptsTTLSeconds(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/v1/tasks", bytes.NewBufferString(`{"queue":"emails","payload":{},"ttl_seconds":300}`))
	req.Header.Set("Authorization", "Bearer test-token")
	rec := httptest.NewRecorder()
	testServer().ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create status=%d body=%s, want %d", rec.Code, rec.Body.String(), http.StatusCreated)
	}
	var task model.Task
	if err := json.Unmarshal(rec.Body.Bytes(), &task); err != nil {
		t.Fatal(err)
	}
	if task.TTLSeconds == nil || *task.TTLSeconds != 300 {
		t.Fatalf("ttl_seconds=%v, want 300", task.TTLSeconds)
	}
}

func TestCreateTaskRejectsTTLBelowOne(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/v1/tasks", bytes.NewBufferString(`{"queue":"emails","payload":{},"ttl_seconds":0}`))
	req.Header.Set("Authorization", "Bearer test-token")
	rec := httptest.NewRecorder()
	testServer().ServeHTTP(rec, req)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("create status=%d body=%s, want %d", rec.Code, rec.Body.String(), http.StatusUnprocessableEntity)
	}
}

func TestQueueRetriesDefaultToFive(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/v1/queues", bytes.NewBufferString(`{"name":"emails","concurrency":4}`))
	req.Header.Set("Authorization", "Bearer test-token")
	rec := httptest.NewRecorder()
	testServer().ServeHTTP(rec, req)
	if rec.Code != 201 {
		t.Fatalf("create status=%d body=%s", rec.Code, rec.Body.String())
	}
	var queue struct {
		Retries     int `json:"retries"`
		MaxAttempts int `json:"maxAttempts"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &queue); err != nil {
		t.Fatal(err)
	}
	if queue.Retries != 5 || queue.MaxAttempts != 5 {
		t.Fatalf("retries=%d maxAttempts=%d, want both 5", queue.Retries, queue.MaxAttempts)
	}
}

func TestListQueuesShowsMaxConcurrencyOrUnlimited(t *testing.T) {
	memory := store.NewMemory()
	limit := 3
	if _, err := memory.CreateQueue(model.CreateQueueRequest{Name: "limited", Concurrency: 4, MaxConcurrency: &limit}); err != nil {
		t.Fatalf("create limited queue: %v", err)
	}
	if _, err := memory.CreateQueue(model.CreateQueueRequest{Name: "unlimited", Concurrency: 4}); err != nil {
		t.Fatalf("create unlimited queue: %v", err)
	}
	req := httptest.NewRequest(http.MethodGet, "/v1/queues", nil)
	req.Header.Set("Authorization", "Bearer test-token")
	rec := httptest.NewRecorder()
	testServerWithStore(memory).ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("list status=%d body=%s, want %d", rec.Code, rec.Body.String(), http.StatusOK)
	}
	var queues []struct {
		Name           string `json:"name"`
		MaxConcurrency any    `json:"max_concurrency"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &queues); err != nil {
		t.Fatal(err)
	}
	if len(queues) != 2 {
		t.Fatalf("listed %d queues, want 2", len(queues))
	}
	got := map[string]any{}
	for _, queue := range queues {
		got[queue.Name] = queue.MaxConcurrency
	}
	if got["limited"] != float64(3) {
		t.Fatalf("limited max_concurrency=%v, want 3", got["limited"])
	}
	if got["unlimited"] != "unlimited" {
		t.Fatalf("unlimited max_concurrency=%v, want unlimited", got["unlimited"])
	}
}

func TestCreateQueueRejectsInvalidConcurrency(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/v1/queues", bytes.NewBufferString("{\"name\":\"emails\",\"concurrency\":0,\"maxAttempts\":5}"))
	req.Header.Set("Authorization", "Bearer test-token")
	rec := httptest.NewRecorder()
	testServer().ServeHTTP(rec, req)
	if rec.Code != 422 {
		t.Fatalf("status=%d, want 422", rec.Code)
	}
}

func TestCreateQueueRejectsMaxConcurrencyBelowOne(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/v1/queues", bytes.NewBufferString(`{"name":"emails","concurrency":4,"max_concurrency":0}`))
	req.Header.Set("Authorization", "Bearer test-token")
	rec := httptest.NewRecorder()
	testServer().ServeHTTP(rec, req)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status=%d body=%s, want %d", rec.Code, rec.Body.String(), http.StatusUnprocessableEntity)
	}
}

func TestCreateQueueSetsMaxConcurrency(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/v1/queues", bytes.NewBufferString(`{"name":"emails","concurrency":4,"max_concurrency":2}`))
	req.Header.Set("Authorization", "Bearer test-token")
	rec := httptest.NewRecorder()
	testServer().ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status=%d body=%s, want %d", rec.Code, rec.Body.String(), http.StatusCreated)
	}
	var queue model.Queue
	if err := json.Unmarshal(rec.Body.Bytes(), &queue); err != nil {
		t.Fatal(err)
	}
	if queue.MaxConcurrency == nil || *queue.MaxConcurrency != 2 {
		t.Fatalf("max_concurrency=%v, want 2", queue.MaxConcurrency)
	}
}

func TestPauseQueueAcceptsTasksAndResumeAllowsWorkersToClaim(t *testing.T) {
	memory := store.NewMemory()
	if _, err := memory.CreateQueue(model.CreateQueueRequest{Name: "emails", Concurrency: 4}); err != nil {
		t.Fatalf("create queue: %v", err)
	}
	h := testServerWithStore(memory)
	request := httptest.NewRequest(http.MethodPost, "/v1/queues/emails/pause", nil)
	request.Header.Set("Authorization", "Bearer test-token")
	paused := httptest.NewRecorder()
	h.ServeHTTP(paused, request)
	if paused.Code != http.StatusOK {
		t.Fatalf("pause status=%d body=%s", paused.Code, paused.Body.String())
	}
	var queue model.Queue
	if err := json.Unmarshal(paused.Body.Bytes(), &queue); err != nil {
		t.Fatal(err)
	}
	if !queue.Paused {
		t.Fatal("queue was not marked paused")
	}

	create := httptest.NewRequest(http.MethodPost, "/v1/tasks", bytes.NewBufferString(`{"queue":"emails","payload":{"recipient":"a@example.test"}}`))
	create.Header.Set("Authorization", "Bearer test-token")
	created := httptest.NewRecorder()
	h.ServeHTTP(created, create)
	if created.Code != http.StatusCreated {
		t.Fatalf("create while paused status=%d body=%s", created.Code, created.Body.String())
	}
	if _, err := memory.ClaimTask("emails"); !errors.Is(err, store.ErrQueuePaused) {
		t.Fatalf("claim while paused error=%v, want ErrQueuePaused", err)
	}

	resume := httptest.NewRequest(http.MethodPost, "/v1/queues/emails/resume", nil)
	resume.Header.Set("Authorization", "Bearer test-token")
	resumed := httptest.NewRecorder()
	h.ServeHTTP(resumed, resume)
	if resumed.Code != http.StatusOK {
		t.Fatalf("resume status=%d body=%s", resumed.Code, resumed.Body.String())
	}
	if err := json.Unmarshal(resumed.Body.Bytes(), &queue); err != nil {
		t.Fatal(err)
	}
	if queue.Paused {
		t.Fatal("queue remained paused after resume")
	}
	claimed, err := memory.ClaimTask("emails")
	if err != nil {
		t.Fatalf("claim after resume: %v", err)
	}
	if claimed.State != "running" {
		t.Fatalf("claimed task state=%q, want running", claimed.State)
	}
}
func TestWebhookRejectsMissingSignature(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/webhooks/tasks", bytes.NewBufferString("{\"id\":\"task_1\"}"))
	rec := httptest.NewRecorder()
	testServer().ServeHTTP(rec, req)
	if rec.Code != 401 {
		t.Fatalf("status=%d, want 401", rec.Code)
	}
}
