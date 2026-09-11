package server

import (
	"bytes"
	"context"
	"net/http"
	"strings"
	"testing"
	"time"
)

type messageWriter chan string

func (w messageWriter) Write(data []byte) (int, error) {
	w <- string(data)
	return len(data), nil
}

func TestServeBindsAndPrintsURL(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	out := make(messageWriter, 1)
	errCh := make(chan error, 1)
	go func() {
		errCh <- Serve(ctx, Options{
			ListenAddress: "127.0.0.1:0",
			Output:        out,
			Errors:        &bytes.Buffer{},
			Handler:       http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusNoContent) }),
		})
	}()

	select {
	case message := <-out:
		if !strings.Contains(message, "http://127.0.0.1:") {
			t.Fatalf("listener message = %q", message)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("server did not announce its listener")
	}
	cancel()

	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("Serve: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("server did not shut down after cancellation")
	}
}

func TestServeBrowserFailureIsWarning(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	out := make(messageWriter, 1)
	var errors bytes.Buffer
	errCh := make(chan error, 1)
	go func() {
		errCh <- Serve(ctx, Options{
			ListenAddress: "127.0.0.1:0",
			Open:          true,
			Output:        out,
			Errors:        &errors,
			OpenBrowser:   func(string) error { return context.Canceled },
		})
	}()

	select {
	case <-out:
	case <-time.After(2 * time.Second):
		t.Fatal("server did not announce its listener")
	}
	cancel()
	if err := <-errCh; err != nil {
		t.Fatalf("Serve: %v", err)
	}
	if !strings.Contains(errors.String(), "warning: could not open browser") {
		t.Fatalf("browser warning = %q", errors.String())
	}
}

func TestServeCallsShutdownHook(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	out := make(messageWriter, 1)
	called := make(chan struct{}, 1)
	errCh := make(chan error, 1)
	go func() {
		errCh <- Serve(ctx, Options{
			ListenAddress: "127.0.0.1:0",
			Output:        out,
			OnShutdown:    func() { called <- struct{}{} },
		})
	}()
	select {
	case <-out:
	case <-time.After(2 * time.Second):
		t.Fatal("server did not announce its listener")
	}
	cancel()
	select {
	case <-called:
	case <-time.After(2 * time.Second):
		t.Fatal("shutdown hook was not called")
	}
	if err := <-errCh; err != nil {
		t.Fatalf("Serve: %v", err)
	}
}
