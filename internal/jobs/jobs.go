// Package jobs manages the bounded in-memory lifecycle state used by the local
// web runner. It deliberately does not start workers or serve HTTP.
package jobs

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/sinanganiz/commitography/internal/analysis"
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
// stderr; adapters decide how much detail is appropriate for their audience.
type Failure struct {
	Code    string `json:"code"`
	Message string `json:"message"`
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
	Progress     *analysis.ProgressEvent
	WarningCount int
	Failure      *Failure
	Result       *analysis.Result
}

// Options configures a Manager. Test hooks are intentionally small and do not
// alter production behavior.
type Options struct {
	Limit int
	NewID func() (string, error)
	Now   func() time.Time
}

// Manager owns at most one active job and a bounded terminal history.
type Manager struct {
	mu       sync.RWMutex
	limit    int
	newID    func() (string, error)
	now      func() time.Time
	jobs     map[string]*job
	activeID string
}

type job struct {
	snapshot Snapshot
}

// New constructs a bounded in-memory job manager.
func New(options Options) *Manager {
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
	return &Manager{
		limit: limit,
		newID: newID,
		now:   now,
		jobs:  make(map[string]*job),
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
func (m *Manager) UpdateProgress(id string, event analysis.ProgressEvent) error {
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

// Complete stores an analysis result and releases the active slot. Stale
// results are retained for diagnosis but are never reported as current.
func (m *Manager) Complete(id string, result *analysis.Result, finishedAt time.Time) error {
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
	if result != nil && result.Report != nil {
		entry.snapshot.WarningCount = len(result.Report.Warnings)
	}
	if result != nil && result.Stale {
		entry.snapshot.Status = StatusStale
		entry.snapshot.Failure = &Failure{Code: "repository_changed", Message: result.StaleReason}
	} else {
		entry.snapshot.Status = StatusSucceeded
	}
	m.activeID = ""
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
	m.evictLocked()
	return nil
}

// Get returns a snapshot of a retained job.
func (m *Manager) Get(id string) (Snapshot, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	entry, ok := m.jobs[id]
	if !ok {
		return Snapshot{}, ErrJobNotFound
	}
	return cloneSnapshot(entry.snapshot), nil
}

// List returns newest jobs first and never exposes internal pointers.
func (m *Manager) List() []Snapshot {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]Snapshot, 0, len(m.jobs))
	for _, entry := range m.jobs {
		out = append(out, cloneSnapshot(entry.snapshot))
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.After(out[j].CreatedAt) })
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
