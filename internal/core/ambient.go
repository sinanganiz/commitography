package core

import (
	"crypto/rand"
	"io/fs"
	mathrand "math/rand/v2"
	"os"
	"time"
)

// The clock, randomness and filesystem are the three ambient sources a run
// could otherwise reach for directly, and each one would make output vary
// between runs or tie a test to the machine. They are injected instead
// (ADR-0042 clause 4): the entry points construct the real ones below once, at
// composition, and every consumer receives them through its constructor. Tests
// pass fixed ones, which is what lets them run in parallel with no shared
// state (ADR-0042 clause 5).

// Clock reads the current time.
type Clock interface {
	Now() time.Time
}

// ClockFunc adapts a function to a Clock.
type ClockFunc func() time.Time

// Now returns the function's result.
func (f ClockFunc) Now() time.Time { return f() }

// SystemClock returns the process clock. It is the one place in the tree that
// reads it; everything else receives a Clock.
func SystemClock() Clock {
	return ClockFunc(time.Now) //nolint:forbidigo // ADR-0042 clause 4: the real clock every consumer receives by injection.
}

// FixedClock returns a clock that always reads t.
func FixedClock(t time.Time) Clock {
	return ClockFunc(func() time.Time { return t })
}

// Random is a source of random bytes.
type Random interface {
	Read(p []byte) (int, error)
}

// SystemRandom returns the operating system's cryptographic random source,
// which is what session secrets and job identifiers are drawn from.
func SystemRandom() Random {
	return rand.Reader
}

// SeededRandom returns a deterministic source: the same seed yields the same
// bytes. It is for tests, which need randomness they can repeat.
func SeededRandom(seed uint64) Random {
	var key [32]byte
	for i := range 8 {
		key[i] = byte(seed >> (8 * i))
	}
	return mathrand.NewChaCha8(key)
}

// Filesystem is the read access an analysis has to files it did not write:
// the configuration file, the repository's attribute file, its git directory
// and its working tree. It is deliberately this narrow; writing output and
// validating request paths are not analysis input and do not go through it.
type Filesystem interface {
	Open(name string) (fs.File, error)
	ReadFile(name string) ([]byte, error)
	Stat(name string) (fs.FileInfo, error)
}

// SystemFilesystem returns the operating system's filesystem.
func SystemFilesystem() Filesystem {
	return osFilesystem{}
}

type osFilesystem struct{}

func (osFilesystem) Open(name string) (fs.File, error)     { return os.Open(name) }
func (osFilesystem) ReadFile(name string) ([]byte, error)  { return os.ReadFile(name) }
func (osFilesystem) Stat(name string) (fs.FileInfo, error) { return os.Stat(name) }
