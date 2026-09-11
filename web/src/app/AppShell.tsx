import { startTransition, useCallback, useEffect, useState } from 'react';
import type { ReactElement } from 'react';
import Alert from '@mui/material/Alert';
import Box from '@mui/material/Box';
import Button from '@mui/material/Button';
import Chip from '@mui/material/Chip';
import Container from '@mui/material/Container';
import CssBaseline from '@mui/material/CssBaseline';
import Divider from '@mui/material/Divider';
import IconButton from '@mui/material/IconButton';
import Paper from '@mui/material/Paper';
import Stack from '@mui/material/Stack';
import { ThemeProvider } from '@mui/material/styles';
import Tooltip from '@mui/material/Tooltip';
import Typography from '@mui/material/Typography';
import type { PaletteMode } from '@mui/material/styles';

import { getCapabilities, isActive, listJobs } from '../api/client';
import type { JobSummary } from '../api/client';
import { createCommitographyTheme } from '../ui/theme';
import { RepositoryForm, initialRepositoryForm } from './RepositoryForm';
import type { RepositoryFormState } from './RepositoryForm';

type View = 'analyze' | 'recent';
type SessionState = 'connecting' | 'ready' | 'failed';

/** Local runner shell shared by the web application views. */
export function AppShell(): ReactElement {
  const [view, setView] = useState<View>('analyze');
  const [mode, setMode] = useState<PaletteMode>('dark');
  const [session, setSession] = useState<SessionState>('connecting');
  const [connectAttempt, setConnectAttempt] = useState(0);
  const [jobs, setJobs] = useState<JobSummary[]>([]);
  // Kept here rather than in the form so switching views does not clear it.
  const [form, setForm] = useState<RepositoryFormState>(initialRepositoryForm);
  const theme = createCommitographyTheme(mode);
  const activeJob = jobs.find((job) => isActive(job.status)) ?? null;

  useEffect(() => {
    let cancelled = false;
    setSession('connecting');
    // Capabilities go first: that response issues the session cookie the job
    // list and every later request depend on.
    getCapabilities()
      .then(() => listJobs())
      .then((list) => {
        if (cancelled) return;
        setJobs(list);
        setSession('ready');
      })
      .catch(() => {
        if (!cancelled) setSession('failed');
      });
    return () => {
      cancelled = true;
    };
  }, [connectAttempt]);

  const refreshJobs = useCallback(() => {
    listJobs()
      .then(setJobs)
      .catch(() => setSession('failed'));
  }, []);

  const navigate = (next: View) => {
    startTransition(() => setView(next));
  };

  return (
    <ThemeProvider theme={theme}>
      <CssBaseline />
      <Box sx={{ minHeight: '100vh', bgcolor: 'background.default' }}>
        <Container maxWidth="lg" sx={{ py: { xs: 2, md: 4 } }}>
          <Stack spacing={{ xs: 3, md: 5 }}>
            <header>
              <Stack
                direction={{ xs: 'column', sm: 'row' }}
                spacing={2}
                sx={{ alignItems: { xs: 'flex-start', sm: 'center' }, justifyContent: 'space-between' }}
              >
                <Stack direction="row" spacing={1.5} sx={{ alignItems: 'center' }}>
                  <Typography variant="h4" component="h1" sx={{ letterSpacing: '-0.04em', fontWeight: 800 }}>
                    commitography
                  </Typography>
                  <Chip label="LOCAL" color="primary" size="small" variant="outlined" />
                </Stack>
                <Tooltip title={`Switch to ${mode === 'dark' ? 'light' : 'dark'} theme`}>
                  <IconButton
                    color="inherit"
                    aria-label={`Switch to ${mode === 'dark' ? 'light' : 'dark'} theme`}
                    onClick={() => setMode(mode === 'dark' ? 'light' : 'dark')}
                  >
                    {mode === 'dark' ? 'Light' : 'Dark'}
                  </IconButton>
                </Tooltip>
              </Stack>
              <Typography color="text.secondary" sx={{ mt: 1, maxWidth: 680 }}>
                Read your repository locally. Watch the analysis happen, then keep the report in the same workspace.
              </Typography>
            </header>

            <Stack direction="row" spacing={1} sx={{ borderBottom: 1, borderColor: 'divider' }}>
              <Button
                color={view === 'analyze' ? 'primary' : 'inherit'}
                variant={view === 'analyze' ? 'contained' : 'text'}
                onClick={() => navigate('analyze')}
              >
                Analyze
              </Button>
              <Button
                color={view === 'recent' ? 'primary' : 'inherit'}
                variant={view === 'recent' ? 'contained' : 'text'}
                onClick={() => navigate('recent')}
              >
                Recent jobs
              </Button>
            </Stack>

            {session === 'failed' ? (
              <Alert
                severity="error"
                action={
                  <Button color="inherit" size="small" onClick={() => setConnectAttempt((n) => n + 1)}>
                    Retry
                  </Button>
                }
              >
                The local server could not be reached. Check that commitography serve is still running.
              </Alert>
            ) : null}

            {view === 'analyze' ? (
              <AnalyzeView
                form={form}
                onFormChange={setForm}
                ready={session === 'ready'}
                activeJob={activeJob}
                onRefreshJobs={refreshJobs}
              />
            ) : (
              <RecentPlaceholder />
            )}
          </Stack>
        </Container>
      </Box>
    </ThemeProvider>
  );
}

interface AnalyzeViewProps {
  form: RepositoryFormState;
  onFormChange: (next: RepositoryFormState) => void;
  ready: boolean;
  activeJob: JobSummary | null;
  onRefreshJobs: () => void;
}

function AnalyzeView({ form, onFormChange, ready, activeJob, onRefreshJobs }: AnalyzeViewProps): ReactElement {
  return (
    <Paper component="main" sx={{ p: { xs: 2.5, md: 5 }, borderRadius: 3 }}>
      <Stack spacing={2.5} sx={{ maxWidth: 720 }}>
        <Typography variant="overline" color="primary">
          Start a local analysis
        </Typography>
        <Typography variant="h3" component="h2">
          See the shape of your repository.
        </Typography>
        <Typography color="text.secondary">
          Enter the path of a repository on this machine. The analysis runs locally, and the completed report stays in
          this dashboard.
        </Typography>
        <Divider />
        <RepositoryForm
          value={form}
          onChange={onFormChange}
          ready={ready}
          activeJob={activeJob}
          onStarted={onRefreshJobs}
          onRefreshJobs={onRefreshJobs}
        />
      </Stack>
    </Paper>
  );
}

function RecentPlaceholder(): ReactElement {
  return (
    <Paper component="main" sx={{ p: { xs: 2.5, md: 5 }, borderRadius: 3 }}>
      <Stack spacing={2}>
        <Typography variant="overline" color="primary">
          Recent jobs
        </Typography>
        <Typography variant="h3" component="h2">
          Nothing analyzed yet.
        </Typography>
        <Typography color="text.secondary">
          Completed, failed and cancelled analyses will appear here for the lifetime of this local server.
        </Typography>
      </Stack>
    </Paper>
  );
}
