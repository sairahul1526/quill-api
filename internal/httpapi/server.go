package httpapi

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/sairahul1526/quill-api/internal/auth"
	"github.com/sairahul1526/quill-api/internal/model"
	"github.com/sairahul1526/quill-api/internal/store"
)

type Server struct {
	store                *store.Memory
	token, webhookSecret string
	mux                  *http.ServeMux
}

func New(memory *store.Memory, token, secret string) http.Handler {
	s := &Server{store: memory, token: token, webhookSecret: secret, mux: http.NewServeMux()}
	s.routes()
	return s.mux
}
func (s *Server) routes() {
	s.mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	p := http.NewServeMux()
	p.HandleFunc("GET /v1/tasks", s.listTasks)
	p.HandleFunc("POST /v1/tasks", s.createTask)
	p.HandleFunc("POST /v1/tasks/{id}/cancel", s.cancelTask)
	p.HandleFunc("GET /v1/queues", s.listQueues)
	p.HandleFunc("POST /v1/queues", s.createQueue)
	p.HandleFunc("POST /v1/queues/{name}/pause", s.pauseQueue)
	p.HandleFunc("POST /v1/queues/{name}/resume", s.resumeQueue)
	p.HandleFunc("GET /v1/schedules", s.listSchedules)
	p.HandleFunc("POST /v1/schedules", s.createSchedule)
	p.HandleFunc("POST /v1/webhooks", s.createWebhook)
	s.mux.Handle("/v1/", auth.Bearer(s.token, p))
	s.mux.HandleFunc("POST /webhooks/tasks", s.receiveWebhook)
}
func (s *Server) listTasks(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, s.store.ListTasks())
}
func (s *Server) createTask(w http.ResponseWriter, r *http.Request) {
	var in model.CreateTaskRequest
	if !decode(w, r, &in) {
		return
	}
	if strings.TrimSpace(in.Queue) == "" || in.Payload == nil {
		writeError(w, 422, "queue and payload are required")
		return
	}
	if in.Priority != nil && !in.Priority.Valid() {
		writeError(w, 422, "priority must be low, normal or high")
		return
	}
	writeJSON(w, 201, s.store.CreateTask(in))
}
func (s *Server) cancelTask(w http.ResponseWriter, r *http.Request) {
	task, err := s.store.CancelTask(r.PathValue("id"))
	switch {
	case errors.Is(err, store.ErrNotFound):
		writeError(w, 404, "task not found")
	case errors.Is(err, store.ErrConflict):
		writeError(w, http.StatusConflict, "task already finished or cancelled")
	case err != nil:
		writeError(w, 500, "could not cancel task")
	default:
		writeJSON(w, 200, task)
	}
}
func (s *Server) listQueues(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, 200, s.store.ListQueues())
}
func (s *Server) createQueue(w http.ResponseWriter, r *http.Request) {
	var in model.CreateQueueRequest
	if !decode(w, r, &in) {
		return
	}
	if strings.TrimSpace(in.Name) == "" || in.Concurrency < 1 || in.MaxAttempts < 0 || (in.Retries != nil && *in.Retries < 0) {
		writeError(w, 422, "name and positive concurrency are required; retries cannot be negative")
		return
	}
	q, err := s.store.CreateQueue(in)
	if errors.Is(err, store.ErrConflict) {
		writeError(w, 409, "queue already exists")
		return
	}
	writeJSON(w, 201, q)
}
func (s *Server) pauseQueue(w http.ResponseWriter, r *http.Request) {
	s.setQueuePaused(w, r, true)
}
func (s *Server) resumeQueue(w http.ResponseWriter, r *http.Request) {
	s.setQueuePaused(w, r, false)
}
func (s *Server) setQueuePaused(w http.ResponseWriter, r *http.Request, paused bool) {
	var queue model.Queue
	var err error
	if paused {
		queue, err = s.store.PauseQueue(r.PathValue("name"))
	} else {
		queue, err = s.store.ResumeQueue(r.PathValue("name"))
	}
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "queue not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not update queue state")
		return
	}
	writeJSON(w, http.StatusOK, queue)
}
func (s *Server) listSchedules(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, 200, s.store.ListSchedules())
}
func (s *Server) createSchedule(w http.ResponseWriter, r *http.Request) {
	var in model.CreateScheduleRequest
	if !decode(w, r, &in) {
		return
	}
	if strings.TrimSpace(in.Queue) == "" || strings.TrimSpace(in.Interval) == "" || strings.TrimSpace(in.TimeZone) == "" {
		writeError(w, 422, "queue, interval and timeZone are required")
		return
	}
	writeJSON(w, 201, s.store.CreateSchedule(in))
}
func (s *Server) createWebhook(w http.ResponseWriter, r *http.Request) {
	var in struct {
		URL    string   `json:"url"`
		Events []string `json:"events"`
	}
	if !decode(w, r, &in) {
		return
	}
	if !strings.HasPrefix(in.URL, "https://") || len(in.Events) == 0 {
		writeError(w, 422, "an https URL and at least one event are required")
		return
	}
	writeJSON(w, 201, map[string]any{"id": "wh_quill_001", "url": in.URL, "events": in.Events, "signing": "HMAC-SHA256"})
}
func (s *Server) receiveWebhook(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 1<<20))
	if err != nil {
		writeError(w, 413, "payload exceeds 1 MiB")
		return
	}
	mac := hmac.New(sha256.New, []byte(s.webhookSecret))
	_, _ = mac.Write(body)
	want := "sha256=" + hex.EncodeToString(mac.Sum(nil))
	if s.webhookSecret == "" || !hmac.Equal([]byte(want), []byte(r.Header.Get("X-Quill-Signature"))) {
		writeError(w, 401, "invalid webhook signature")
		return
	}
	writeJSON(w, 202, map[string]string{"status": "accepted"})
}
func decode(w http.ResponseWriter, r *http.Request, target any) bool {
	d := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	d.DisallowUnknownFields()
	if err := d.Decode(target); err != nil {
		writeError(w, 400, "invalid JSON")
		return false
	}
	return true
}
func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]any{"error": map[string]string{"message": message, "status": strconv.Itoa(status)}})
}
func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
