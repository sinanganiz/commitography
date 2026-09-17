// The browser opener is one of the non-git process execution sites ADR-0065
// clause 3 permits. It starts one fixed program per platform with the URL as
// its only variable argument, never through a shell, and under a timeout
// (ADR-0065 clause 4). The only URL it accepts is the server's own listen
// address (ADR-0065 clause 5).

package server

import (
	"context"
	"net"
	"os/exec"
	"runtime"
	"strconv"
	"time"

	"github.com/sinanganiz/commitography/internal/core"
)

// browserTimeout bounds the platform opener. Each one hands the URL to the
// desktop and returns; one that does not is stopped rather than left to hold
// up the server.
const browserTimeout = 10 * time.Second

// openBrowser opens url in the default browser, provided url is the address
// the listener at addr is reachable on.
func openBrowser(ctx context.Context, addr net.Addr, url string) error {
	if !isOwnURL(addr, url) {
		// A URL that is not the server's own reaching here is a defect in the
		// caller, not something the operator expressed (ADR-0065 clause 5).
		return core.Internalf(nil, "refusing to open a URL that is not this server's listen address")
	}

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

	ctx, cancel := context.WithTimeout(ctx, browserTimeout)
	defer cancel()
	return exec.CommandContext(ctx, command, args...).Run()
}

// isOwnURL reports whether url is one announce produces for the listener at
// addr: its own host and port, or loopback when it listens on every
// interface.
func isOwnURL(addr net.Addr, url string) bool {
	tcp, ok := addr.(*net.TCPAddr)
	if !ok {
		return url == "http://"+addr.String()
	}
	port := strconv.Itoa(tcp.Port)
	if tcp.IP.IsUnspecified() {
		return url == "http://"+net.JoinHostPort("127.0.0.1", port)
	}
	return url == "http://"+net.JoinHostPort(tcp.IP.String(), port)
}
