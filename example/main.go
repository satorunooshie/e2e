package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	server := &http.Server{
		Addr:    ":8080",
		Handler: newRouter(),
	}
	go func() {
		if err := server.ListenAndServe(); err != nil {
			if errors.Is(err, http.ErrServerClosed) {
				log.Fatalf("server closed with error: %v\n", err)
			}
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGTERM, os.Interrupt)
	<-quit

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Printf("failed to gracefully shutdown: %v\n", err)
	}
}

func newRouter() http.Handler {
	mux := http.NewServeMux()

	// GET: StatusOK
	mux.HandleFunc("/v1/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"hoge":"fuga"}`))
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("/v2/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ping":"pong"}`))
		w.WriteHeader(http.StatusOK)
	})

	// GET: http.StatusOK
	// PUT: http.StatusNoContent
	mux.HandleFunc("/v1/user/1", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			switch r.URL.Query().Get("typ") {
			case "exception":
				http.Error(w, "Server error", http.StatusInternalServerError)
			case "new":
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"name":"Giorno Giovanna"}`))
				w.WriteHeader(http.StatusOK)
			default:
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"name":"JoJo"}`))
				w.WriteHeader(http.StatusOK)
			}
		case http.MethodPut:
			w.WriteHeader(http.StatusNoContent)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})

	// POST: http.StatusCreated
	mux.HandleFunc("/v1/user", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost:
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			_, _ = fmt.Fprintf(w, `{"id":1,"created_time":%d}`, time.Now().Unix())
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})
	return mux
}
