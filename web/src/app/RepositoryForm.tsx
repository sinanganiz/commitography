import { useId, useRef, useState } from 'react';
import type { FormEvent, ReactElement, ReactNode } from 'react';
import Alert from '@mui/material/Alert';
import AlertTitle from '@mui/material/AlertTitle';
import Box from '@mui/material/Box';
import Button from '@mui/material/Button';
import Checkbox from '@mui/material/Checkbox';
import CircularProgress from '@mui/material/CircularProgress';
import Collapse from '@mui/material/Collapse';
import FormControlLabel from '@mui/material/FormControlLabel';
import Stack from '@mui/material/Stack';
import TextField from '@mui/material/TextField';
import Typography from '@mui/material/Typography';

import { createJob } from '../api/client';
import type { AnalysisOptions, CreateJobResponse, JobSummary } from '../api/client';
import { describeStartError } from './startErrors';
import type { StartErrorGuidance } from './startErrors';

/** Form values. They live only in React state for the lifetime of the page. */
export interface RepositoryFormState {
  repoPath: string;
  options: AnalysisOptions;
  advancedOpen: boolean;
}

export const initialRepositoryForm: RepositoryFormState = {
  repoPath: '',
  options: {
    noBlame: false,
    perAuthor: false,
    anonymize: false,
    allowShallow: false,
    countMerges: false,
    since: '',
    until: '',
  },
  advancedOpen: false,
};

type FlagOption = 'noBlame' | 'perAuthor' | 'anonymize' | 'allowShallow' | 'countMerges';

const flagOptions: { key: FlagOption; label: string; description: string }[] = [
  {
    key: 'noBlame',
    label: 'Skip blame',
    description:
      'Much faster on large repositories and Docker mounts. Line ownership and knowledge concentration are omitted.',
  },
  {
    key: 'perAuthor',
    label: 'Include per-author section',
    description: 'Adds a section with one row per contributor.',
  },
  {
    key: 'anonymize',
    label: 'Anonymize contributors',
    description: 'Replaces names with stable pseudonyms and drops e-mail addresses.',
  },
  {
    key: 'allowShallow',
    label: 'Allow shallow clone',
    description: 'Analyzes an incomplete history. Totals and first-commit dates will be understated.',
  },
  {
    key: 'countMerges',
    label: 'Count merge commits',
    description: 'Includes merge commits in the analysis. When off, the repository configuration decides.',
  },
];

interface RepositoryFormProps {
  value: RepositoryFormState;
  onChange: (next: RepositoryFormState) => void;
  /** False until the local session has been negotiated. */
  ready: boolean;
  /** The job that currently owns the server's single worker, if any. */
  activeJob: JobSummary | null;
  onStarted: (job: CreateJobResponse) => void;
  onRefreshJobs: () => void;
}

/** Repository path entry, advanced analysis options and the Start action. */
export function RepositoryForm({
  value,
  onChange,
  ready,
  activeJob,
  onStarted,
  onRefreshJobs,
}: RepositoryFormProps): ReactElement {
  const ids = useId();
  const advancedId = `${ids}-advanced`;
  const [submitting, setSubmitting] = useState(false);
  const [emptyPath, setEmptyPath] = useState(false);
  const [failure, setFailure] = useState<StartErrorGuidance | null>(null);
  // A ref, not state, so a double click inside one render cannot send twice.
  const inFlight = useRef(false);

  const { options } = value;
  const rangeError = invalidRange(options.since, options.until);
  const blocked = !ready || submitting || activeJob !== null;

  const setPath = (repoPath: string) => {
    setEmptyPath(false);
    if (failure?.pathProblem) setFailure(null);
    onChange({ ...value, repoPath });
  };

  const setOption = <K extends keyof AnalysisOptions>(key: K, next: AnalysisOptions[K]) => {
    if (key === 'allowShallow' && failure?.action === 'allowShallow') setFailure(null);
    onChange({ ...value, options: { ...options, [key]: next } });
  };

  const submit = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    if (blocked || inFlight.current) return;
    const repoPath = value.repoPath.trim();
    if (!repoPath) {
      setEmptyPath(true);
      return;
    }
    if (rangeError) {
      onChange({ ...value, advancedOpen: true });
      return;
    }

    inFlight.current = true;
    setSubmitting(true);
    setFailure(null);
    try {
      const job = await createJob({
        repoPath,
        options: { ...options, since: options.since.trim(), until: options.until.trim() },
      });
      onStarted(job);
    } catch (error) {
      const guidance = describeStartError(error);
      setFailure(guidance);
      if (guidance.action === 'refreshJobs') onRefreshJobs();
    } finally {
      inFlight.current = false;
      setSubmitting(false);
    }
  };

  const allowShallow = () => {
    setFailure(null);
    onChange({ ...value, advancedOpen: true, options: { ...options, allowShallow: true } });
  };

  const pathError = emptyPath ? 'Enter the path of a repository on this machine.' : failure?.pathProblem ? failure.title : '';
  // The active-job notice already explains a 409, so the error is not repeated.
  const shownFailure = failure?.action === 'refreshJobs' && activeJob ? null : failure;

  return (
    <Box component="form" noValidate onSubmit={submit} aria-label="Start a repository analysis">
      <Stack spacing={3}>
        <TextField
          label="Repository path"
          value={value.repoPath}
          onChange={(event) => setPath(event.target.value)}
          required
          fullWidth
          autoComplete="off"
          disabled={submitting}
          error={pathError !== ''}
          helperText={
            pathError ||
            'Type the path as the server sees it: the folder that contains .git. In Docker, use the mounted ' +
              'container path, such as /repos/project.'
          }
          slotProps={{
            htmlInput: {
              spellCheck: false,
              autoCapitalize: 'off',
              autoCorrect: 'off',
              style: { fontFamily: 'ui-monospace, SFMono-Regular, Menlo, Consolas, monospace' },
            },
          }}
        />

        <Box>
          <Button
            variant="text"
            color="inherit"
            aria-expanded={value.advancedOpen}
            aria-controls={advancedId}
            onClick={() => onChange({ ...value, advancedOpen: !value.advancedOpen })}
            sx={{ px: 1, ml: -1 }}
          >
            <Box component="span" aria-hidden sx={{ display: 'inline-block', width: '1.25em' }}>
              {value.advancedOpen ? '▾' : '▸'}
            </Box>
            Advanced options
            {!value.advancedOpen && changedOptionCount(options) > 0 ? ` (${changedOptionCount(options)} changed)` : ''}
          </Button>
          <Collapse in={value.advancedOpen} id={advancedId}>
            <Stack spacing={2.5} sx={{ pt: 2 }}>
              <Stack spacing={1}>
                {flagOptions.map((option) => (
                  <FormControlLabel
                    key={option.key}
                    sx={{ alignItems: 'flex-start', mr: 0 }}
                    control={
                      <Checkbox
                        checked={options[option.key]}
                        onChange={(event) => setOption(option.key, event.target.checked)}
                        disabled={submitting}
                        sx={{ mt: -0.75 }}
                      />
                    }
                    label={
                      <OptionLabel title={option.label}>{option.description}</OptionLabel>
                    }
                  />
                ))}
              </Stack>
              <Stack direction={{ xs: 'column', sm: 'row' }} spacing={2}>
                <TextField
                  label="Since"
                  value={options.since}
                  onChange={(event) => setOption('since', event.target.value)}
                  autoComplete="off"
                  disabled={submitting}
                  helperText="Any date Git understands, such as 2025-01-01 or 6 months ago."
                  fullWidth
                />
                <TextField
                  label="Until"
                  value={options.until}
                  onChange={(event) => setOption('until', event.target.value)}
                  autoComplete="off"
                  disabled={submitting}
                  error={rangeError}
                  helperText={rangeError ? 'Until must not be earlier than Since.' : 'Leave empty to include everything.'}
                  fullWidth
                />
              </Stack>
            </Stack>
          </Collapse>
        </Box>

        {options.perAuthor ? (
          <Alert severity={options.anonymize ? 'info' : 'warning'}>
            <AlertTitle>{options.anonymize ? 'Contributors appear under pseudonyms' : 'This report will name contributors'}</AlertTitle>
            {options.anonymize
              ? 'The per-author section lists each contributor under a stable pseudonym, without e-mail addresses.'
              : 'The per-author section lists each contributor by name with their activity. Enable it only when ' +
                'everyone involved is comfortable being named, or turn on Anonymize contributors.'}
          </Alert>
        ) : null}

        {activeJob ? (
          <Alert
            severity="info"
            action={
              <Button color="inherit" size="small" sx={{ whiteSpace: 'nowrap' }} onClick={onRefreshJobs}>
                Check again
              </Button>
            }
          >
            <AlertTitle>Analysis in progress</AlertTitle>
            {activeJob.repoName || 'A repository'} is being analyzed. A new analysis can start once this one finishes
            or is cancelled.
          </Alert>
        ) : null}

        {shownFailure ? (
          <Alert
            severity="error"
            action={
              shownFailure.action ? (
                <FailureAction action={shownFailure.action} onAllowShallow={allowShallow} onRefreshJobs={onRefreshJobs} />
              ) : undefined
            }
          >
            <AlertTitle>{shownFailure.title}</AlertTitle>
            {shownFailure.detail}
          </Alert>
        ) : null}

        <Stack direction="row" spacing={2} sx={{ alignItems: 'center' }}>
          <Button
            type="submit"
            variant="contained"
            size="large"
            disabled={blocked}
            startIcon={submitting ? <CircularProgress size={18} color="inherit" aria-hidden /> : undefined}
          >
            {submitting ? 'Starting…' : 'Start analysis'}
          </Button>
          {!ready ? (
            <Typography variant="body2" color="text.secondary" role="status">
              Connecting to the local server…
            </Typography>
          ) : null}
        </Stack>
      </Stack>
    </Box>
  );
}

function OptionLabel({ title, children }: { title: string; children: ReactNode }): ReactElement {
  return (
    <Box component="span" sx={{ display: 'block' }}>
      <Typography component="span" sx={{ display: 'block', fontWeight: 600 }}>
        {title}
      </Typography>
      <Typography component="span" variant="body2" color="text.secondary" sx={{ display: 'block' }}>
        {children}
      </Typography>
    </Box>
  );
}

function FailureAction({
  action,
  onAllowShallow,
  onRefreshJobs,
}: {
  action: NonNullable<StartErrorGuidance['action']>;
  onAllowShallow: () => void;
  onRefreshJobs: () => void;
}): ReactElement {
  switch (action) {
    case 'allowShallow':
      return (
        <Button color="inherit" size="small" sx={{ whiteSpace: 'nowrap' }} onClick={onAllowShallow}>
          Allow shallow
        </Button>
      );
    case 'refreshJobs':
      return (
        <Button color="inherit" size="small" sx={{ whiteSpace: 'nowrap' }} onClick={onRefreshJobs}>
          Check again
        </Button>
      );
    case 'reload':
      return (
        <Button color="inherit" size="small" sx={{ whiteSpace: 'nowrap' }} onClick={() => window.location.reload()}>
          Reload
        </Button>
      );
  }
}

function changedOptionCount(options: AnalysisOptions): number {
  const defaults = initialRepositoryForm.options;
  const normalize = (value: string | boolean) => (typeof value === 'string' ? value.trim() : value);
  return (Object.keys(defaults) as (keyof AnalysisOptions)[]).filter(
    (key) => normalize(options[key]) !== normalize(defaults[key]),
  ).length;
}

/**
 * Git accepts free-form dates, so only an unambiguous inversion of two
 * calendar dates is caught here; everything else is left to Git.
 */
function invalidRange(since: string, until: string): boolean {
  const isoDate = /^\d{4}-\d{2}-\d{2}$/;
  const from = since.trim();
  const to = until.trim();
  return isoDate.test(from) && isoDate.test(to) && from > to;
}
