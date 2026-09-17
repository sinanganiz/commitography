// The job manager holds the bounded in-memory lifecycle state used by the
// local web runner. It deliberately does not serve HTTP.

package server

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"sort"
	"sync"
	"time"

	"github.com/sinanganiz/commitography/internal/core"
	"github.com/sinanganiz/commitography/internal/pipeline"
)

// Status is the externally meaningful state of an analysis job.
type Status string

const (
	StatusQueued    Status = "queued"
	StatusRunning   Status = "running"
	StatusSucceeded Status = "succeeded"
	StatusFailed    Status = "failed"
	StatusCancelled Status = "cancelled"
	StatusStale     Status = "stale"

	maxRecentJobs = 10
	// maxJobWarnings bounds the diagnostics retained per job so a noisy
	// repository cannot grow process memory without limit.
	maxJobWarnings = 100
)

var (
	// ErrActiveJob means another job currently owns the single worker slot.
	ErrActiveJob = errors.New("another analysis job is already active")
	// ErrJobNotFound means the requested ID is not retained by the manager.
	ErrJobNotFound = errors.New("analysis job not found")
	// ErrInvalidState means the requested transition is not valid for the job.
	ErrInvalidState = errors.New("invalid analysis job state transition")
)

// Failure is a safe, structured terminal error. The manager stores no raw Git
// stderr; the message is an error's artifact rendering, so it is safe wherever
// the status is shown (ADR-0067 clause 2). Reason and Code are the two
// identities apiErrorBody documents, and are empty and set respectively for a
// condition section 13 does not describe.
type Failure struct {
	Code    string      `json:"code"`
	Reason  core.Reason `json:"reason,omitempty"`
	Message string      `json:"message"`
}

// Snapshot is a race-free copy of one job's public state. Report data is kept
// in Result for the report endpoint and is not included in status JSON by
// default adapters.
type Snapshot struct {
	ID           string
	Status       Status
	RepoPath     string
	RepoName     string
	CreatedAt    time.Time
	StartedAt    *time.Time
	FinishedAt   *time.Time
	Elapsed      time.Duration
	Progress     *pipeline.ProgressEvent
	Warnings     []string
	WarningCount int
	Failure      *Failure
	Result       *pipeline.Result
}

// ManagerOptions configures a Manager. Test hooks are intentionally small and
// do not alter production behavior.
type ManagerOptions struct {
	Limit  int
	NewID  func() (string, error)
	Now    func() time.Time
	Runner Runner
	// ToolVersion is passed to every analysis the manager runs.
	ToolVersion string
}

// Runner is the shared analysis operation executed by a worker.
type Runner func(context.Context, pipeline.Options, pipeline.ProgressSink) (*pipeline.Result, error)

// Manager owns at most one active job and a bounded terminal history.
type Manager struct {
	mu       sync.RWMutex
	limit    int
	newID    func() (string, error)
	now      func() time.Time
	runner   Runner
	version  string
	jobs     map[string]*job
	activeID string
}

type job struct {
	snapshot Snapshot
	cancel   context.CancelFunc
}

// NewManager constructs a bounded in-memory job manager.
func NewManager(options ManagerOptions) *Manager {
	limit := options.Limit
	if limit <= 0 {
		limit = maxRecentJobs
	}
	newID := options.NewID
	if newID == nil {
		newID = randomID
	}
	now := options.Now
	if now == nil {
		now = time.Now
	}
	runner := options.Runner
	if runner == nil {
		runner = pipeline.Run
	}
	return &Manager{
		limit:   limit,
		newID:   newID,
		now:     now,
		runner:  runner,
		version: options.ToolVersion,
		jobs:    make(map[string]*job),
	}
}

// Create reserves the single active slot and creates a queued job.
func (m *Manager) Create(repoPath string) (Snapshot, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.activeID != "" {
		return Snapshot{}, ErrActiveJob
	}
	id, err := m.newID()
	if err != nil {
		return Snapshot{}, fmt.Errorf("creating job ID: %w", err)
	}
	if id == "" || m.jobs[id] != nil {
		return Snapshot{}, errors.New("job ID generator returned a duplicate or empty ID")
	}
	now := m.now()
	entry := &job{snapshot: Snapshot{
		ID:        id,
		Status:    StatusQueued,
		RepoPath:  repoPath,
		RepoName:  filepath.Base(filepath.Clean(repoPath)),
		CreatedAt: now,
	}}
	m.jobs[id] = entry
	m.activeID = id
	return cloneSnapshot(entry.snapshot), nil
}

// Start creates a job and runs the configured analysis asynchronously. The
// returned snapshot is the state observed immediately after reservation.
func (m *Manager) Start(repoPath string, options pipeline.Options) (Snapshot, error) {
	snapshot, err := m.Create(repoPath)
	if err != nil {
		return Snapshot{}, err
	}
	ctx, cancel := context.WithCancel(context.Background())
	m.mu.Lock()
	entry := m.jobs[snapshot.ID]
	entry.cancel = cancel
	m.mu.Unlock()

	options.RepoPath = repoPath
	options.ToolVersion = m.version
	callerWarning := options.OnWarning
	options.OnWarning = func(message string) {
		_ = m.AddWarning(snapshot.ID, message)
		if callerWarning != nil {
			callerWarning(message)
		}
	}
	go m.run(snapshot.ID, ctx, options)
	return snapshot, nil
}

func (m *Manager) run(id string, ctx context.Context, options pipeline.Options) {
	if err := m.MarkRunning(id, m.now()); err != nil {
		return
	}
	result, err := m.runner(ctx, options, func(event pipeline.ProgressEvent) {
		_ = m.UpdateProgress(id, event)
	})
	// A requested cancellation wins over whatever the runner returned after it,
	// so a job the user cancelled never ends as succeeded, failed or stale.
	if ctx.Err() != nil || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		_ = m.Cancelled(id, m.now())
		return
	}
	if err != nil {
		_ = m.Fail(id, FailureFromError(err), m.now())
		return
	}
	_ = m.Complete(id, result, m.now())
}

// Cancel requests cancellation of an active job. The job remains running until
// its context-aware Git work exits, then transitions to cancelled.
func (m *Manager) Cancel(id string) error {
	m.mu.RLock()
	entry, ok := m.jobs[id]
	if !ok {
		m.mu.RUnlock()
		return ErrJobNotFound
	}
	if entry.snapshot.Status != StatusQueued && entry.snapshot.Status != StatusRunning {
		m.mu.RUnlock()
		return ErrInvalidState
	}
	cancel := entry.cancel
	m.mu.RUnlock()
	if cancel == nil {
		return ErrInvalidState
	}
	cancel()
	return nil
}

// CancelAll requests cancellation for every active worker. Workers still own
// their terminal transition and must observe the context before shutdown is
// considered complete.
func (m *Manager) CancelAll() {
	m.mu.RLock()
	cancels := make([]context.CancelFunc, 0, 1)
	for _, entry := range m.jobs {
		if (entry.snapshot.Status == StatusQueued || entry.snapshot.Status == StatusRunning) && entry.cancel != nil {
			cancels = append(cancels, entry.cancel)
		}
	}
	m.mu.RUnlock()
	for _, cancel := range cancels {
		cancel()
	}
}

// Cancelled marks a job cancelled after its worker has observed context
// cancellation and exited.
func (m *Manager) Cancelled(id string, finishedAt time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	entry, err := m.lookupLocked(id)
	if err != nil {
		return err
	}
	if entry.snapshot.Status != StatusQueued && entry.snapshot.Status != StatusRunning {
		return ErrInvalidState
	}
	if finishedAt.IsZero() {
		finishedAt = m.now()
	}
	entry.snapshot.Status = StatusCancelled
	entry.snapshot.FinishedAt = timePtr(finishedAt)
	m.activeID = ""
	entry.cancel = nil
	m.evictLocked()
	return nil
}

// MarkRunning transitions a queued job to running.
func (m *Manager) MarkRunning(id string, startedAt time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	entry, err := m.lookupLocked(id)
	if err != nil {
		return err
	}
	if entry.snapshot.Status != StatusQueued || m.activeID != id {
		return ErrInvalidState
	}
	if startedAt.IsZero() {
		startedAt = m.now()
	}
	entry.snapshot.Status = StatusRunning
	entry.snapshot.StartedAt = timePtr(startedAt)
	return nil
}

// UpdateProgress stores the latest progress event for a queued or running job.
func (m *Manager) UpdateProgress(id string, event pipeline.ProgressEvent) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	entry, err := m.lookupLocked(id)
	if err != nil {
		return err
	}
	if entry.snapshot.Status != StatusQueued && entry.snapshot.Status != StatusRunning {
		return ErrInvalidState
	}
	if entry.snapshot.Progress != nil && event.Sequence < entry.snapshot.Progress.Sequence {
		return ErrInvalidState
	}
	copy := event
	entry.snapshot.Progress = &copy
	return nil
}

// AddWarning records a diagnostic raised while a job is queued or running, so
// status polling can show warnings before the report exists.
func (m *Manager) AddWarning(id, message string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	entry, err := m.lookupLocked(id)
	if err != nil {
		return err
	}
	if entry.snapshot.Status != StatusQueued && entry.snapshot.Status != StatusRunning {
		return ErrInvalidState
	}
	appendWarnings(&entry.snapshot, message)
	return nil
}

// appendWarnings adds messages that are not already recorded, up to the
// per-job bound, and keeps WarningCount equal to the retained list.
func appendWarnings(snapshot *Snapshot, messages ...string) {
	for _, message := range messages {
		if len(snapshot.Warnings) >= maxJobWarnings {
			break
		}
		if !slices.Contains(snapshot.Warnings, message) {
			snapshot.Warnings = append(snapshot.Warnings, message)
		}
	}
	snapshot.WarningCount = len(snapshot.Warnings)
}

// Complete stores an analysis result and releases the active slot. Stale
// results are retained for diagnosis but are never reported as current.
func (m *Manager) Complete(id string, result *pipeline.Result, finishedAt time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	entry, err := m.lookupLocked(id)
	if err != nil {
		return err
	}
	if entry.snapshot.Status != StatusQueued && entry.snapshot.Status != StatusRunning {
		return ErrInvalidState
	}
	if finishedAt.IsZero() {
		finishedAt = m.now()
	}
	entry.snapshot.FinishedAt = timePtr(finishedAt)
	entry.snapshot.Result = result
	// Warnings raised during the run are kept and the report's own warnings,
	// such as blame diagnostics, are merged in, so the count never drops.
	if result != nil && result.Report != nil {
		appendWarnings(&entry.snapshot, result.Report.Warnings...)
	}
	if result != nil && result.Stale {
		entry.snapshot.Status = StatusStale
		entry.snapshot.Failure = &Failure{Code: "repository_changed", Message: staleMessage(result.StaleReason)}
	} else {
		entry.snapshot.Status = StatusSucceeded
	}
	m.activeID = ""
	entry.cancel = nil
	m.evictLocked()
	return nil
}

// Fail marks a queued or running job as failed and releases the active slot.
func (m *Manager) Fail(id string, failure Failure, finishedAt time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	entry, err := m.lookupLocked(id)
	if err != nil {
		return err
	}
	if entry.snapshot.Status != StatusQueued && entry.snapshot.Status != StatusRunning {
		return ErrInvalidState
	}
	if finishedAt.IsZero() {
		finishedAt = m.now()
	}
	entry.snapshot.Status = StatusFailed
	entry.snapshot.FinishedAt = timePtr(finishedAt)
	entry.snapshot.Failure = &failure
	m.activeID = ""
	entry.cancel = nil
	m.evictLocked()
	return nil
}

// FailureFromError describes a terminal analysis error for the job status
// contract. The reason code is the condition's one identity and the published
// code is the job status vocabulary, both as errors.go documents them.
//
// A job status is an API response, so the message is the error's artifact
// rendering: no path, no address, no git output (ADR-0067 clause 2). That is
// what the manager's promise to store no raw Git stderr now rests on, rather
// than on each adapter remembering.
func FailureFromError(err error) Failure {
	if err == nil {
		return Failure{Code: "analysis_failed", Message: "analysis failed"}
	}
	return Failure{
		Code:    publishedFailureCode(err),
		Reason:  core.ReasonOf(err),
		Message: core.Artifact(err),
	}
}

// staleMessage passes only the fixed consistency-check reasons: a revalidation
// failure carries the underlying error, which may include Git stderr.
func staleMessage(reason string) string {
	switch reason {
	case pipeline.StaleHeadChanged, pipeline.StaleCheckoutChanged, pipeline.StaleHistoryChanged:
		return reason
	default:
		return pipeline.StaleRevalidationFailed
	}
}

// Report returns the completed report only for a succeeded job. Stale and
// failed jobs retain diagnostics but cannot be rendered as current reports.
func (m *Manager) Report(id string) (*core.Report, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	entry, err := m.lookupLocked(id)
	if err != nil {
		return nil, err
	}
	if entry.snapshot.Status != StatusSucceeded || entry.snapshot.Result == nil || entry.snapshot.Result.Report == nil {
		return nil, ErrInvalidState
	}
	return entry.snapshot.Result.Report, nil
}

// Get returns a snapshot of a retained job.
func (m *Manager) Get(id string) (Snapshot, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	entry, ok := m.jobs[id]
	if !ok {
		return Snapshot{}, ErrJobNotFound
	}
	return m.projectSnapshotLocked(entry), nil
}

// List returns newest jobs first and never exposes internal pointers.
func (m *Manager) List() []Snapshot {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]Snapshot, 0, len(m.jobs))
	for _, entry := range m.jobs {
		out = append(out, m.projectSnapshotLocked(entry))
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.After(out[j].CreatedAt) })
	return out
}

func (m *Manager) projectSnapshotLocked(entry *job) Snapshot {
	out := cloneSnapshot(entry.snapshot)
	end := m.now()
	if out.FinishedAt != nil {
		end = *out.FinishedAt
	}
	start := out.CreatedAt
	if out.StartedAt != nil {
		start = *out.StartedAt
	}
	if end.After(start) {
		out.Elapsed = end.Sub(start)
	}
	return out
}

// Delete removes a terminal job. Active jobs must be cancelled by the worker
// layer before deletion is allowed.
func (m *Manager) Delete(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	entry, err := m.lookupLocked(id)
	if err != nil {
		return err
	}
	if entry.snapshot.Status == StatusQueued || entry.snapshot.Status == StatusRunning {
		return ErrInvalidState
	}
	delete(m.jobs, id)
	return nil
}

func (m *Manager) lookupLocked(id string) (*job, error) {
	entry, ok := m.jobs[id]
	if !ok {
		return nil, ErrJobNotFound
	}
	return entry, nil
}

func (m *Manager) evictLocked() {
	for len(m.jobs) > m.limit {
		var oldest *job
		for _, entry := range m.jobs {
			if entry.snapshot.Status == StatusQueued || entry.snapshot.Status == StatusRunning {
				continue
			}
			if oldest == nil || entry.snapshot.CreatedAt.Before(oldest.snapshot.CreatedAt) {
				oldest = entry
			}
		}
		if oldest == nil {
			return
		}
		delete(m.jobs, oldest.snapshot.ID)
	}
}

func cloneSnapshot(in Snapshot) Snapshot {
	out := in
	if in.StartedAt != nil {
		value := *in.StartedAt
		out.StartedAt = &value
	}
	if in.FinishedAt != nil {
		value := *in.FinishedAt
		out.FinishedAt = &value
	}
	if in.Progress != nil {
		value := *in.Progress
		out.Progress = &value
	}
	if in.Failure != nil {
		value := *in.Failure
		out.Failure = &value
	}
	out.Warnings = slices.Clone(in.Warnings)
	return out
}

func randomID() (string, error) {
	data := make([]byte, 16)
	if _, err := rand.Read(data); err != nil {
		return "", err
	}
	return hex.EncodeToString(data), nil
}

func timePtr(value time.Time) *time.Time { return &value }
