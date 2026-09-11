import './styles.css';
import { dashboardSections, footer, header, warnings } from './sections';
import { renderWrapped } from './wrapped';
import type { Report } from './types';

const DATA_ID = 'commitography-data';
const ROOT_ID = 'commitography-root';

export function readEmbeddedReport(): Report | null {
  const node = document.getElementById(DATA_ID);
  if (!node || !node.textContent) return null;
  try {
    const parsed = JSON.parse(node.textContent) as Report;
    return parsed && parsed.repository ? parsed : null;
  } catch {
    return null;
  }
}

export function mountLegacy(): void {
  const root = document.getElementById(ROOT_ID);
  if (!root) return;

  const report = readEmbeddedReport();
  if (!report) {
    root.appendChild(
      Object.assign(document.createElement('p'), {
        className: 'banner',
        textContent: 'No report data was embedded in this page.',
      }),
    );
    return;
  }

  const mode = document.documentElement.dataset.mode;
  if (mode === 'wrapped') {
    const year = Number(document.documentElement.dataset.year || '0');
    const previousRaw = document.documentElement.dataset.previousYearCommits;
    const previous = previousRaw === undefined || previousRaw === '' ? null : Number(previousRaw);
    renderWrapped(root, report, year, previous);
    return;
  }

  const page = document.createElement('div');
  page.className = 'page';

  const head = header(report);
  if (head) page.appendChild(head);

  const banner = warnings(report);
  if (banner) page.appendChild(banner);

  const main = document.createElement('main');
  main.id = 'content';
  for (const build of dashboardSections) {
    // A section with nothing to show is omitted entirely rather than rendered
    // as a row of zeroes.
    const node = build(report);
    if (node) main.appendChild(node);
  }
  page.appendChild(main);

  const foot = footer(report);
  if (foot) page.appendChild(foot);

  root.appendChild(page);
}
