package server

import (
	"bytes"
	"context"
	"net"
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

// The announcement is tested without binding: listening on every interface
// would make the operating system firewall prompt on each test run.
func TestAnnounceStatesListenerReachability(t *testing.T) {
	cases := []struct {
		name        string
		addr        *net.TCPAddr
		requested   string
		inContainer bool
		wantOut     []string
		wantErr     []string
	}{
		{
			name:      "native loopback is quiet",
			addr:      &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 8080},
			requested: "127.0.0.1:8080",
			wantOut:   []string{"listening at http://127.0.0.1:8080"},
		},
		{
			name:      "native IPv4 wildcard warns",
			addr:      &net.TCPAddr{IP: net.IPv4zero, Port: 8080},
			requested: "0.0.0.0:8080",
			wantOut:   []string{"listening at http://127.0.0.1:8080"},
			wantErr:   []string{"warning: --listen 0.0.0.0:8080 accepts connections from other machines"},
		},
		{
			name:      "native IPv6 wildcard warns",
			addr:      &net.TCPAddr{IP: net.IPv6unspecified, Port: 8080},
			requested: ":8080",
			wantOut:   []string{"listening at http://127.0.0.1:8080"},
			wantErr:   []string{"accepts connections from other machines"},
		},
		{
			name:      "native interface address warns",
			addr:      &net.TCPAddr{IP: net.IPv4(192, 168, 1, 20), Port: 8080},
			requested: "192.168.1.20:8080",
			wantOut:   []string{"listening at http://192.168.1.20:8080"},
			wantErr:   []string{"warning: 192.168.1.20:8080 accepts connections from other machines"},
		},
		{
			name:        "container loopback explains publishing",
			addr:        &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 8080},
			requested:   "127.0.0.1:8080",
			inContainer: true,
			wantOut:     []string{"listening at http://127.0.0.1:8080"},
			wantErr:     []string{"--listen 0.0.0.0:8080", "--publish 127.0.0.1:8080:8080"},
		},
		{
			name:        "container wildcard points to the host loopback",
			addr:        &net.TCPAddr{IP: net.IPv6unspecified, Port: 8080},
			requested:   "0.0.0.0:8080",
			inContainer: true,
			wantOut:     []string{"listening on container port 8080", "http://127.0.0.1:8080 with --publish 127.0.0.1:8080:8080"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var out, errs bytes.Buffer
			url := announce(tc.addr, tc.requested, tc.inContainer, &out, &errs)
			for _, want := range tc.wantOut {
				if !strings.Contains(out.String(), want) {
					t.Errorf("output %q does not contain %q", out.String(), want)
				}
			}
			for _, want := range tc.wantErr {
				if !strings.Contains(errs.String(), want) {
					t.Errorf("errors %q do not contain %q", errs.String(), want)
				}
			}
			if len(tc.wantErr) == 0 && errs.Len() > 0 {
				t.Errorf("unexpected errors output %q", errs.String())
			}
			if strings.Contains(out.String(), "0.0.0.0") || strings.Contains(out.String(), "[::]") || strings.Contains(url, "[::]") {
				t.Errorf("announced an unspecified address a browser cannot open: %q (url %q)", out.String(), url)
			}
		})
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
