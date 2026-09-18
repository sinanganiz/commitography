package core

import (
	"bytes"
	"testing"
	"time"
)

func TestFixedClockReadsOneInstant(t *testing.T) {
	t.Parallel()
	at := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	clock := FixedClock(at)
	if got := clock.Now(); !got.Equal(at) {
		t.Fatalf("Now() = %s, want %s", got, at)
	}
	if got := clock.Now(); !got.Equal(at) {
		t.Fatalf("second Now() = %s, want %s", got, at)
	}
}

func TestSeededRandomRepeatsForOneSeed(t *testing.T) {
	t.Parallel()
	read := func(seed uint64) []byte {
		buf := make([]byte, 32)
		if _, err := SeededRandom(seed).Read(buf); err != nil {
			t.Fatal(err)
		}
		return buf
	}
	if !bytes.Equal(read(7), read(7)) {
		t.Fatal("one seed produced two different byte sequences")
	}
	if bytes.Equal(read(7), read(8)) {
		t.Fatal("two seeds produced the same byte sequence")
	}
}
