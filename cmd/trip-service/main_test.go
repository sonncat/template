package main

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestShutdownHTTPClosesActiveConnectionWhenDeadlinePassed(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	requestDone := make(chan struct{})

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(started)
		<-release
		w.WriteHeader(http.StatusOK)
	}))

	defer server.Close()
	defer close(release)

	go func() {
		defer close(requestDone)

		response, err := server.Client().Get(server.URL)
		if err == nil {
			_ = response.Body.Close()
		}
	}()

	select {
	case <-started:
	case <-time.After(5 * time.Second):
		t.Fatal("request did not reach handler")
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := shutdownHTTP(ctx, server.Config)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("shutdown error = %v, want context canceled", err)
	}

	select {
	case <-requestDone:
	case <-time.After(5 * time.Second):
		t.Fatal("active connection was not closed")
	}
}
