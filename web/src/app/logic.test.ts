import { describe, expect, it } from 'vitest';

import { ApiError } from '../api/client';
import type { JobStatus } from '../api/client';
import { countedDetail, formatDuration, sentence, stageLabel } from './format';
import { jobOutcome, outcomeSummary } from './outcomes';
import { parseRoute, routeHash } from './routes';
import { describeStartError } from './startErrors';

function status(overrides: Partial<JobStatus>): JobStatus {
  return {
    id: 'abc',
    status: 'failed',
    repoName: 'api',
    repoPath: '/repos/api',
    createdAt: '2026-09-13T10:00:00Z',
    startedAt: '2026-09-13T10:00:01Z',
    finishedAt: '2026-09-13T10:00:05Z',
    elapsedMilliseconds: 4000,
    progress: null,
    warnings: [],
    error: null,
    ...overrides,
  };
}

describe('start error guidance', () => {
  it('marks path problems and explains container paths', () => {
    const guidance = describeStartError(new ApiError(400, 'invalid_repository_path', 'repository path does not exist'));
    expect(guidance.pathProblem).toBe(true);
    expect(guidance.detail).toContain('/repos/project');
  });

  it('offers to allow a shallow clone', () => {
    expect(describeStartError(new ApiError(400, 'shallow_repository', 'shallow')).action).toBe('allowShallow');
  });

  it('points an active job conflict at the running job', () => {
    const guidance = describeStartError(new ApiError(409, 'active_job', 'another analysis job is already active'));
    expect(guidance.action).toBe('refreshJobs');
    expect(guidance.pathProblem).toBe(false);
  });

  it('asks for a reload when the session is gone or the page failed', () => {
    expect(describeStartError(new ApiError(401, 'invalid_session', 'expired')).action).toBe('reload');
    expect(describeStartError(new Error('boom')).action).toBe('reload');
  });

  it('tells a server failure apart from a rejected request', () => {
    expect(describeStartError(new ApiError(500, 'job_start_failed', 'x')).title).toContain('could not start');
    expect(describeStartError(new ApiError(413, 'request_too_large', 'request body exceeds 1 MiB')).detail).toBe(
      'Request body exceeds 1 MiB.',
    );
  });
});

describe('outcome wording', () => {
  it('distinguishes rejected, failed, cancelled and stale jobs', () => {
    const rejected = outcomeSummary('failed', { code: 'invalid_analysis_request', message: '' });
    const failed = outcomeSummary('failed', { code: 'analysis_failed', message: '' });
    const summaries = [rejected, failed, outcomeSummary('cancelled', null), outcomeSummary('stale', null)];
    expect(new Set(summaries).size).toBe(4);
    expect(rejected).toMatch(/^Rejected/);
    expect(failed).toMatch(/^Failed/);
    expect(outcomeSummary('stale', null)).toContain('No report');
  });

  it('gives each terminal state its own title', () => {
    expect(jobOutcome(status({ status: 'succeeded' })).title).toBe('Analysis complete');
    expect(jobOutcome(status({ status: 'cancelled' })).title).toBe('Analysis cancelled');
    expect(jobOutcome(status({ status: 'stale', error: { code: 'repository_changed', message: 'repository HEAD changed during analysis' } })).title).toBe(
      'The repository changed during the analysis',
    );
    expect(jobOutcome(status({ error: { code: 'invalid_analysis_request', message: '' } })).title).toBe('The analysis request was rejected');
    expect(jobOutcome(status({ error: { code: 'analysis_failed', message: '' } })).title).toBe('The analysis failed');
  });
});

describe('formatting', () => {
  it('shows only counted progress details', () => {
    expect(countedDetail('2512 of 2632 commits')).toBe('2512 of 2632 commits');
    expect(countedDetail('code')).toBe('');
    expect(countedDetail('analysis complete')).toBe('');
  });

  it('formats durations, sentences and stages', () => {
    expect(formatDuration(65_000)).toBe('1:05');
    expect(formatDuration(3_725_000)).toBe('1:02:05');
    expect(formatDuration(-5)).toBe('0:00');
    expect(sentence('repository path does not exist')).toBe('Repository path does not exist.');
    expect(sentence('Done!')).toBe('Done!');
    expect(stageLabel('collecting')).toBe('Reading history');
    expect(stageLabel('future-stage')).toBe('future-stage');
  });
});

describe('routes', () => {
  it('round-trips every route', () => {
    for (const route of [{ name: 'analyze' }, { name: 'recent' }, { name: 'job', id: '0123456789abcdef' }] as const) {
      expect(parseRoute(routeHash(route))).toEqual(route);
    }
  });

  it('falls back to the start view for unknown or unsafe hashes', () => {
    expect(parseRoute('#/jobs/../../etc')).toEqual({ name: 'analyze' });
    expect(parseRoute('#/jobs/<script>')).toEqual({ name: 'analyze' });
    expect(parseRoute('#/elsewhere')).toEqual({ name: 'analyze' });
  });
});
