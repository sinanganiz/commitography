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
	"strconv"
	"time"

	"github.com/sinanganiz/commitography/internal/container"
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
	// InContainer reports whether the process runs inside a container. Nil
	// detects it from the marker files Docker and Podman create.
	InContainer func() bool
}

// Serve binds the local HTTP listener and blocks until the context is
// cancelled or the server exits. It announces where the server can be opened
// after the listener is ready and treats browser-launch failures as warnings.
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

	inContainer := opts.InContainer
	if inContainer == nil {
		inContainer = container.Running
	}
	url := announce(listener.Addr(), opts.ListenAddress, inContainer(), opts.Output, opts.Errors)
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

// announce prints where the server can be opened and returns that URL. A
// listener other machines can reach is never silent about it. Inside a
// container the published host port is invisible, so the container listener
// explains how the host reaches it instead of printing an address such as
// 0.0.0.0 that a browser cannot open. Warnings are written before the
// announcement.
func announce(addr net.Addr, requested string, inContainer bool, out, errs io.Writer) string {
	tcp, ok := addr.(*net.TCPAddr)
	if !ok {
		url := "http://" + addr.String()
		fmt.Fprintf(out, "Commitography listening at %s\n", url)
		return url
	}
	port := strconv.Itoa(tcp.Port)
	switch {
	case tcp.IP.IsLoopback():
		if inContainer {
			fmt.Fprintf(errs, "note: a loopback listener inside a container is not reachable through a published port; "+
				"run serve with --listen 0.0.0.0:%s and publish it with --publish 127.0.0.1:%s:%s\n", port, port, port)
		}
	case tcp.IP.IsUnspecified():
		url := "http://" + net.JoinHostPort("127.0.0.1", port)
		if inContainer {
			fmt.Fprintf(out, "Commitography listening on container port %s\n"+
				"Open it on the host through a loopback port, for example http://127.0.0.1:%s with --publish 127.0.0.1:%s:%s\n",
				port, port, port, port)
			return url
		}
		fmt.Fprintf(errs, "warning: --listen %s accepts connections from other machines; use 127.0.0.1:%s unless that is intended\n",
			requested, port)
		fmt.Fprintf(out, "Commitography listening at %s\n", url)
		return url
	default:
		if !inContainer {
			fmt.Fprintf(errs, "warning: %s accepts connections from other machines on that network\n",
				net.JoinHostPort(tcp.IP.String(), port))
		}
	}
	url := "http://" + net.JoinHostPort(tcp.IP.String(), port)
	fmt.Fprintf(out, "Commitography listening at %s\n", url)
	return url
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
