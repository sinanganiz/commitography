import { useEffect, useRef, useState } from 'react';
import type { ReactElement } from 'react';
import Alert from '@mui/material/Alert';
import AlertTitle from '@mui/material/AlertTitle';
import Box from '@mui/material/Box';
import Button from '@mui/material/Button';
import Chip from '@mui/material/Chip';
import LinearProgress from '@mui/material/LinearProgress';
import Paper from '@mui/material/Paper';
import Stack from '@mui/material/Stack';
import Typography from '@mui/material/Typography';

import { ApiError, deleteJob, getJob, isActive } from '../api/client';
import type { JobFailure, JobStatusValue, JobSummary } from '../api/client';
import { visuallyHidden } from './a11y';
import { formatDateTime, formatDuration, plural } from './format';
import { outcomeSummary, statusChip } from './outcomes';
import { routeHash } from './routes';

interface RecentJobsProps {
  jobs: JobSummary[];
  session: 'connecting' | 'ready' | 'failed';
  maxJobs: number;
  /** A reconnect found none of the jobs this page knew: the server restarted. */
  restarted: boolean;
  onRefresh: () => Promise<void>;
  onReconnect: () => void;
  onStartNew: () => void;
}

const summaryColor: Record<JobStatusValue, string> = {
  queued: 'text.primary',
  running: 'text.primary',
  succeeded: 'success.main',
  failed: 'error.main',
  cancelled: 'text.secondary',
  stale: 'warning.main',
};

/** The in-memory history of the local server, newest first. */
export function RecentJobs({
  jobs,
  session,
  maxJobs,
  restarted,
  onRefresh,
  onReconnect,
  onStartNew,
}: RecentJobsProps): ReactElement {
  const headingRef = useRef<HTMLHeadingElement>(null);
  // Failure details by job ID. The list response carries no error code, and a
  // terminal job never changes, so each failed or stale job is read once.
  const [failures, setFailures] = useState<Record<string, JobFailure | null>>({});
  const [confirming, setConfirming] = useState<string | null>(null);
  const [removing, setRemoving] = useState<string | null>(null);
  const [removeError, setRemoveError] = useState<{ id: string; message: string } | null>(null);
  const [announcement, setAnnouncement] = useState('');

  useEffect(() => {
    const missing = jobs.filter((job) => (job.status === 'failed' || job.status === 'stale') && !(job.id in failures));
    if (missing.length === 0) return;
    let stopped = false;
    Promise.all(
      missing.map((job) =>
        getJob(job.id)
          .then((status) => [job.id, status.error] as const)
          .catch(() => [job.id, null] as const),
      ),
    ).then((entries) => {
      if (!stopped) setFailures((current) => ({ ...current, ...Object.fromEntries(entries) }));
    });
    return () => {
      stopped = true;
    };
  }, [jobs, failures]);

  const remove = async (job: JobSummary) => {
    setRemoving(job.id);
    setRemoveError(null);
    try {
      await deleteJob(job.id);
    } catch (caught) {
      // A job that is already gone has reached the state the user asked for.
      if (!(caught instanceof ApiError && caught.status === 404)) {
        setRemoveError({
          id: job.id,
          message:
            caught instanceof ApiError && caught.status === 409
              ? 'This job is still running. Cancel it before removing it.'
              : 'The job could not be removed. Check that commitography serve is still running, then try again.',
        });
        setRemoving(null);
        return;
      }
    }
    setConfirming(null);
    setRemoving(null);
    setAnnouncement(`Removed ${job.repoName || 'the job'} from recent jobs.`);
    await onRefresh();
    // The removed row held focus; the heading is the nearest stable place.
    headingRef.current?.focus();
  };

  const nextEvicted = oldestFinished(jobs, maxJobs);
  const title =
    session === 'failed'
      ? 'Recent jobs are unavailable'
      : session === 'ready' && jobs.length > 0
        ? `${jobs.length} of ${maxJobs} kept in memory`
        : 'No analyses yet';

  return (
    <Paper component="main" sx={{ p: { xs: 2.5, md: 5 }, borderRadius: 3 }}>
      <Box role="status" aria-live="polite" sx={visuallyHidden}>
        {announcement}
      </Box>
      <Stack spacing={3}>
        <Stack spacing={1}>
          <Typography variant="overline" color="primary">
            Recent jobs
          </Typography>
          <Typography ref={headingRef} tabIndex={-1} variant="h3" component="h2" sx={{ outline: 'none' }}>
            {title}
          </Typography>
          <Typography color="text.secondary" sx={{ maxWidth: 720 }}>
            The {maxJobs} most recent jobs stay here while commitography serve runs. When the list is full and another
            analysis finishes, the oldest finished job is removed. Restarting the server clears the list.
          </Typography>
        </Stack>

        {restarted ? (
          <Alert severity="info">
            <AlertTitle>The local server was restarted</AlertTitle>
            Jobs from before the restart were kept only in its memory, so they and their reports are gone. Run an
            analysis again to see a repository.
          </Alert>
        ) : null}

        {session === 'connecting' ? <LinearProgress aria-label="Loading recent jobs" /> : null}

        {session === 'failed' ? (
          <Alert
            severity="warning"
            action={
              <Button color="inherit" size="small" sx={{ whiteSpace: 'nowrap' }} onClick={onReconnect}>
                Reconnect
              </Button>
            }
          >
            <AlertTitle>The job list could not be loaded</AlertTitle>
            The local session was lost, usually because commitography serve stopped or restarted. A restarted server
            starts with an empty history.
          </Alert>
        ) : null}

        {session === 'ready' && jobs.length === 0 ? (
          <Stack spacing={2} sx={{ alignItems: 'flex-start' }}>
            <Typography color="text.secondary">
              Nothing has been analyzed in this session. Each analysis you start appears here with its outcome, and a
              finished report can be reopened from this list.
            </Typography>
            <Button variant="contained" onClick={onStartNew}>
              Start an analysis
            </Button>
          </Stack>
        ) : null}

        {session === 'ready' && jobs.length > 0 ? (
          <Stack component="ul" spacing={1.5} sx={{ listStyle: 'none', m: 0, p: 0 }}>
            {jobs.map((job) => (
              <JobRow
                key={job.id}
                job={job}
                failure={failures[job.id]}
                nextEvicted={job.id === nextEvicted}
                confirming={confirming === job.id}
                removing={removing === job.id}
                removeError={removeError?.id === job.id ? removeError.message : null}
                onConfirm={() => {
                  setRemoveError(null);
                  setConfirming(job.id);
                }}
                onKeep={() => setConfirming(null)}
                onRemove={() => void remove(job)}
              />
            ))}
          </Stack>
        ) : null}
      </Stack>
    </Paper>
  );
}

/** The job the server evicts next: the oldest finished one, once the list is full. */
function oldestFinished(jobs: JobSummary[], maxJobs: number): string | null {
  if (jobs.length < maxJobs) return null;
  let oldest: JobSummary | null = null;
  for (const job of jobs) {
    if (isActive(job.status)) continue;
    if (!oldest || Date.parse(job.createdAt) < Date.parse(oldest.createdAt)) oldest = job;
  }
  return oldest?.id ?? null;
}

interface JobRowProps {
  job: JobSummary;
  failure: JobFailure | null | undefined;
  nextEvicted: boolean;
  confirming: boolean;
  removing: boolean;
  removeError: string | null;
  onConfirm: () => void;
  onKeep: () => void;
  onRemove: () => void;
}

function JobRow({
  job,
  failure,
  nextEvicted,
  confirming,
  removing,
  removeError,
  onConfirm,
  onKeep,
  onRemove,
}: JobRowProps): ReactElement {
  const removeButton = useRef<HTMLButtonElement>(null);
  const restoreFocus = useRef(false);
  const active = isActive(job.status);
  const name = job.repoName || 'Repository';
  const titleId = `job-${job.id}-title`;
  const started = job.startedAt ?? job.createdAt;
  const duration = job.finishedAt ? formatDuration(Date.parse(job.finishedAt) - Date.parse(started)) : null;

  // Keeping a job returns focus to the Remove button that opened the question.
  useEffect(() => {
    if (!confirming && restoreFocus.current) {
      restoreFocus.current = false;
      removeButton.current?.focus();
    }
  }, [confirming]);

  const open =
    job.status === 'succeeded'
      ? { label: 'Open report', variant: 'contained' as const }
      : active
        ? { label: 'View progress', variant: 'contained' as const }
        : { label: 'View details', variant: 'outlined' as const };

  return (
    <Paper component="li" variant="outlined" aria-labelledby={titleId} sx={{ p: { xs: 2, md: 2.5 }, borderRadius: 2 }}>
      <Stack
        direction={{ xs: 'column', md: 'row' }}
        spacing={2}
        sx={{ justifyContent: 'space-between', alignItems: { md: 'center' } }}
      >
        <Stack spacing={0.75} sx={{ minWidth: 0 }}>
          <Stack direction="row" spacing={1} sx={{ alignItems: 'center', flexWrap: 'wrap', rowGap: 0.5 }}>
            <Typography id={titleId} variant="h6" component="h3" sx={{ overflowWrap: 'anywhere' }}>
              {name}
            </Typography>
            <Chip size="small" label={statusChip[job.status].label} color={statusChip[job.status].color} />
            {nextEvicted ? <Chip size="small" variant="outlined" label="Next to be removed" /> : null}
          </Stack>
          <Typography variant="body2" sx={{ color: summaryColor[job.status] }}>
            {outcomeSummary(job.status, failure)}
          </Typography>
          <Typography variant="body2" color="text.secondary">
            Started <time dateTime={started}>{formatDateTime(started)}</time>
            {duration ? ` · took ${duration}` : active ? ' · still running' : ''}
            {job.warningCount > 0 ? ` · ${plural(job.warningCount, 'warning')}` : ''}
          </Typography>
          {nextEvicted ? (
            <Typography variant="caption" color="text.secondary">
              This is the oldest finished job, so it is removed when the next analysis finishes.
            </Typography>
          ) : null}
        </Stack>

        <Stack direction="row" spacing={1} sx={{ flexShrink: 0, flexWrap: 'wrap', rowGap: 1, alignItems: 'center' }}>
          {confirming ? (
            <>
              <Typography variant="body2" sx={{ mr: 0.5 }}>
                Remove from history?
              </Typography>
              <Button variant="contained" color="error" size="small" onClick={onRemove} disabled={removing}>
                {removing ? 'Removing…' : 'Remove'}
              </Button>
              <Button
                color="inherit"
                size="small"
                autoFocus
                disabled={removing}
                onClick={() => {
                  restoreFocus.current = true;
                  onKeep();
                }}
              >
                Keep
              </Button>
            </>
          ) : (
            <>
              <Button variant={open.variant} href={routeHash({ name: 'job', id: job.id })} aria-label={`${open.label}: ${name}`}>
                {open.label}
              </Button>
              {!active ? (
                <Button ref={removeButton} color="inherit" onClick={onConfirm} aria-label={`Remove ${name} from history`}>
                  Remove
                </Button>
              ) : null}
            </>
          )}
        </Stack>
      </Stack>
      {removeError ? (
        <Alert severity="error" sx={{ mt: 2 }}>
          {removeError}
        </Alert>
      ) : null}
    </Paper>
  );
}
