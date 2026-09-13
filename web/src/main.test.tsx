import { cleanup, render, waitFor } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { JobReport } from './app/JobReport';
import { jsonResponse, renderedSections, sampleReport } from './test/sampleReport';

// main.tsx mounts on import, the way the Go renderer and the local server load
// it, so each test prepares the page and imports a fresh copy.
async function boot(page: { mode?: string; data?: string; year?: string }): Promise<HTMLElement> {
  const html = document.documentElement;
  if (page.mode) html.dataset.mode = page.mode;
  if (page.year) html.dataset.year = page.year;
  document.body.innerHTML =
    '<div id="commitography-root"></div><script type="application/json" id="commitography-data"></script>';
  document.getElementById('commitography-data')!.textContent = page.data ?? '';
  await import('./main');
  return document.getElementById('commitography-root')!;
}

beforeEach(() => {
  vi.resetModules();
  const html = document.documentElement;
  delete html.dataset.mode;
  delete html.dataset.year;
  delete html.dataset.previousYearCommits;
  html.className = '';
});

afterEach(() => {
  cleanup();
  document.body.innerHTML = '';
});

describe('static bootstrap', () => {
  it('renders the dashboard from embedded report data', async () => {
    const root = await boot({ mode: 'dashboard', data: JSON.stringify(sampleReport()) });
    await waitFor(() => expect(root.querySelector('section#pulse')).not.toBeNull());
    expect(document.documentElement.classList.contains('cg-static')).toBe(true);
    expect(root.querySelector('h1')?.textContent).toBe('sample');
  });

  it('renders the Wrapped deck for the embedded year', async () => {
    vi.stubGlobal(
      'IntersectionObserver',
      class {
        observe() {}
        disconnect() {}
      },
    );
    const root = await boot({ mode: 'wrapped', year: '2025', data: JSON.stringify(sampleReport()) });
    await waitFor(() => expect(root.querySelectorAll('.wrapped-card').length).toBeGreaterThan(0));
    expect(root.querySelector('.wrapped-card')?.textContent).toContain('2025 in commits');
    expect(document.documentElement.classList.contains('wrapped-mode')).toBe(true);
  });

  it('explains a page without report data', async () => {
    const root = await boot({ mode: 'dashboard', data: 'not json' });
    await waitFor(() => expect(root.textContent).toBe('No report data was embedded in this page.'));
  });

  it('starts the local application when the page carries no mode', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn(async (input: RequestInfo | URL) =>
        String(input).endsWith('/capabilities')
          ? jsonResponse({ apiVersion: 'v1', reportSchemaVersion: 1, maxRecentJobs: 10, activeJobLimit: 1, pollIntervalMilliseconds: 750, supportsCancel: true, supportsOpen: true })
          : jsonResponse({ jobs: [] }),
      ),
    );
    const root = await boot({});
    await waitFor(() => expect(root.querySelector('nav')?.textContent).toContain('Recent jobs'));
    expect(document.documentElement.classList.contains('cg-static')).toBe(false);
  });
});

describe('static and server-fetched reports', () => {
  it('render the same sections with the same text', async () => {
    const report = sampleReport();
    const root = await boot({ mode: 'dashboard', data: JSON.stringify(report) });
    await waitFor(() => expect(root.querySelector('section#contributors')).not.toBeNull());
    const fromStatic = renderedSections(root);
    document.body.innerHTML = '';

    vi.stubGlobal(
      'fetch',
      vi.fn(async () => jsonResponse(report)),
    );
    const { container } = render(<JobReport id="0123456789abcdef" theme="dark" />);
    await waitFor(() => expect(container.querySelector('section#contributors')).not.toBeNull());
    const fromServer = renderedSections(container.querySelector('.cg-report')!);

    expect(fromServer).toEqual(fromStatic);
  });
});
