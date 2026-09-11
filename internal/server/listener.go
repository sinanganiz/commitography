// Package server contains the local web runner and its HTTP lifecycle.
package server

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"time"
)

const defaultListenAddress = "127.0.0.1:8080"

// Options controls the local server listener. AllowedRoots is carried forward
// for the path-validation layer; listener setup itself does not inspect paths.
type Options struct {
	ListenAddress string
	Open          bool
	AllowedRoots  []string
	Handler       http.Handler
	Output        io.Writer
	Errors        io.Writer
	OpenBrowser   func(string) error
	OnShutdown    func()
}

// Serve binds the local HTTP listener and blocks until the context is
// cancelled or the server exits. It prints the bound URL after the listener is
// ready and treats browser-launch failures as warnings.
func Serve(ctx context.Context, opts Options) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if opts.ListenAddress == "" {
		opts.ListenAddress = defaultListenAddress
	}
	if opts.Output == nil {
		opts.Output = os.Stdout
	}
	if opts.Errors == nil {
		opts.Errors = os.Stderr
	}
	if opts.Handler == nil {
		opts.Handler = http.NotFoundHandler()
	}

	listener, err := net.Listen("tcp", opts.ListenAddress)
	if err != nil {
		return fmt.Errorf("listening on %s: %w", opts.ListenAddress, err)
	}

	url := "http://" + listener.Addr().String()
	fmt.Fprintf(opts.Output, "Commitography listening at %s\n", url)
	if opts.Open {
		opener := opts.OpenBrowser
		if opener == nil {
			opener = openBrowser
		}
		if err := opener(url); err != nil {
			fmt.Fprintf(opts.Errors, "warning: could not open browser: %v\n", err)
		}
	}

	httpServer := &http.Server{Handler: opts.Handler}
	go shutdownOnContext(ctx, httpServer, opts.OnShutdown)
	err = httpServer.Serve(listener)
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}

func shutdownOnContext(ctx context.Context, httpServer *http.Server, onShutdown func()) {
	<-ctx.Done()
	if onShutdown != nil {
		onShutdown()
	}
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = httpServer.Shutdown(shutdownCtx)
}

func openBrowser(url string) error {
	var command string
	var args []string
	switch runtime.GOOS {
	case "windows":
		command = "rundll32"
		args = []string{"url.dll,FileProtocolHandler", url}
	case "darwin":
		command = "open"
		args = []string{url}
	default:
		command = "xdg-open"
		args = []string{url}
	}
	return exec.Command(command, args...).Run()
}
