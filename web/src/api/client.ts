// Mirrors the Phase 1.5 local API contract in docs/phase-1.5/m0-contracts.md.
// The report body itself is typed by ../types and is not duplicated here.

export type JobStatusValue = 'queued' | 'running' | 'succeeded' | 'failed' | 'cancelled' | 'stale';

export interface Capabilities {
  apiVersion: string;
  reportSchemaVersion: number;
  maxRecentJobs: number;
  activeJobLimit: number;
  pollIntervalMilliseconds: number;
  supportsCancel: boolean;
  supportsOpen: boolean;
}

export interface JobSummary {
  id: string;
  status: JobStatusValue;
  repoName: string;
  createdAt: string;
  startedAt: string | null;
  finishedAt: string | null;
  warningCount: number;
}

/** One observable point in an analysis. `fraction` is null when unknown. */
export interface ProgressEvent {
  sequence: number;
  stage: string;
  detail: string;
  fraction: number | null;
  current: number;
  total: number;
  estimated: boolean;
}

export interface JobFailure {
  code: string;
  message: string;
}

export interface JobStatus {
  id: string;
  status: JobStatusValue;
  repoName: string;
  repoPath: string;
  createdAt: string;
  startedAt: string | null;
  finishedAt: string | null;
  elapsedMilliseconds: number;
  progress: ProgressEvent | null;
  warnings: string[];
  error: JobFailure | null;
}

export interface AnalysisOptions {
  noBlame: boolean;
  perAuthor: boolean;
  anonymize: boolean;
  allowShallow: boolean;
  countMerges: boolean;
  since: string;
  until: string;
}

export interface CreateJobRequest {
  repoPath: string;
  options: AnalysisOptions;
}

export interface CreateJobResponse {
  id: string;
  status: JobStatusValue;
}

/** A failed API call. `status` is 0 when the server could not be reached. */
export class ApiError extends Error {
  readonly status: number;
  readonly code: string;

  constructor(status: number, code: string, message: string) {
    super(message);
    this.name = 'ApiError';
    this.status = status;
    this.code = code;
  }
}

const BASE = '/api/v1';

export const ACTIVE_STATUSES: readonly JobStatusValue[] = ['queued', 'running'];

export function isActive(status: JobStatusValue): boolean {
  return ACTIVE_STATUSES.includes(status);
}

/** Negotiates capabilities; this request also issues the local session cookie. */
export function getCapabilities(): Promise<Capabilities> {
  return request<Capabilities>('GET', '/capabilities');
}

export async function listJobs(): Promise<JobSummary[]> {
  const body = await request<{ jobs: JobSummary[] }>('GET', '/jobs');
  return body.jobs;
}

export function createJob(input: CreateJobRequest): Promise<CreateJobResponse> {
  return request<CreateJobResponse>('POST', '/jobs', input);
}

export function getJob(id: string): Promise<JobStatus> {
  return request<JobStatus>('GET', `/jobs/${encodeURIComponent(id)}`);
}

/** Requests cooperative cancellation. Repeating it for a cancelled job is safe. */
export function cancelJob(id: string): Promise<JobStatus> {
  return request<JobStatus>('POST', `/jobs/${encodeURIComponent(id)}/cancel`);
}

async function request<T>(method: string, path: string, body?: unknown): Promise<T> {
  let response: Response;
  try {
    response = await fetch(BASE + path, {
      method,
      credentials: 'same-origin',
      cache: 'no-store',
      headers: body === undefined ? undefined : { 'Content-Type': 'application/json' },
      body: body === undefined ? undefined : JSON.stringify(body),
    });
  } catch {
    throw new ApiError(0, 'network_error', 'The local server could not be reached.');
  }

  const payload: unknown = await response.json().catch(() => null);
  if (!response.ok) {
    const error = (payload as { error?: { code?: unknown; message?: unknown } } | null)?.error;
    throw new ApiError(
      response.status,
      typeof error?.code === 'string' ? error.code : 'http_error',
      typeof error?.message === 'string' ? error.message : `The server responded with ${response.status}.`,
    );
  }
  return payload as T;
}
