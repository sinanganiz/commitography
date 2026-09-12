import type { AlertColor } from '@mui/material/Alert';
import type { ChipProps } from '@mui/material/Chip';

import type { JobFailure, JobStatus, JobStatusValue } from '../api/client';
import { formatDuration, plural, sentence, stageLabel } from './format';

// The wording for a job's state lives here so the job view and the recent jobs
// list never describe the same outcome in two different ways.

export const statusChip: Record<JobStatusValue, { label: string; color: ChipProps['color'] }> = {
  queued: { label: 'Queued', color: 'default' },
  running: { label: 'Running', color: 'primary' },
  succeeded: { label: 'Succeeded', color: 'success' },
  failed: { label: 'Failed', color: 'error' },
  cancelled: { label: 'Cancelled', color: 'default' },
  stale: { label: 'Stale', color: 'warning' },
};

export interface OutcomeCopy {
  severity: AlertColor;
  title: string;
  detail: string;
}

/** The full explanation of a terminal job, shown on its own page. */
export function jobOutcome(status: JobStatus): OutcomeCopy {
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
      return failureCopy(status.error);
  }
}

function failureCopy(failure: JobFailure | null): OutcomeCopy {
  switch (failure?.code) {
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
        detail: sentence(failure?.message ?? 'No report was produced.'),
      };
  }
}

/**
 * A one-line outcome for the recent jobs list. `failure` is undefined while a
 * failed job's details are still loading.
 */
export function outcomeSummary(status: JobStatusValue, failure: JobFailure | null | undefined): string {
  switch (status) {
    case 'queued':
      return 'Waiting to start.';
    case 'running':
      return 'Analysis in progress.';
    case 'succeeded':
      return 'Report ready.';
    case 'cancelled':
      return 'Cancelled before a report was produced.';
    case 'stale':
      return 'No report: the repository changed during the analysis.';
    default:
      switch (failure?.code) {
        case 'invalid_analysis_request':
          return 'Rejected: the repository or its settings did not pass validation.';
        case 'analysis_failed':
          return 'Failed: Git or the analysis stopped before a report was produced.';
        case undefined:
          return 'Failed before a report was produced.';
        default:
          return `Failed: ${sentence(failure?.message ?? '')}`;
      }
  }
}
