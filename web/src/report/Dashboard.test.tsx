import { cleanup, fireEvent, render } from '@testing-library/react';
import { afterEach, describe, expect, it } from 'vitest';

import { allSectionIds, renderedSections, sampleReport } from '../test/sampleReport';
import { ReportDashboard } from './Dashboard';

afterEach(cleanup);

describe('ReportDashboard', () => {
  it('renders every section in reading order when the report has data for it', () => {
    const { container } = render(<ReportDashboard report={sampleReport()} />);
    expect(renderedSections(container).map((section) => section.id)).toEqual(allSectionIds);
  });

  it('omits sections without data instead of rendering zeroes', () => {
    const empty = sampleReport();
    empty.temporal.commitsPerMonth = [];
    empty.temporal.hourHistogram = Array(24).fill(0);
    empty.temporal.weekdayHistogram = Array(7).fill(0);
    empty.code.codeAge = [];
    empty.code.mostTouchedFiles = [];
    empty.social.churn = [];
    empty.social.coupling = [];
    empty.social.busFactor = 0;
    empty.messages.typeDistribution = {};
    delete empty.perAuthor;

    const { container } = render(<ReportDashboard report={empty} />);
    // Notables always has the weekend and merge cards to show.
    expect(renderedSections(container).map((section) => section.id)).toEqual(['notables']);
  });

  it('gives every chart an accessible name and a data table', () => {
    const { container } = render(<ReportDashboard report={sampleReport()} />);
    const figures = [...container.querySelectorAll('figure')];
    expect(figures.length).toBeGreaterThan(0);
    for (const figure of figures) {
      const chart = figure.querySelector('svg[role="img"]');
      const ids = chart?.getAttribute('aria-labelledby')?.split(' ') ?? [];
      expect(ids).toHaveLength(2);
      for (const id of ids) expect(document.getElementById(id)?.textContent).toBeTruthy();

      const toggle = figure.querySelector<HTMLButtonElement>('button.data-toggle')!;
      const table = document.getElementById(toggle.getAttribute('aria-controls')!)!;
      expect(table.querySelector('caption')?.textContent).toBeTruthy();
      expect(toggle.getAttribute('aria-expanded')).toBe('false');
      expect(table.closest('[hidden]')).not.toBeNull();
      fireEvent.click(toggle);
      expect(toggle.getAttribute('aria-expanded')).toBe('true');
      expect(table.closest('[hidden]')).toBeNull();
    }
  });

  it('uses page landmarks and an h1 on the static page', () => {
    const { container } = render(<ReportDashboard report={sampleReport()} />);
    expect(container.querySelector('h1')?.textContent).toBe('sample');
    expect(container.querySelector('main#content')).not.toBeNull();
    expect([...container.querySelectorAll('section[id] > .section-heading')].every((h) => h.tagName === 'H2')).toBe(true);
  });

  it('nests below the application heading and follows its theme when embedded', () => {
    const { container } = render(<ReportDashboard report={sampleReport()} embedded theme="light" />);
    expect(container.querySelector('h1')).toBeNull();
    expect(container.querySelector('main')).toBeNull();
    expect(container.querySelector('.cg-report')?.getAttribute('data-theme')).toBe('light');
    expect([...container.querySelectorAll('section[id] > .section-heading')].every((h) => h.tagName === 'H3')).toBe(true);
  });

  it('shows warnings and keeps per-author data opt-in', () => {
    const withWarning = sampleReport({ warnings: ['history: skipped commit'] });
    delete withWarning.perAuthor;
    const { container } = render(<ReportDashboard report={withWarning} />);
    expect(container.querySelector('.banner')?.textContent).toContain('history: skipped commit');
    expect(container.textContent).not.toContain('Ada Lovelace');
  });
});
