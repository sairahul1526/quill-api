package main

import (
	"log"
	"net/http"
	"os"
	"time"

	"github.com/sairahul1526/quill-api/internal/httpapi"
	"github.com/sairahul1526/quill-api/internal/store"
)

func main() {
	addr := os.Getenv("ADDR")
	if addr == "" {
		addr = ":8080"
	}
	server := &http.Server{
		Addr:              addr,
		Handler:           httpapi.New(store.NewMemory(), os.Getenv("QUILL_API_TOKEN"), os.Getenv("WEBHOOK_SECRET")),
		ReadHeaderTimeout: 5 * time.Second,
	}
	log.Printf("Quill Cloud API listening on %s", addr)
	log.Fatal(server.ListenAndServe())
}
