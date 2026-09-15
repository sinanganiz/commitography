import './styles.css';
import { createRoot } from 'react-dom/client';

import { AppShell } from './app/AppShell';
import { ReportDashboard } from './report/Dashboard';
import { readEmbeddedReport, readWrappedOptions } from './report/embedded';
import { Wrapped } from './report/Wrapped';

function mount(): void {
  const container = document.getElementById('commitography-root');
  if (!container) return;
  const root = createRoot(container);
  const html = document.documentElement;

  // The Go renderer marks every static page with its mode; the local server's
  // application shell carries no mode and bootstraps from the API instead.
  const mode = html.dataset.mode;
  if (mode !== 'dashboard' && mode !== 'wrapped') {
    root.render(<AppShell />);
    return;
  }

  html.classList.add('cg-static');
  const report = readEmbeddedReport();
  if (!report) {
    root.render(<p className="banner">No report data was embedded in this page.</p>);
    return;
  }
  if (mode === 'wrapped') {
    const { year, previousYearCommits } = readWrappedOptions();
    root.render(<Wrapped report={report} year={year} previousYearCommits={previousYearCommits} />);
    return;
  }
  root.render(<ReportDashboard report={report} />);
}

if (document.readyState === 'loading') {
  document.addEventListener('DOMContentLoaded', mount, { once: true });
} else {
  mount();
}
