import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';

import type { JobStatus } from '../api/client';
import { jsonResponse, sampleReport } from '../test/sampleReport';
import { JobView } from './JobView';

afterEach(cleanup);

const ID = '0123456789abcdef';
const STATUS_URL = `/api/v1/jobs/${ID}`;

function status(overrides: Partial<JobStatus> = {}): JobStatus {
  return {
    id: ID,
    status: 'running',
    repoName: 'api',
    repoPath: '/repos/api',
    createdAt: '2026-09-13T10:00:00Z',
    startedAt: '2026-09-13T10:00:01Z',
    finishedAt: null,
    elapsedMilliseconds: 1500,
    progress: {
      sequence: 3,
      stage: 'collecting',
      detail: '420 of 1000 commits',
      fraction: 0.42,
      current: 420,
      total: 1000,
      estimated: true,
    },
    warnings: [],
    error: null,
    ...overrides,
  };
}

type Handler = (init?: RequestInit) => Response | Promise<Response>;

/** Routes fetch calls by "METHOD url"; each handler returns a fresh response. */
function mockApi(routes: Record<string, Handler>) {
  const fetchMock = vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
    const handler = routes[`${init?.method ?? 'GET'} ${String(input)}`];
    return handler ? handler(init) : jsonResponse({ error: { code: 'not_found', message: 'job not found' } }, 404);
  });
  vi.stubGlobal('fetch', fetchMock);
  const calls = (method: string, url: string) =>
    fetchMock.mock.calls.filter(([input, init]) => String(input) === url && (init?.method ?? 'GET') === method).length;
  return { fetchMock, calls };
}

/** Serves each status in turn, repeating the last one. */
function sequence(...statuses: JobStatus[]): Handler {
  let index = 0;
  return () => jsonResponse(statuses[Math.min(index++, statuses.length - 1)]);
}

function renderJob() {
  return render(
    <JobView id={ID} pollIntervalMs={5} themeMode="dark" onSettled={() => {}} onStartNew={() => {}} onOpenRecent={() => {}} />,
  );
}

const sleep = (ms: number) => new Promise((resolve) => setTimeout(resolve, ms));

describe('JobView progress', () => {
  it('labels an estimated percentage and names the stage', async () => {
    mockApi({ [`GET ${STATUS_URL}`]: sequence(status()) });
    renderJob();
    const bar = await screen.findByRole('progressbar', { name: 'Analysis progress' });
    expect(bar.getAttribute('aria-valuetext')).toBe('About 42 percent, estimated');
    expect(screen.getByText('estimated')).toBeTruthy();
    expect(screen.getAllByText('Reading history').length).toBeGreaterThan(0);
    expect(screen.getByText('420 of 1000 commits.')).toBeTruthy();
  });

  it('keeps an indeterminate stage visibly active', async () => {
    mockApi({
      [`GET ${STATUS_URL}`]: sequence(status({ progress: { ...status().progress!, stage: 'code', detail: 'code', fraction: null } })),
    });
    renderJob();
    const bar = await screen.findByRole('progressbar', { name: 'Analysis progress' });
    expect(bar.getAttribute('aria-valuenow')).toBeNull();
    expect(bar.getAttribute('aria-valuetext')).toBe('Progress unknown for this stage');
    expect(screen.queryByText('Code.')).toBeNull();
  });

  it('stops polling once the job succeeds and shows the report', async () => {
    const { calls } = mockApi({
      [`GET ${STATUS_URL}`]: sequence(status(), status({ status: 'succeeded', finishedAt: '2026-09-13T10:00:09Z' })),
      [`GET ${STATUS_URL}/report`]: () => jsonResponse(sampleReport()),
    });
    renderJob();
    await screen.findByText('Analysis complete', { selector: '.MuiAlertTitle-root' });
    await waitFor(() => expect(document.querySelector('section#pulse')).not.toBeNull());
    const settled = calls('GET', STATUS_URL);
    await sleep(80);
    expect(calls('GET', STATUS_URL)).toBe(settled);
    expect(document.querySelector('[role="status"][aria-live="polite"]')?.textContent).toBe('Analysis complete');
  });

  it('cancels without a reload and reports the cancelled state', async () => {
    let cancelled = false;
    const { calls } = mockApi({
      [`GET ${STATUS_URL}`]: () => jsonResponse(cancelled ? status({ status: 'cancelled' }) : status()),
      [`POST ${STATUS_URL}/cancel`]: () => {
        cancelled = true;
        return jsonResponse(status(), 202);
      },
    });
    renderJob();
    fireEvent.click(await screen.findByRole('button', { name: 'Cancel analysis' }));
    await screen.findByText('Analysis cancelled', { selector: '.MuiAlertTitle-root' });
    expect(calls('POST', `${STATUS_URL}/cancel`)).toBe(1);
  });
});

describe('JobView terminal states', () => {
  it('tells a rejected request from a failed analysis', async () => {
    mockApi({ [`GET ${STATUS_URL}`]: sequence(status({ status: 'failed', error: { code: 'invalid_analysis_request', message: '' } })) });
    renderJob();
    await screen.findByText('The analysis request was rejected');
    cleanup();

    mockApi({ [`GET ${STATUS_URL}`]: sequence(status({ status: 'failed', error: { code: 'analysis_failed', message: '' } })) });
    renderJob();
    await screen.findByText('The analysis failed');
  });

  it('never loads a report for a stale job', async () => {
    const { calls } = mockApi({
      [`GET ${STATUS_URL}`]: sequence(
        status({ status: 'stale', error: { code: 'repository_changed', message: 'repository HEAD changed during analysis' } }),
      ),
    });
    renderJob();
    await screen.findByText('The repository changed during the analysis');
    await sleep(50);
    expect(calls('GET', `${STATUS_URL}/report`)).toBe(0);
    expect(document.querySelector('.cg-report')).toBeNull();
  });

  it('offers a retry that only reads status when the server is lost', async () => {
    const fetchMock = vi.fn(async (_input: RequestInfo | URL, _init?: RequestInit): Promise<Response> => {
      throw new TypeError('Failed to fetch');
    });
    vi.stubGlobal('fetch', fetchMock);
    renderJob();
    const retry = await screen.findByRole('button', { name: 'Retry' });
    expect(screen.getByText('Lost contact with the local server')).toBeTruthy();
    const before = fetchMock.mock.calls.length;
    fireEvent.click(retry);
    await waitFor(() => expect(fetchMock.mock.calls.length).toBeGreaterThan(before));
    expect(fetchMock.mock.calls.every(([, init]) => ((init as RequestInit | undefined)?.method ?? 'GET') === 'GET')).toBe(true);
  });

  it('explains a job the server no longer has', async () => {
    mockApi({});
    renderJob();
    await screen.findByText('This job is no longer available.');
  });
});
