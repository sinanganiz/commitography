// The length-framed object reader of ADR-0072: the contents of objects, by
// object name, through one `git cat-file --batch` process kept open for as long
// as its caller reads.
//
// Git offers file contents at the minimum supported version only as
// line-framed patch output, which ADR-0072 clause 3 forbids, or as this
// length-prefixed stream. Each response is
//
//	<object name> SP <type> SP <size> LF <size bytes of content> LF
//
// or `<object name> SP missing LF` for an object the repository lacks. The
// header carries nothing a repository controls: an object name, a type word and
// a decimal size (ADR-0072 clause 2). The content is taken by its stated
// length and never searched, so no content can end a record early or start one
// (clause 1). Within a record the caller may split the content into lines,
// because no framing decision depends on that split (clause 4).
//
// A header that does not parse, a name that is not the one asked for, or a
// byte after the content that is not the terminating LF aborts the read with an
// internal error, and every later read returns that error without reading
// anything (clause 6). The reader never looks for the next plausible header:
// resynchronising on a forged boundary is the failure the record exists to
// prevent.
//
// A record larger than the reader's cap is skipped by its length, without being
// held, and reported as oversized; the stream stays in frame and the run goes
// on (clause 5). The caller marks the file degraded.

package git

import (
	"bufio"
	"context"
	"errors"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/sinanganiz/commitography/internal/core"
)

const (
	// ObjectReadTimeout bounds one object reader. Replay keeps its reader open
	// for the whole walk and reads every version of every file through it, so
	// the bound is on the walk rather than on one object. Replaying the
	// designated large fixture takes seconds; this stops a hung process
	// without interrupting any walk the product is known to make
	// (ADR-0044 clause 3).
	ObjectReadTimeout = time.Hour

	// maxHeaderBytes bounds a response header. The longest well-formed one is
	// a 64-digit SHA-256 name, the longest type word and a 19-digit size, which
	// is under 100 bytes.
	maxHeaderBytes = 256

	// responseBuffer is the read buffer in front of the process's output.
	responseBuffer = 64 << 10
)

// Object is one response of the object reader.
type Object struct {
	// Type is git's object type: blob, tree, commit or tag. Empty when Missing.
	Type string
	// Size is the content's length in bytes, as the header stated it.
	Size int64
	// Content is the object's content, a fresh slice the caller may keep. It
	// is nil when the object is over the cap or missing.
	Content []byte
	// Oversized marks an object larger than the reader's cap. Its content was
	// skipped by its length and not held (ADR-0072 clause 5).
	Oversized bool
	// Missing marks an object the repository does not contain.
	Missing bool
}

// Objects reads object contents by name. It is not safe for concurrent use: a
// request and its response are one step of one stream.
type Objects struct {
	process   *Process
	requests  io.WriteCloser
	responses *bufio.Reader
	limit     int64

	// failed, once set, is returned by every later read. A framing failure
	// leaves the stream at a position nothing can vouch for, so nothing
	// further is read from it (ADR-0072 clause 6).
	failed error
}

// OpenObjects starts an object reader against repo whose records are capped at
// limit bytes, the single-file-size limit of ADR-0048. The caller must Close
// it.
func OpenObjects(ctx context.Context, repo string, limit int64) (*Objects, error) {
	return openObjects(ctx, At(repo), limit)
}

// openObjects is OpenObjects over a prepared Spec, so this package's own
// argument-vector checker can drive it the way every other entry point is
// driven.
func openObjects(ctx context.Context, base Spec, limit int64) (*Objects, error) {
	if limit <= 0 {
		return nil, core.Internalf(nil, "opening an object reader with a cap of %d bytes", limit)
	}
	// Requests go through a pipe rather than a prepared reader, because the
	// next object asked for depends on what the last one contained. It is an
	// operating-system pipe handed to git as its input, not an in-process one
	// copied across by a goroutine: a write to a git that has exited then fails
	// instead of waiting for a copier that has stopped.
	requests, feed, err := os.Pipe()
	if err != nil {
		return nil, core.Internalf(err, "creating the object reader's request pipe")
	}
	base.Args = append(append([]string(nil), base.Args...), "cat-file", "--batch")
	spec := base.WithStdin(requests).WithTimeout(ObjectReadTimeout)
	process, err := Start(ctx, spec)
	// The child holds its own end now; this process keeps only the write end.
	_ = requests.Close()
	if err != nil {
		_ = feed.Close()
		return nil, err
	}
	return &Objects{
		process:   process,
		requests:  feed,
		responses: bufio.NewReaderSize(process.stdout, responseBuffer),
		limit:     limit,
	}, nil
}

// Read returns the object with the given name. A framing failure is an
// internal error, and so is every read after one.
func (o *Objects) Read(name string) (Object, error) {
	if o.failed != nil {
		return Object{}, o.failed
	}
	if !isObjectName(name) {
		// A name is a request line of its own; one holding a space or a line
		// terminator would be a different request. The caller passes names git
		// produced, so this is a defect, and nothing was sent.
		return Object{}, core.Internalf(nil, "asking the object reader for %q, which is not an object name", name)
	}
	if _, err := io.WriteString(o.requests, name+"\n"); err != nil {
		return Object{}, o.fail(o.streamError(err, "asking for object %s", name))
	}
	object, err := readObject(o.responses, name, o.limit)
	if err != nil {
		return Object{}, o.fail(o.streamError(err, "reading object %s", name))
	}
	return object, nil
}

// fail records a failure as the reader's last word.
func (o *Objects) fail(err error) error {
	o.failed = err
	return err
}

// streamError prefers the context's own error to whatever the broken stream
// said: a cancelled read is that, not a framing failure.
func (o *Objects) streamError(err error, format string, args ...any) error {
	if o.process != nil {
		if ctxErr := o.process.ctxErr(); ctxErr != nil {
			return ctxErr
		}
	}
	var internal *core.InternalError
	if errors.As(err, &internal) {
		return err
	}
	return core.Internalf(err, format, args...)
}

// Close ends the read. A reader that is still in frame closes git's input and
// waits for it to finish; one that failed has its process terminated, so that
// nothing more is read from a stream no header can be trusted in.
func (o *Objects) Close() error {
	if o.process == nil {
		return nil
	}
	_ = o.requests.Close()
	if o.failed != nil {
		o.process.Close()
		return nil
	}
	err := o.process.Wait()
	o.process.Close()
	return err
}

// readObject reads one response to a request for name. It reads exactly the
// header, the stated length and the terminator, or fails.
func readObject(r *bufio.Reader, name string, limit int64) (Object, error) {
	header, err := r.ReadSlice('\n')
	if err != nil {
		if errors.Is(err, bufio.ErrBufferFull) {
			return Object{}, core.Internalf(nil, "object %s: the response header has no terminator within %d bytes",
				name, responseBuffer)
		}
		return Object{}, core.Internalf(err, "object %s: reading the response header", name)
	}
	// The header's own bytes are never quoted in a message: once framing has
	// failed they may be file content, which may hold an address (ADR-0041
	// clause 5).
	if len(header) > maxHeaderBytes {
		return Object{}, core.Internalf(nil, "object %s: the response header is %d bytes, over %d",
			name, len(header), maxHeaderBytes)
	}
	fields := strings.Split(strings.TrimSuffix(string(header), "\n"), " ")
	if len(fields) == 2 && fields[0] == name && fields[1] == "missing" {
		return Object{Missing: true}, nil
	}
	if len(fields) != 3 || fields[0] != name || !isObjectType(fields[1]) {
		return Object{}, core.Internalf(nil, "object %s: the response header does not parse as a header for it", name)
	}
	size, ok := parseSize(fields[2])
	if !ok {
		return Object{}, core.Internalf(nil, "object %s: the response header's size does not parse", name)
	}

	object := Object{Type: fields[1], Size: size}
	if size > limit {
		if _, err := io.CopyN(io.Discard, r, size); err != nil {
			return Object{}, core.Internalf(err, "object %s: skipping %d bytes of content over the cap", name, size)
		}
		object.Oversized = true
	} else {
		object.Content = make([]byte, size)
		if _, err := io.ReadFull(r, object.Content); err != nil {
			return Object{}, core.Internalf(err, "object %s: reading %d bytes of content", name, size)
		}
	}
	terminator, err := r.ReadByte()
	if err != nil {
		return Object{}, core.Internalf(err, "object %s: reading the terminator after the content", name)
	}
	if terminator != '\n' {
		return Object{}, core.Internalf(nil, "object %s: the content is not followed by its terminator, so its "+
			"stated length is not its length", name)
	}
	return object, nil
}

// parseSize reads a size in the one form git writes it: decimal digits with no
// sign and no leading zero, within an int64.
func parseSize(s string) (int64, bool) {
	if s == "" || (len(s) > 1 && s[0] == '0') {
		return 0, false
	}
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return 0, false
		}
	}
	size, err := strconv.ParseInt(s, 10, 64)
	return size, err == nil
}

// isObjectType reports whether s is one of git's four object types.
func isObjectType(s string) bool {
	switch s {
	case "blob", "tree", "commit", "tag":
		return true
	}
	return false
}

// isObjectName reports whether s is a full object name in git's form: 40
// lowercase hexadecimal digits for SHA-1, 64 for SHA-256.
func isObjectName(s string) bool {
	if len(s) != 40 && len(s) != 64 {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			return false
		}
	}
	return true
}
