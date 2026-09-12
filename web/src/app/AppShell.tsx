import { useCallback, useEffect, useRef, useState } from 'react';
import type { ReactElement } from 'react';
import Alert from '@mui/material/Alert';
import Box from '@mui/material/Box';
import Button from '@mui/material/Button';
import Chip from '@mui/material/Chip';
import CircularProgress from '@mui/material/CircularProgress';
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

import { ApiError, getCapabilities, isActive, listJobs } from '../api/client';
import type { CreateJobResponse, JobSummary } from '../api/client';
import { createCommitographyTheme } from '../ui/theme';
import { JobView } from './JobView';
import { RecentJobs } from './RecentJobs';
import { RepositoryForm, initialRepositoryForm } from './RepositoryForm';
import type { RepositoryFormState } from './RepositoryForm';
import { navigate, routeHash, useRoute } from './routes';

type SessionState = 'connecting' | 'ready' | 'failed';
/** Why the session failed: no answer at all, or an answer rejecting the cookie. */
type SessionProblem = 'unreachable' | 'expired';

/** The API contract asks clients to poll every 500-1000 ms. */
const DEFAULT_POLL_MS = 750;
const DEFAULT_MAX_RECENT_JOBS = 10;

function clampPollInterval(value: number | undefined): number {
  if (!value || !Number.isFinite(value)) return DEFAULT_POLL_MS;
  return Math.min(1000, Math.max(500, value));
}

/** Local runner shell shared by the web application views. */
export function AppShell(): ReactElement {
  const route = useRoute();
  const [mode, setMode] = useState<PaletteMode>('dark');
  const [session, setSession] = useState<SessionState>('connecting');
  const [problem, setProblem] = useState<SessionProblem>('unreachable');
  const [connectAttempt, setConnectAttempt] = useState(0);
  const [pollIntervalMs, setPollIntervalMs] = useState(DEFAULT_POLL_MS);
  const [maxRecentJobs, setMaxRecentJobs] = useState(DEFAULT_MAX_RECENT_JOBS);
  const [jobs, setJobs] = useState<JobSummary[]>([]);
  const [restarted, setRestarted] = useState(false);
  // The job IDs of the last successful list, to recognise a restarted server.
  const knownIds = useRef<string[]>([]);
  // Kept here rather than in the form so switching views does not clear it.
  const [form, setForm] = useState<RepositoryFormState>(initialRepositoryForm);
  const theme = createCommitographyTheme(mode);
  const activeJob = jobs.find((job) => isActive(job.status)) ?? null;

  const acceptJobs = useCallback((list: JobSummary[]) => {
    knownIds.current = list.map((job) => job.id);
    setJobs(list);
  }, []);

  useEffect(() => {
    let cancelled = false;
    setSession('connecting');
    // Capabilities go first: that response issues the session cookie the job
    // list and every later request depend on.
    getCapabilities()
      .then((capabilities) => {
        if (!cancelled) {
          setPollIntervalMs(clampPollInterval(capabilities.pollIntervalMilliseconds));
          setMaxRecentJobs(capabilities.maxRecentJobs > 0 ? capabilities.maxRecentJobs : DEFAULT_MAX_RECENT_JOBS);
        }
        return listJobs();
      })
      .then((list) => {
        if (cancelled) return;
        // Jobs live only in the server's memory, so a reconnect that finds none
        // of the jobs this page knew means the server was restarted.
        const previous = knownIds.current;
        if (previous.length > 0 && !list.some((job) => previous.includes(job.id))) setRestarted(true);
        acceptJobs(list);
        setSession('ready');
      })
      .catch(() => {
        if (cancelled) return;
        setProblem('unreachable');
        setSession('failed');
      });
    return () => {
      cancelled = true;
    };
  }, [connectAttempt, acceptJobs]);

  // A failed background refresh keeps the last known list; only a lost
  // session needs the user, and views report their own connection problems.
  const refreshJobs = useCallback(async () => {
    try {
      acceptJobs(await listJobs());
    } catch (error) {
      if (error instanceof ApiError && error.status === 401) {
        setProblem('expired');
        setSession('failed');
      }
    }
  }, [acceptJobs]);

  const reconnect = () => setConnectAttempt((n) => n + 1);

  // While a job is active the list is refreshed at a relaxed pace, so the
  // form unlocks and the activity indicator clears when it finishes.
  const activeId = activeJob?.id;
  useEffect(() => {
    if (!activeId || session !== 'ready') return;
    const timer = window.setInterval(refreshJobs, pollIntervalMs * 2);
    return () => window.clearInterval(timer);
  }, [activeId, session, pollIntervalMs, refreshJobs]);

  // The history is read again whenever it is opened, and when the tab becomes
  // visible, which is also how a restarted server is noticed.
  const viewingRecent = route.name === 'recent';
  useEffect(() => {
    if (viewingRecent && session === 'ready') void refreshJobs();
  }, [viewingRecent, session, refreshJobs]);
  useEffect(() => {
    const onVisible = () => {
      if (document.visibilityState === 'visible' && session === 'ready') void refreshJobs();
    };
    document.addEventListener('visibilitychange', onVisible);
    return () => document.removeEventListener('visibilitychange', onVisible);
  }, [session, refreshJobs]);

  const jobStarted = (job: CreateJobResponse) => {
    setRestarted(false);
    void refreshJobs();
    navigate({ name: 'job', id: job.id });
  };

  // A view change removes the control that had focus. Focus moves to the new
  // view's heading instead, so a screen reader announces where the user is and
  // Tab continues from the top of that view. The first render keeps the
  // browser's own starting point.
  const routeKey = routeHash(route);
  const firstRoute = useRef(true);
  useEffect(() => {
    if (firstRoute.current) {
      firstRoute.current = false;
      return;
    }
    const frame = window.requestAnimationFrame(() => {
      document.querySelector<HTMLElement>('main h2[tabindex="-1"]')?.focus();
    });
    return () => window.cancelAnimationFrame(frame);
  }, [routeKey]);

  const viewingActiveJob = route.name === 'job' && route.id === activeJob?.id;

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

            <Stack
              component="nav"
              aria-label="Main"
              direction="row"
              spacing={1}
              sx={{ borderBottom: 1, borderColor: 'divider', pb: 1, flexWrap: 'wrap', rowGap: 1 }}
            >
              <NavLink href={routeHash({ name: 'analyze' })} current={route.name === 'analyze'}>
                Analyze
              </NavLink>
              <NavLink href={routeHash({ name: 'recent' })} current={route.name === 'recent'}>
                Recent jobs
              </NavLink>
              {activeJob && !viewingActiveJob ? (
                <Button
                  href={routeHash({ name: 'job', id: activeJob.id })}
                  variant="outlined"
                  color="primary"
                  startIcon={<CircularProgress size={14} color="inherit" aria-hidden />}
                  sx={{ ml: { sm: 'auto !important' } }}
                >
                  Analyzing {activeJob.repoName || 'repository'}
                </Button>
              ) : null}
            </Stack>

            {session === 'failed' ? (
              <Alert
                severity="error"
                action={
                  <Button color="inherit" size="small" onClick={reconnect}>
                    Reconnect
                  </Button>
                }
              >
                {problem === 'expired'
                  ? 'The local session has expired, usually because commitography serve was restarted. Reconnect to start a new session.'
                  : 'The local server could not be reached. Check that commitography serve is still running.'}
              </Alert>
            ) : null}

            {route.name === 'analyze' ? (
              <AnalyzeView
                form={form}
                onFormChange={setForm}
                ready={session === 'ready'}
                activeJob={activeJob}
                onStarted={jobStarted}
                onRefreshJobs={refreshJobs}
              />
            ) : null}
            {route.name === 'recent' ? (
              <RecentJobs
                jobs={jobs}
                session={session}
                maxJobs={maxRecentJobs}
                restarted={restarted}
                onRefresh={refreshJobs}
                onReconnect={reconnect}
                onStartNew={() => navigate({ name: 'analyze' })}
              />
            ) : null}
            {/* The job view waits for the first session negotiation, then stays
                mounted and reports its own polling problems. Keying by ID gives
                every job a fresh poll and stage log. */}
            {route.name === 'job' && session !== 'connecting' ? (
              <JobView
                key={route.id}
                id={route.id}
                pollIntervalMs={pollIntervalMs}
                themeMode={mode}
                onSettled={refreshJobs}
                onStartNew={() => navigate({ name: 'analyze' })}
                onOpenRecent={() => navigate({ name: 'recent' })}
              />
            ) : null}
          </Stack>
        </Container>
      </Box>
    </ThemeProvider>
  );
}

function NavLink({ href, current, children }: { href: string; current: boolean; children: string }): ReactElement {
  return (
    <Button
      href={href}
      color={current ? 'primary' : 'inherit'}
      variant={current ? 'contained' : 'text'}
      aria-current={current ? 'page' : undefined}
    >
      {children}
    </Button>
  );
}

interface AnalyzeViewProps {
  form: RepositoryFormState;
  onFormChange: (next: RepositoryFormState) => void;
  ready: boolean;
  activeJob: JobSummary | null;
  onStarted: (job: CreateJobResponse) => void;
  onRefreshJobs: () => void;
}

function AnalyzeView({ form, onFormChange, ready, activeJob, onStarted, onRefreshJobs }: AnalyzeViewProps): ReactElement {
  return (
    <Paper component="main" sx={{ p: { xs: 2.5, md: 5 }, borderRadius: 3 }}>
      <Stack spacing={2.5} sx={{ maxWidth: 720 }}>
        <Typography variant="overline" color="primary">
          Start a local analysis
        </Typography>
        <Typography variant="h3" component="h2" tabIndex={-1} sx={{ outline: 'none' }}>
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
          onStarted={onStarted}
          onRefreshJobs={onRefreshJobs}
        />
      </Stack>
    </Paper>
  );
}
