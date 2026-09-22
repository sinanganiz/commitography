package git

import (
	"bufio"
	"context"
	"strings"
	"testing"

	"github.com/sinanganiz/commitography/internal/core"
)

// objectRepository builds a repository holding the given blobs and returns it
// with each blob's object name, in order.
func objectRepository(t *testing.T, contents ...string) (string, []string) {
	t.Helper()
	ctx := context.Background()
	dir := t.TempDir()
	if _, err := Output(ctx, At("", "init", "-q", dir)); err != nil {
		t.Fatalf("creating the repository: %v", err)
	}
	names := make([]string, len(contents))
	for i, content := range contents {
		name, err := Output(ctx, At(dir, "hash-object", "-w", "--stdin").WithStdin(strings.NewReader(content)))
		if err != nil {
			t.Fatalf("writing blob %d: %v", i, err)
		}
		names[i] = name
	}
	return dir, names
}

func startObjects(t *testing.T, dir string, limit int64) *Objects {
	t.Helper()
	objects, err := OpenObjects(context.Background(), dir, limit)
	if err != nil {
		t.Fatalf("opening the object reader: %v", err)
	}
	t.Cleanup(func() {
		if err := objects.Close(); err != nil {
			t.Errorf("closing the object reader: %v", err)
		}
	})
	return objects
}

// Content is taken by its stated length, so content shaped like a response
// header, content with NUL bytes, content with no final line terminator and
// empty content all come back whole, and the stream stays in frame after each
// (ADR-0072 clause 1).
func TestReplayObjectsReadContentByName(t *testing.T) {
	t.Parallel()
	contents := []string{"plain\n", "no final line terminator", "", "\x00\x01\n\x00binary\n"}
	dir, names := objectRepository(t, contents...)
	// A blob whose content is a well-formed response header for another blob,
	// followed by that blob's content and terminator.
	forged := names[0] + " blob 6\nplain\n\n"
	forgedName, err := Output(context.Background(),
		At(dir, "hash-object", "-w", "--stdin").WithStdin(strings.NewReader(forged)))
	if err != nil {
		t.Fatalf("writing the forged blob: %v", err)
	}
	contents = append(contents, forged)
	names = append(names, forgedName)

	objects := startObjects(t, dir, 1<<20)
	// Each twice, in an order that puts the forged blob between the others.
	order := []int{4, 0, 4, 1, 2, 3, 0, 1, 2, 3}
	for _, i := range order {
		got, err := objects.Read(names[i])
		if err != nil {
			t.Fatalf("reading blob %d: %v", i, err)
		}
		if got.Type != "blob" || got.Size != int64(len(contents[i])) || string(got.Content) != contents[i] ||
			got.Oversized || got.Missing {
			t.Errorf("blob %d came back as %+v with content %q, want a blob of %d bytes %q",
				i, got, got.Content, len(contents[i]), contents[i])
		}
	}
}

// A record over the cap is skipped by its length, not held, and reported as
// oversized; the next record is read in frame (ADR-0072 clause 5).
func TestReplayObjectsSkipARecordOverTheCap(t *testing.T) {
	t.Parallel()
	dir, names := objectRepository(t, "twenty bytes, exactly", "small\n")
	objects := startObjects(t, dir, 8)

	big, err := objects.Read(names[0])
	if err != nil {
		t.Fatalf("reading the oversized blob: %v", err)
	}
	if !big.Oversized || big.Content != nil || big.Size != int64(len("twenty bytes, exactly")) {
		t.Errorf("the oversized blob came back as %+v, want it oversized, with its size and no content", big)
	}
	small, err := objects.Read(names[1])
	if err != nil {
		t.Fatalf("reading the blob after the oversized one: %v", err)
	}
	if string(small.Content) != "small\n" {
		t.Errorf("the blob after the oversized one came back as %q; the stream left its frame", small.Content)
	}
}

// An object the repository lacks is a well-framed response of its own, not a
// failure of the stream.
func TestReplayObjectsReportAMissingObject(t *testing.T) {
	t.Parallel()
	dir, names := objectRepository(t, "present\n")
	objects := startObjects(t, dir, 1<<20)

	absent := strings.Repeat("e", 40)
	got, err := objects.Read(absent)
	if err != nil {
		t.Fatalf("reading an absent object: %v", err)
	}
	if !got.Missing || got.Content != nil {
		t.Errorf("an absent object came back as %+v, want it missing", got)
	}
	if present, err := objects.Read(names[0]); err != nil || string(present.Content) != "present\n" {
		t.Errorf("the object after the absent one came back as %q, %v", present.Content, err)
	}
}

// A request is one line naming one object. A value that is not an object name
// is refused before anything is sent, and the reader stays usable.
func TestReplayObjectsRefuseAValueThatIsNotAnObjectName(t *testing.T) {
	t.Parallel()
	dir, names := objectRepository(t, "present\n")
	objects := startObjects(t, dir, 1<<20)

	for _, value := range []string{"HEAD", names[0][:12], names[0] + "\n" + names[0], strings.ToUpper(names[0]),
		names[0] + " ", ""} {
		if _, err := objects.Read(value); core.ClassOf(err) != core.ClassInternal {
			t.Errorf("asking for %q returned %v, want an internal error", value, err)
		}
	}
	if present, err := objects.Read(names[0]); err != nil || string(present.Content) != "present\n" {
		t.Errorf("a refused request left the reader unusable: %q, %v", present.Content, err)
	}
}

// discardRequests stands in for git's input in the framing test.
type discardRequests struct{}

func (discardRequests) Write(p []byte) (int, error) { return len(p), nil }
func (discardRequests) Close() error                { return nil }

// A header that does not parse, one naming another object, a stated length
// the stream contradicts and a stream that ends inside a record each abort the
// read with an internal error. Every read after that returns the same error
// and takes nothing more from the stream: in particular the well-formed record
// that follows each forged one is never returned, which a reader that
// resynchronised on the next plausible header would do (ADR-0072 clause 6).
func TestReplayObjectsRejectAForgedLengthHeader(t *testing.T) {
	t.Parallel()
	a, b := strings.Repeat("a", 40), strings.Repeat("b", 40)
	next := b + " blob 3\nxyz\n"
	cases := map[string]string{
		"a stated length shorter than the content": a + " blob 3\nabcdef\n",
		"a stated length longer than the stream":   a + " blob 30\nabc\n",
		"a negative length":                        a + " blob -3\nabc\n",
		"a signed length":                          a + " blob +3\nabc\n",
		"a length with a leading zero":             a + " blob 03\nabc\n",
		"a length with trailing text":              a + " blob 3x\nabc\n",
		"a length beyond an int64":                 a + " blob 99999999999999999999\nabc\n",
		"no length":                                a + " blob\nabc\n",
		"a field too many":                         a + " blob 3 3\nabc\n",
		"an unknown type":                          a + " blub 3\nabc\n",
		"another object's name":                    b + " blob 3\nabc\n",
		"an empty header":                          "\n",
		"a missing response with a field too many": a + " missing now\n",
		"a header over its bound":                  a + " blob 3" + strings.Repeat(" ", maxHeaderBytes) + "\nabc\n",
		"a header with no terminator in reach":     strings.Repeat("x", responseBuffer+1),
	}
	for name, stream := range cases {
		cases[name] = stream + next + next
	}
	// Streams that end inside a record have nothing after them.
	cases["a stream that ends in the header"] = a + " blob"
	cases["a stream that ends in the content"] = a + " blob 10\nabc"
	cases["a stream that ends before the terminator"] = a + " blob 3\nabc"

	for name, stream := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			responses := bufio.NewReaderSize(strings.NewReader(stream), responseBuffer)
			objects := &Objects{requests: discardRequests{}, responses: responses, limit: 1 << 20}

			got, err := objects.Read(a)
			if core.ClassOf(err) != core.ClassInternal {
				t.Fatalf("the forged record came back as %+v, %v; want an internal error", got, err)
			}
			buffered := responses.Buffered()
			for i := 0; i < 2; i++ {
				again, againErr := objects.Read(b)
				if againErr != err {
					t.Errorf("read %d after the failure returned %+v, %v; want the same error, and no record",
						i+1, again, againErr)
				}
			}
			if responses.Buffered() != buffered {
				t.Errorf("reads after the failure took %d more bytes from the stream; want none",
					buffered-responses.Buffered())
			}
		})
	}
}
