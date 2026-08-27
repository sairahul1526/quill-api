# Quill Cloud API

A Go 1.26 REST API for durable tasks, queues, schedules, webhook endpoints and bearer-token authentication.

## Run

Set QUILL_API_TOKEN and WEBHOOK_SECRET, then run go run ./cmd/api. The server listens on :8080 by default. GET /healthz is public; /v1 routes require a bearer token.

## Endpoints

- GET and POST /v1/tasks
- POST /v1/tasks/{id}/cancel
- GET and POST /v1/queues
- GET and POST /v1/schedules
- POST /v1/webhooks
- POST /webhooks/tasks (HMAC-SHA256 signature required)

The in-memory store keeps this sample self-contained. A production deployment would use a durable database and worker pool. The API default retry policy allows 5 attempts.
