package server

import (
	"context"
	"io"
	"net"
	"strings"
	"testing"
)

// The opener starts a process, so it refuses anything but the server's own
// address before starting one (ADR-0065 clause 5). None of these cases reaches
// a process.
func TestBrowserOpenerRefusesForeignURLs(t *testing.T) {
	t.Parallel()
	loopback := &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 8080}
	everywhere := &net.TCPAddr{IP: net.IPv4zero, Port: 9090}
	for _, tc := range []struct {
		name string
		addr net.Addr
		url  string
	}{
		{"other host", loopback, "http://attacker.example/"},
		{"other port", loopback, "http://127.0.0.1:8081"},
		{"other scheme", loopback, "file:///etc/passwd"},
		{"option-shaped argument", loopback, "--help"},
		{"extra path", loopback, "http://127.0.0.1:8080/../x"},
		{"unspecified address", everywhere, "http://0.0.0.0:9090"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := openBrowser(context.Background(), tc.addr, tc.url)
			if err == nil || !strings.Contains(err.Error(), "refusing to open") {
				t.Fatalf("openBrowser(%q) = %v, want a refusal before any process starts", tc.url, err)
			}
		})
	}
}

// Every URL the server announces is one the opener accepts, so the refusal
// above never blocks the real call.
func TestBrowserOpenerAcceptsTheAnnouncedAddress(t *testing.T) {
	t.Parallel()
	for _, addr := range []net.Addr{
		&net.TCPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 8080},
		&net.TCPAddr{IP: net.IPv4zero, Port: 9090},
		&net.TCPAddr{IP: net.IPv6loopback, Port: 7070},
		&net.TCPAddr{IP: net.IPv4(192, 168, 1, 20), Port: 6060},
	} {
		for _, inContainer := range []bool{false, true} {
			url := announce(addr, addr.String(), inContainer, io.Discard, io.Discard)
			if !isOwnURL(addr, url) {
				t.Errorf("announce(%s, container=%v) = %q, which the opener would refuse", addr, inContainer, url)
			}
		}
	}
}
