import { useCallback, useEffect, useRef, useState } from 'react';

import { ApiError, getJob, isActive } from '../api/client';
import type { JobStatus } from '../api/client';

/** Consecutive failed polls tolerated before polling pauses for the user. */
const MAX_FAILURES = 3;

export interface JobPoll {
  status: JobStatus | null;
  /** When `status` arrived, from `performance.now()`, for local elapsed time. */
  receivedAt: number;
  /** Set when polling has paused because the server could not be read. */
  error: ApiError | null;
  /** The job is not retained by this server process. */
  notFound: boolean;
  /** Resumes a paused poll. It only reads status and never starts a job. */
  retry: () => void;
  /** Adopts a status returned by another call, such as cancel. */
  accept: (status: JobStatus) => void;
}

/**
 * Polls one job until it reaches a terminal state. Requests never overlap:
 * the next poll is scheduled only after the previous one has settled. Callers
 * key the consuming component by job ID so a new job starts from a clean state.
 */
export function useJobStatus(id: string, intervalMs: number): JobPoll {
  const [status, setStatus] = useState<JobStatus | null>(null);
  const [receivedAt, setReceivedAt] = useState(0);
  const [error, setError] = useState<ApiError | null>(null);
  const [notFound, setNotFound] = useState(false);
  const [generation, setGeneration] = useState(0);
  const latestSequence = useRef(-1);
  const settled = useRef(false);

  const accept = useCallback((next: JobStatus) => {
    // A slow response must not roll progress back behind what is already
    // shown, and nothing may reopen a job that has reached a terminal state.
    const active = isActive(next.status);
    const sequence = next.progress?.sequence ?? -1;
    if (active && (settled.current || sequence < latestSequence.current)) return;
    latestSequence.current = Math.max(latestSequence.current, sequence);
    settled.current = !active;
    setStatus(next);
    setReceivedAt(performance.now());
  }, []);

  useEffect(() => {
    let stopped = false;
    let timer: number | undefined;
    let failures = 0;
    setError(null);
    setNotFound(false);

    const poll = async () => {
      try {
        const next = await getJob(id);
        if (stopped) return;
        failures = 0;
        accept(next);
        if (isActive(next.status) && !settled.current) timer = window.setTimeout(poll, intervalMs);
      } catch (caught) {
        if (stopped) return;
        const failure =
          caught instanceof ApiError ? caught : new ApiError(0, 'network_error', 'The local server could not be read.');
        if (failure.status === 404) {
          setNotFound(true);
          return;
        }
        failures += 1;
        if (failure.status === 401 || failures >= MAX_FAILURES) {
          setError(failure);
          return;
        }
        timer = window.setTimeout(poll, intervalMs * 2 ** failures);
      }
    };
    void poll();

    return () => {
      stopped = true;
      window.clearTimeout(timer);
    };
  }, [id, intervalMs, generation, accept]);

  const retry = useCallback(() => setGeneration((value) => value + 1), []);

  return { status, receivedAt, error, notFound, retry, accept };
}
