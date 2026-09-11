import { startTransition, useState } from 'react';
import type { ReactElement } from 'react';
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

import { createCommitographyTheme } from '../ui/theme';

type View = 'analyze' | 'recent';

/** Local runner shell shared by the web application views. */
export function AppShell(): ReactElement {
  const [view, setView] = useState<View>('analyze');
  const [mode, setMode] = useState<PaletteMode>('dark');
  const theme = createCommitographyTheme(mode);

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

            {view === 'analyze' ? <AnalyzePlaceholder /> : <RecentPlaceholder />}
          </Stack>
        </Container>
      </Box>
    </ThemeProvider>
  );
}

function AnalyzePlaceholder(): ReactElement {
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
          Choose a repository path to begin. The analysis runs on this machine, and the completed report stays in this
          dashboard.
        </Typography>
        <Divider />
        <Button variant="contained" size="large" disabled sx={{ alignSelf: 'flex-start' }}>
          Repository form coming next
        </Button>
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
