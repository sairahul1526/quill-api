package httpapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/sairahul1526/quill-api/internal/store"
)

func testServer() http.Handler { return New(store.NewMemory(), "test-token", "hook-secret") }
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
func TestCreateQueueRejectsInvalidConcurrency(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/v1/queues", bytes.NewBufferString("{\"name\":\"emails\",\"concurrency\":0,\"maxAttempts\":5}"))
	req.Header.Set("Authorization", "Bearer test-token")
	rec := httptest.NewRecorder()
	testServer().ServeHTTP(rec, req)
	if rec.Code != 422 {
		t.Fatalf("status=%d, want 422", rec.Code)
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
