// The route from the registry into the report: every registered family's
// declared inputs are resolved, the families run concurrently, and each
// section is placed under the namespace its family declares (ADR-0076 clauses
// 3 and 5, ADR-0052 clauses 3, 5 and 6).

package aggregate

import (
	"context"
	"runtime"
	"sync"

	"github.com/sinanganiz/commitography/internal/core"
)

// job is one registered family with its input resolved, ready to run.
type job struct {
	entry    entry
	declared core.FamilyDeclaration
	input    core.Input
	skip     []core.Reason
}

// outcome is what running one job produced.
type outcome struct {
	section  section
	warnings []string
	err      error
}

// route runs every registered family and places its section into families:
// the only route by which a family's output reaches the report. The families
// run at once, up to the stage's degree of parallelism, and nothing they
// produce depends on the order they finish in: sections are placed and
// warnings returned in registry order (ADR-0052 clause 6).
func route(in core.Input, families *core.Families, registered []entry) ([]string, error) {
	ctx := in.Context
	if ctx == nil {
		ctx = context.Background()
	}

	jobs := make([]job, 0, len(registered))
	for _, e := range registered {
		declared := e.family.Declaration()
		if e.section == nil {
			return nil, core.Internalf(nil, "the family %s is registered with no way to produce its section",
				declared.Name)
		}
		input, skip, err := resolve(in, declared)
		if err != nil {
			return nil, err
		}
		jobs = append(jobs, job{entry: e, declared: declared, input: input, skip: skip})
	}

	outcomes := make([]outcome, len(jobs))
	work := make(chan int)
	var wg sync.WaitGroup
	for range min(degree(in), len(jobs)) {
		wg.Go(func() {
			for i := range work {
				j := jobs[i]
				s, warnings, err := j.entry.section(j.input, j.declared, j.skip)
				outcomes[i] = outcome{section: s, warnings: warnings, err: err}
			}
		})
	}
	// Progress is reported from here alone, as each family is handed out, so
	// the caller's callback is never called concurrently. A stage is reported
	// once, where its first family starts.
	started := map[string]bool{}
dispatch:
	for i, j := range jobs {
		if detail := j.entry.progress; detail != "" && !started[detail] {
			started[detail] = true
			progress(in, "metrics", detail, 0, 0)
		}
		select {
		case work <- i:
		case <-ctx.Done():
			break dispatch
		}
	}
	close(work)
	wg.Wait()
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	// Two conditions of the analysis as a whole reach families from here.
	// An author that cannot be resolved into an identity is neither dropped
	// nor split: every family attributing values to identities is degraded
	// instead (ADR-0032, docs/metrics.md section 13). An analysis the operator
	// let proceed on a shallow clone has computed every family over an
	// incomplete history.
	unresolved := unresolvedAuthor(in, in.Analyzed())
	var warnings []string
	for i, j := range jobs {
		o := outcomes[i]
		if o.err != nil {
			return nil, o.err
		}
		if unresolved && j.entry.identities {
			o.section.degrade(core.ReasonUnresolvedIdentity, core.ConfidenceLow)
		}
		if in.Repository.IsShallow {
			o.section.degrade(core.ReasonShallowClone, core.ConfidenceLow)
		}
		if err := o.section.place(families, j.declared.Namespace); err != nil {
			return nil, err
		}
		warnings = append(warnings, o.warnings...)
	}
	return warnings, nil
}

// resolve returns the input a family runs over, holding its declared inputs
// and no others, and the reasons it is skipped for instead of run: one for
// each declared input this analysis cannot provide (ADR-0076 clause 3).
//
// The collect stage's records and the replay stage's state are provided
// whenever the stage runs, so a family declaring them is never skipped for
// them, and replay state missing is a defect of composition rather than a
// condition. No external service is available to a family: the capability
// defaults to unavailable (ADR-0076 clause 8) and nothing configures one.
//
// The input names no repository location, because no input kind is the
// repository (ADR-0076 clause 2). It carries no progress callback, which is
// the stage's own, and a path filter of its own, because a filter's memo is
// not safe for concurrent use.
func resolve(in core.Input, declared core.FamilyDeclaration) (core.Input, []core.Reason, error) {
	out := core.Input{
		Context:    in.Context,
		Repository: in.Repository,
		Config:     in.Config,
		PathFilter: in.PathFilter.Clone(),
	}
	out.Repository.Path = ""
	var skip []core.Reason
	for _, kind := range declared.Inputs {
		switch kind {
		case core.InputCommitRecords:
			out.Filtered, out.Resolver = in.Filtered, in.Resolver
		case core.InputReplayState:
			if in.Replay == nil {
				return core.Input{}, nil, core.Internalf(nil, "aggregating the family %s, which declares replay "+
					"state, without the replay stage's state", declared.Name)
			}
			out.Replay = in.Replay
		case core.InputExternalService:
			skip = append(skip, core.ReasonExternalServiceUnavailable)
		default:
			return core.Input{}, nil, core.Internalf(nil, "the family %s declares the input %q, which is none of "+
				"the three kinds", declared.Name, kind)
		}
	}
	return out, skip, nil
}

// degree returns how many families run at once: the configured degree, or
// the number of available cores where none is configured (ADR-0052 clause 5).
func degree(in core.Input) int {
	if in.Parallelism > 0 {
		return in.Parallelism
	}
	return max(runtime.NumCPU(), 1)
}
