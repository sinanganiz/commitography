import { createTheme } from '@mui/material/styles';
import type { PaletteMode } from '@mui/material/styles';

const light = {
  background: '#fbfaf8',
  paper: '#ffffff',
  border: '#d9d4cc',
  text: '#1b1a18',
  muted: '#55514b',
  accent: '#8a4b1f',
  accentSoft: '#f0e2d4',
};

const dark = {
  background: '#14130f',
  paper: '#1c1b17',
  border: '#33312b',
  text: '#f2efe8',
  muted: '#a8a29a',
  accent: '#e0a76a',
  accentSoft: '#33261a',
};

const systemFont = 'ui-sans-serif, system-ui, -apple-system, "Segoe UI", Roboto, "Helvetica Neue", Arial, sans-serif';
const monoFont = 'ui-monospace, SFMono-Regular, "SF Mono", Menlo, Consolas, "Liberation Mono", monospace';

/** Creates the local dashboard theme without loading any external assets. */
export function createCommitographyTheme(mode: PaletteMode) {
  const colors = mode === 'dark' ? dark : light;
  return createTheme({
    palette: {
      mode,
      primary: { main: colors.accent },
      background: { default: colors.background, paper: colors.paper },
      text: { primary: colors.text, secondary: colors.muted },
      divider: colors.border,
      warning: { main: mode === 'dark' ? '#e8b04b' : '#8a6500' },
      error: { main: mode === 'dark' ? '#ef8a7a' : '#a2382b' },
      success: { main: mode === 'dark' ? '#9ecb9b' : '#3d7540' },
    },
    typography: {
      fontFamily: systemFont,
      h1: { fontWeight: 700, letterSpacing: '-0.02em' },
      h2: { fontWeight: 700, letterSpacing: '-0.01em' },
      h3: { fontWeight: 650 },
      button: { fontWeight: 650, textTransform: 'none' },
      overline: { fontFamily: monoFont, letterSpacing: '0.08em' },
    },
    shape: { borderRadius: 10 },
    components: {
      MuiCssBaseline: {
        styleOverrides: {
          ':focus-visible': {
            outline: `2px solid ${colors.accent}`,
            outlineOffset: 2,
          },
          body: {
            backgroundColor: colors.background,
            color: colors.text,
          },
          code: { fontFamily: monoFont },
        },
      },
      MuiPaper: {
        styleOverrides: {
          root: {
            backgroundImage: 'none',
            border: `1px solid ${colors.border}`,
          },
        },
      },
      MuiButton: {
        styleOverrides: {
          root: { borderRadius: 999, paddingInline: 16 },
        },
      },
      MuiOutlinedInput: {
        styleOverrides: {
          root: { borderRadius: 10 },
        },
      },
      MuiLinearProgress: {
        styleOverrides: {
          root: { borderRadius: 999, height: 8 },
          bar: { borderRadius: 999 },
        },
      },
    },
  });
}
