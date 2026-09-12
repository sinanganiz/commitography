import { useEffect, useId, useState } from 'react';
import type { ReactElement } from 'react';
import Alert from '@mui/material/Alert';
import AlertTitle from '@mui/material/AlertTitle';
import Box from '@mui/material/Box';
import Button from '@mui/material/Button';
import LinearProgress from '@mui/material/LinearProgress';
import Typography from '@mui/material/Typography';
import type { PaletteMode } from '@mui/material/styles';

import { ApiError, getReport } from '../api/client';
import { ReportDashboard } from '../report/Dashboard';
import type { Report } from '../types';

interface ReportState {
  report: Report | null;
  error: ApiError | null;
}

/**
 * Loads the completed report of a succeeded job and renders it with the same
 * dashboard components as the static CLI page.
 */
export function JobReport({ id, theme }: { id: string; theme: PaletteMode }): ReactElement {
  const headingId = useId();
  const [attempt, setAttempt] = useState(0);
  const [state, setState] = useState<ReportState>({ report: null, error: null });

  useEffect(() => {
    let stopped = false;
    setState((current) => ({ report: current.report, error: null }));
    getReport(id)
      .then((report) => {
        if (!stopped) setState({ report, error: null });
      })
      .catch((caught: unknown) => {
        if (stopped) return;
        const error =
          caught instanceof ApiError ? caught : new ApiError(0, 'network_error', 'The report could not be read.');
        setState({ report: null, error });
      });
    return () => {
      stopped = true;
    };
  }, [id, attempt]);

  return (
    <Box component="section" aria-labelledby={headingId}>
      <Typography id={headingId} variant="overline" component="h2" color="primary">
        Report
      </Typography>
      {state.error ? <ReportError error={state.error} onRetry={() => setAttempt((n) => n + 1)} /> : null}
      {!state.report && !state.error ? <LinearProgress aria-label="Loading report" sx={{ mt: 2 }} /> : null}
      {state.report ? <ReportDashboard report={state.report} embedded theme={theme} /> : null}
    </Box>
  );
}

function ReportError({ error, onRetry }: { error: ApiError; onRetry: () => void }): ReactElement {
  if (error.status === 404) {
    return (
      <Alert severity="info">
        <AlertTitle>This report is no longer available</AlertTitle>
        The local server keeps only the ten most recent jobs, and only while it is running. Start a new analysis to see
        the repository again.
      </Alert>
    );
  }
  if (error.status === 409) {
    return (
      <Alert severity="warning">
        <AlertTitle>This job has no current report</AlertTitle>
        Only a succeeded analysis has a report. Stale, failed and cancelled jobs never show one.
      </Alert>
    );
  }
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
      <AlertTitle>{sessionLost ? 'The local session has expired' : 'The report could not be loaded'}</AlertTitle>
      {sessionLost
        ? 'The server was probably restarted, which also clears its jobs. Reload the page to start a new session.'
        : 'Retrying reads the finished report again; it does not run the analysis again.'}
    </Alert>
  );
}
