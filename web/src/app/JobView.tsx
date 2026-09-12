import { useEffect, useRef, useState } from 'react';
import type { ReactElement, ReactNode } from 'react';
import Alert from '@mui/material/Alert';
import AlertTitle from '@mui/material/AlertTitle';
import Box from '@mui/material/Box';
import Button from '@mui/material/Button';
import Chip from '@mui/material/Chip';
import CircularProgress from '@mui/material/CircularProgress';
import Collapse from '@mui/material/Collapse';
import Divider from '@mui/material/Divider';
import LinearProgress from '@mui/material/LinearProgress';
import Paper from '@mui/material/Paper';
import Stack from '@mui/material/Stack';
import Typography from '@mui/material/Typography';
import type { ChipProps } from '@mui/material/Chip';
import type { AlertColor } from '@mui/material/Alert';

import { ApiError, cancelJob, isActive } from '../api/client';
import type { JobStatus, JobStatusValue, ProgressEvent } from '../api/client';
import { countedDetail, formatDuration, plural, sentence, stageLabel } from './format';
import { useJobStatus } from './useJobStatus';

// Pixel strings, not numbers: MUI's sx reads 1 as 100% and -1 as a spacing unit.
const visuallyHidden = {
  position: 'absolute',
  width: '1px',
  height: '1px',
  margin: '-1px',
  padding: 0,
  overflow: 'hidden',
  clip: 'rect(0 0 0 0)',
  whiteSpace: 'nowrap',
  border: 0,
} as const;

const statusChip: Record<JobStatusValue, { label: string; color: ChipProps['color'] }> = {
  queued: { label: 'Queued', color: 'default' },
  running: { label: 'Running', color: 'primary' },
  succeeded: { label: 'Succeeded', color: 'success' },
  failed: { label: 'Failed', color: 'error' },
  cancelled: { label: 'Cancelled', color: 'default' },
  stale: { label: 'Stale', color: 'warning' },
};

type CancelState = 'idle' | 'requesting' | 'requested' | 'failed';

interface StageEntry {
  stage: string;
  detail: string;
}

interface JobViewProps {
  id: string;
  pollIntervalMs: number;
  /** Called once when the job is first seen in a terminal state. */
  onSettled: () => void;
  onStartNew: () => void;
  onOpenRecent: () => void;
}

/** Live progress, cancellation and the terminal outcome of one analysis job. */
export function JobView({ id, pollIntervalMs, onSettled, onStartNew, onOpenRecent }: JobViewProps): ReactElement {
  const { status, receivedAt, error, notFound, retry, accept } = useJobStatus(id, pollIntervalMs);
  const [cancelState, setCancelState] = useState<CancelState>('idle');
  const [showWarnings, setShowWarnings] = useState(false);
  const [stages, setStages] = useState<StageEntry[]>([]);
  const [announcement, setAnnouncement] = useState('');
  const active = status ? isActive(status.status) : true;
  const now = useNow(active && status !== null);
  const settledRef = useRef(false);

  const progress = status?.progress ?? null;

  // Record each stage observed during this visit. Blame can run after the
  // metric stages, so the order shown is the order seen rather than a fixed plan.
  useEffect(() => {
    if (!progress) return;
    setStages((current) => {
      const last = current[current.length - 1];
      if (last && last.stage === progress.stage) {
        return last.detail === progress.detail
          ? current
          : [...current.slice(0, -1), { stage: progress.stage, detail: progress.detail }];
      }
      return [...current.filter((entry) => entry.stage !== progress.stage), { stage: progress.stage, detail: progress.detail }];
    });
  }, [progress?.sequence]);

  // Announce stage changes and the outcome, not every percentage tick.
  const currentStage = progress?.stage ?? '';
  const outcome = status && !isActive(status.status) ? status.status : null;
  useEffect(() => {
    if (outcome) setAnnouncement(outcomeCopy(status!).title);
    else if (currentStage) setAnnouncement(stageLabel(currentStage));
  }, [currentStage, outcome]);

  useEffect(() => {
    if (outcome && !settledRef.current) {
      settledRef.current = true;
      onSettled();
    }
  }, [outcome, onSettled]);

  const cancel = async () => {
    setCancelState('requesting');
    try {
      accept(await cancelJob(id));
      setCancelState('requested');
    } catch (caught) {
      // A conflict means the job finished first; polling reports how.
      setCancelState(caught instanceof ApiError && caught.code === 'invalid_job_state' ? 'idle' : 'failed');
    }
  };

  if (notFound) {
    return <JobNotFound onStartNew={onStartNew} onOpenRecent={onOpenRecent} />;
  }

  const elapsed = status ? status.elapsedMilliseconds + (active ? Math.max(0, now - receivedAt) : 0) : 0;
  const warnings = status?.warnings ?? [];

  return (
    <Paper component="main" sx={{ p: { xs: 2.5, md: 5 }, borderRadius: 3 }}>
      <Box role="status" aria-live="polite" sx={visuallyHidden}>
        {announcement}
      </Box>
      <Stack spacing={3} sx={{ maxWidth: 760 }}>
        <Stack spacing={1}>
          <Stack direction="row" spacing={1.5} sx={{ alignItems: 'center', flexWrap: 'wrap', rowGap: 1 }}>
            <Typography variant="overline" color="primary">
              Analysis
            </Typography>
            {status ? (
              <Chip size="small" label={statusChip[status.status].label} color={statusChip[status.status].color} />
            ) : null}
            <Typography variant="caption" color="text.secondary" sx={{ fontFamily: 'monospace' }}>
              job {id.slice(0, 8)}
            </Typography>
          </Stack>
          <Typography variant="h3" component="h2" sx={{ overflowWrap: 'anywhere' }}>
            {status ? status.repoName || 'Repository' : 'Loading analysis…'}
          </Typography>
          {status?.repoPath ? (
            <Typography variant="body2" color="text.secondary" sx={{ fontFamily: 'monospace', overflowWrap: 'anywhere' }}>
              {status.repoPath}
            </Typography>
          ) : null}
        </Stack>

        {error ? <PollError error={error} onRetry={retry} /> : null}

        {status && active ? (
          <ActiveProgress
            status={status.status}
            progress={progress}
            cancelling={cancelState === 'requested' || cancelState === 'requesting'}
          />
        ) : null}
        {status && !active ? <Outcome status={status} /> : null}
        {!status && !error ? <LinearProgress aria-label="Loading analysis status" /> : null}

        {status ? (
          <Stack direction="row" spacing={3} sx={{ flexWrap: 'wrap', rowGap: 1 }}>
            <Meta label="Elapsed">
              <time dateTime={`PT${Math.floor(elapsed / 1000)}S`}>{formatDuration(elapsed)}</time>
            </Meta>
            <Meta label="Warnings">{warnings.length}</Meta>
          </Stack>
        ) : null}

        {warnings.length > 0 ? (
          <Box>
            <Button
              variant="text"
              color="warning"
              aria-expanded={showWarnings}
              onClick={() => setShowWarnings(!showWarnings)}
              sx={{ px: 1, ml: -1 }}
            >
              {showWarnings ? 'Hide' : 'Show'} {plural(warnings.length, 'warning')}
            </Button>
            <Collapse in={showWarnings}>
              <Box component="ul" sx={{ m: 0, mt: 1, pl: 3, color: 'text.secondary' }}>
                {warnings.map((warning) => (
                  <Typography component="li" variant="body2" key={warning} sx={{ overflowWrap: 'anywhere' }}>
                    {warning}
                  </Typography>
                ))}
              </Box>
            </Collapse>
          </Box>
        ) : null}

        {stages.length > 0 ? <StageLog stages={stages} status={status?.status ?? 'queued'} /> : null}

        <Divider />
        <Stack direction={{ xs: 'column', sm: 'row' }} spacing={1.5} sx={{ alignItems: { sm: 'center' } }}>
          {status && active ? (
            <Button
              variant="outlined"
              color="error"
              onClick={cancel}
              disabled={cancelState === 'requesting' || cancelState === 'requested'}
              startIcon={
                cancelState === 'requesting' || cancelState === 'requested' ? (
                  <CircularProgress size={16} color="inherit" aria-hidden />
                ) : undefined
              }
            >
              {cancelState === 'requesting' || cancelState === 'requested' ? 'Cancelling…' : 'Cancel analysis'}
            </Button>
          ) : (
            <Button variant="contained" onClick={onStartNew}>
              {status?.status === 'succeeded' ? 'Analyze another repository' : 'Start a new analysis'}
            </Button>
          )}
          <Button variant="text" color="inherit" onClick={onOpenRecent}>
            Recent jobs
          </Button>
        </Stack>
        {cancelState === 'failed' ? (
          <Alert severity="error">
            The cancel request did not reach the server. The analysis may still be running; try again.
          </Alert>
        ) : null}
      </Stack>
    </Paper>
  );
}

function ActiveProgress({
  status,
  progress,
  cancelling,
}: {
  status: JobStatusValue;
  progress: ProgressEvent | null;
  cancelling: boolean;
}): ReactElement {
  const fraction = progress?.fraction ?? null;
  const percent = fraction === null ? null : Math.round(Math.min(1, Math.max(0, fraction)) * 100);
  const estimated = progress?.estimated ?? true;
  const title = cancelling
    ? 'Stopping the analysis'
    : status === 'queued' || !progress
      ? 'Waiting to start'
      : stageLabel(progress.stage);
  const valueText =
    percent === null ? 'Progress unknown for this stage' : `${estimated ? 'About ' : ''}${percent} percent${estimated ? ', estimated' : ''}`;

  return (
    <Stack spacing={1.25}>
      <Stack direction="row" spacing={1.5} sx={{ alignItems: 'center', justifyContent: 'space-between' }}>
        <Stack direction="row" spacing={1.25} sx={{ alignItems: 'center', minWidth: 0 }}>
          {/* The spinner keeps a slow stage visibly alive between updates. */}
          <CircularProgress size={18} aria-hidden sx={{ flexShrink: 0 }} />
          <Typography variant="h6" component="p" sx={{ overflowWrap: 'anywhere' }}>
            {title}
          </Typography>
        </Stack>
        {percent !== null ? (
          <Typography sx={{ fontVariantNumeric: 'tabular-nums', whiteSpace: 'nowrap' }} aria-hidden>
            {estimated ? '≈ ' : ''}
            {percent}%
            {estimated ? (
              <Typography component="span" variant="caption" color="text.secondary" sx={{ ml: 0.75 }}>
                estimated
              </Typography>
            ) : null}
          </Typography>
        ) : null}
      </Stack>
      <LinearProgress
        variant={percent === null || cancelling ? 'indeterminate' : 'determinate'}
        value={percent ?? undefined}
        color={cancelling ? 'inherit' : 'primary'}
        aria-label="Analysis progress"
        aria-valuetext={cancelling ? 'Cancelling' : valueText}
      />
      <Typography variant="body2" color="text.secondary" sx={{ minHeight: '1.5em', overflowWrap: 'anywhere' }}>
        {cancelling
          ? 'Cancellation requested. Running Git processes are being stopped.'
          : progress
            ? sentence(countedDetail(progress.detail))
            : ''}
      </Typography>
    </Stack>
  );
}

interface OutcomeCopy {
  severity: AlertColor;
  title: string;
  detail: string;
}

function outcomeCopy(status: JobStatus): OutcomeCopy {
  const stage = status.progress ? stageLabel(status.progress.stage) : '';
  const elapsed = formatDuration(status.elapsedMilliseconds);
  switch (status.status) {
    case 'succeeded':
      return {
        severity: 'success',
        title: 'Analysis complete',
        detail: `Finished in ${elapsed}${status.warnings.length ? ` with ${plural(status.warnings.length, 'warning')}` : ''}.`,
      };
    case 'cancelled':
      return {
        severity: 'info',
        title: 'Analysis cancelled',
        detail: `Stopped${stage ? ` during “${stage}”` : ''} after ${elapsed}. No report was produced.`,
      };
    case 'stale':
      return {
        severity: 'warning',
        title: 'The repository changed during the analysis',
        detail:
          `${sentence(status.error?.message ?? '')} The result is not shown because it would not match the repository ` +
          'as it is now. Run the analysis again once the repository is stable.',
      };
    default:
      return failureCopy(status);
  }
}

function failureCopy(status: JobStatus): OutcomeCopy {
  switch (status.error?.code) {
    case 'invalid_analysis_request':
      return {
        severity: 'error',
        title: 'The analysis request was rejected',
        detail:
          'The repository or its .commitography.yml did not pass validation, or a Since or Until date was not ' +
          'understood. Run commitography on the same path from a terminal to see the full message.',
      };
    case 'analysis_failed':
      return {
        severity: 'error',
        title: 'The analysis failed',
        detail:
          'Git or the analysis stopped before a report was produced. Try again; if it keeps failing, run ' +
          'commitography on the same path from a terminal to see the full error.',
      };
    default:
      return {
        severity: 'error',
        title: 'The analysis failed',
        detail: sentence(status.error?.message ?? 'No report was produced.'),
      };
  }
}

function Outcome({ status }: { status: JobStatus }): ReactElement {
  const copy = outcomeCopy(status);
  return (
    <Alert severity={copy.severity}>
      <AlertTitle>{copy.title}</AlertTitle>
      {copy.detail}
    </Alert>
  );
}

function PollError({ error, onRetry }: { error: ApiError; onRetry: () => void }): ReactElement {
  const sessionLost = error.status === 401;
  return (
    <Alert
      severity="warning"
      action={
        <Button
          color="inherit"
          size="small"
          sx={{ whiteSpace: 'nowrap' }}
          onClick={sessionLost ? () => window.location.reload() : onRetry}
        >
          {sessionLost ? 'Reload' : 'Retry'}
        </Button>
      }
    >
      <AlertTitle>{sessionLost ? 'The local session has expired' : 'Lost contact with the local server'}</AlertTitle>
      {sessionLost
        ? 'The server was probably restarted, which also clears its jobs. Reload the page to start a new session.'
        : 'Status updates are paused. Retrying only checks the status again; it does not start another analysis.'}
    </Alert>
  );
}

function JobNotFound({ onStartNew, onOpenRecent }: { onStartNew: () => void; onOpenRecent: () => void }): ReactElement {
  return (
    <Paper component="main" sx={{ p: { xs: 2.5, md: 5 }, borderRadius: 3 }}>
      <Stack spacing={2.5} sx={{ maxWidth: 680 }}>
        <Typography variant="overline" color="primary">
          Analysis
        </Typography>
        <Typography variant="h3" component="h2">
          This job is no longer available.
        </Typography>
        <Typography color="text.secondary">
          The local server keeps only the ten most recent jobs, and only while it is running. The server may have been
          restarted, or the job was removed or replaced by newer ones.
        </Typography>
        <Stack direction={{ xs: 'column', sm: 'row' }} spacing={1.5}>
          <Button variant="contained" onClick={onStartNew}>
            Start a new analysis
          </Button>
          <Button variant="text" color="inherit" onClick={onOpenRecent}>
            Recent jobs
          </Button>
        </Stack>
      </Stack>
    </Paper>
  );
}

function StageLog({ stages, status }: { stages: StageEntry[]; status: JobStatusValue }): ReactElement {
  const active = isActive(status);
  return (
    <Box>
      <Typography variant="overline" color="text.secondary" component="h3">
        Stages this session
      </Typography>
      <Box component="ol" sx={{ listStyle: 'none', m: 0, p: 0 }}>
        {stages.map((entry, index) => {
          const last = index === stages.length - 1;
          const current = active && last;
          // A run that ended without a report stopped inside its last stage.
          const stopped = last && !active && status !== 'succeeded';
          return (
            <Box
              component="li"
              key={entry.stage}
              aria-current={current ? 'step' : undefined}
              sx={{ display: 'flex', columnGap: 1.25, py: 0.5, alignItems: 'baseline' }}
            >
              <Box
                component="span"
                aria-hidden
                sx={{
                  width: '1em',
                  flexShrink: 0,
                  color: current ? 'primary.main' : stopped ? 'text.secondary' : 'success.main',
                }}
              >
                {current ? '●' : stopped ? '■' : '✓'}
              </Box>
              {stopped ? <Box component="span" sx={visuallyHidden}>Stopped: </Box> : null}
              <Box component="span" sx={{ display: 'flex', flexWrap: 'wrap', columnGap: 1.25, minWidth: 0 }}>
                <Typography component="span" sx={{ fontWeight: current ? 650 : 400 }}>
                  {stageLabel(entry.stage)}
                </Typography>
                {countedDetail(entry.detail) ? (
                  <Typography component="span" variant="body2" color="text.secondary" sx={{ overflowWrap: 'anywhere' }}>
                    {entry.detail}
                  </Typography>
                ) : null}
              </Box>
            </Box>
          );
        })}
      </Box>
    </Box>
  );
}

function Meta({ label, children }: { label: string; children: ReactNode }): ReactElement {
  return (
    <Box>
      <Typography variant="caption" color="text.secondary" component="p">
        {label}
      </Typography>
      <Typography sx={{ fontVariantNumeric: 'tabular-nums', fontWeight: 600 }}>{children}</Typography>
    </Box>
  );
}

/** Re-renders every second while `running` so elapsed time keeps moving. */
function useNow(running: boolean): number {
  const [now, setNow] = useState(() => performance.now());
  useEffect(() => {
    if (!running) return;
    const timer = window.setInterval(() => setNow(performance.now()), 1000);
    return () => window.clearInterval(timer);
  }, [running]);
  return now;
}
