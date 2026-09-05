import { areaChart, barChart, figure, heatmap, radialHistogram, strataChart } from './charts';
import { el, isoDate, isoDateTime, num, offsetLabel, pct, shortPath, svgEl } from './dom';
import type { Report } from './types';

const WEEKDAYS = ['Mon', 'Tue', 'Wed', 'Thu', 'Fri', 'Sat', 'Sun'];

/** A section is omitted entirely when it has nothing to say. Returning null is
 *  how a builder declines to render. */
export type SectionBuilder = (r: Report) => HTMLElement | null;

function section(id: string, title: string, ...children: (Node | string | null | false)[]): HTMLElement {
  return el(
    'section',
    { class: 'section', id, 'aria-labelledby': `${id}-heading` },
    el('h2', { class: 'section-heading', id: `${id}-heading` }, title),
    ...children,
  );
}

function subhead(content: string): HTMLElement {
  return el('p', { class: 'section-sub' }, content);
}

function stat(value: string, label: string, note?: string): HTMLElement {
  return el(
    'div',
    { class: 'stat' },
    el('div', { class: 'stat-value' }, value),
    el('div', { class: 'stat-label' }, label),
    note ? el('div', { class: 'stat-note' }, note) : null,
  );
}

function statRow(...stats: HTMLElement[]): HTMLElement {
  return el('div', { class: 'stat-row' }, ...stats);
}

function table(columns: string[], rows: (Node | string)[][]): HTMLElement {
  return el(
    'div',
    { class: 'table-wrap' },
    el(
      'table',
      { class: 'listing' },
      el('thead', {}, el('tr', {}, ...columns.map((c) => el('th', { scope: 'col' }, c)))),
      el('tbody', {}, ...rows.map((cells) => el('tr', {}, ...cells.map((c) => el('td', {}, c))))),
    ),
  );
}

// --- header ---------------------------------------------------------------

export const header: SectionBuilder = (r) => {
  const repo = r.repository;
  const range =
    repo.firstCommit && repo.lastCommit
      ? `${isoDate(repo.firstCommit)} – ${isoDate(repo.lastCommit)}`
      : 'no dated commits';

  return el(
    'header',
    { class: 'page-header' },
    el(
      'div',
      { class: 'page-header-top' },
      el('h1', { class: 'repo-name' }, repo.name || 'repository'),
      themeToggle(),
    ),
    el('p', { class: 'repo-range' }, range),
    statRow(
      stat(num(repo.commitsAnalyzed), 'commits analyzed', `${num(repo.commitsTotal)} in history`),
      stat(num(repo.contributors), 'contributors'),
      stat(num(repo.ageDays), 'days of history'),
      stat(num(repo.trackedFiles), 'files tracked', `${num(repo.trackedLines)} lines`),
    ),
    el(
      'p',
      { class: 'meta-line' },
      `Analyzed ${isoDate(r.generatedAt)} with commitography ${r.toolVersion}`,
      repo.headCommit ? ` at ${repo.headCommit.slice(0, 10)}` : '',
    ),
  );
};

function themeToggle(): HTMLElement {
  const button = el('button', { type: 'button', class: 'theme-toggle' }, 'Switch theme');
  button.addEventListener('click', () => {
    const root = document.documentElement;
    // Held in memory only. The page uses no storage API of any kind.
    const next = root.dataset.theme === 'dark' ? 'light' : 'dark';
    root.dataset.theme = next;
    button.setAttribute('aria-label', `Switch to ${next === 'dark' ? 'light' : 'dark'} theme`);
  });
  return button;
}

// --- warnings -------------------------------------------------------------

export const warnings: SectionBuilder = (r) => {
  if (r.warnings.length === 0) return null;
  return el(
    'div',
    { class: 'banner', role: 'status' },
    el('strong', {}, r.warnings.length === 1 ? 'Warning' : 'Warnings'),
    el('ul', {}, ...r.warnings.map((w) => el('li', {}, w))),
  );
};

// --- pulse ----------------------------------------------------------------

export const pulse: SectionBuilder = (r) => {
  const series = r.temporal.commitsPerMonth;
  if (series.length === 0) return null;

  const chart = areaChart(
    series.map((m) => m.month),
    series.map((m) => m.count),
  );

  return section(
    'pulse',
    'Pulse',
    subhead('Commits per month across the whole history, with quiet months left visible.'),
    figure(chart, {
      title: 'Commits per month',
      desc: `Commit counts for each of the ${series.length} months between ${series[0].month} and ${series[series.length - 1].month}.`,
      columns: ['Month', 'Commits'],
      rows: series.map((m) => ({ label: m.month, values: [m.count] })),
    }),
  );
};

// --- chronotype -----------------------------------------------------------

export const chronotype: SectionBuilder = (r) => {
  const t = r.temporal;
  if (t.hourHistogram.every((v) => v === 0)) return null;

  const ring = radialHistogram(t.hourHistogram);
  const grid = heatmap(t.hourWeekdayGrid, WEEKDAYS);

  const gridRows = t.hourWeekdayGrid.map((row, day) => ({
    label: WEEKDAYS[day],
    values: row,
  }));

  return section(
    'chronotype',
    'Chronotype',
    subhead("When this repository is awake, in each author's own local time."),
    statRow(
      stat(pct(t.nightOwlRatio), 'commits at night', 'between 22:00 and 06:00'),
      stat(num(t.braveDeploys), 'brave deploys', 'Friday, after 17:00'),
    ),
    figure(ring, {
      title: 'Commits by hour of day',
      desc: 'A twenty-four segment ring; each segment extends outward in proportion to the commits made in that local hour.',
      columns: ['Hour', 'Commits'],
      rows: t.hourHistogram.map((v, hour) => ({
        label: `${String(hour).padStart(2, '0')}:00`,
        values: [v],
      })),
    }),
    figure(grid, {
      title: 'Commits by weekday and hour',
      desc: 'A seven by twenty-four grid; each cell is shaded in five steps according to how many commits fall in that weekday and hour.',
      columns: ['Weekday', ...Array.from({ length: 24 }, (_, h) => String(h).padStart(2, '0'))],
      rows: gridRows,
    }),
  );
};

// --- rhythm ---------------------------------------------------------------

export const rhythm: SectionBuilder = (r) => {
  const t = r.temporal;
  if (t.weekdayHistogram.every((v) => v === 0)) return null;

  const chart = barChart(WEEKDAYS, t.weekdayHistogram, 'commits');

  return section(
    'rhythm',
    'Rhythm',
    subhead('The working week, and the longest stretches of noise and quiet.'),
    figure(chart, {
      title: 'Commits by weekday',
      desc: 'A bar per weekday, Monday through Sunday, with the count printed beside each bar.',
      columns: ['Weekday', 'Commits'],
      rows: WEEKDAYS.map((d, i) => ({ label: d, values: [t.weekdayHistogram[i]] })),
    }),
    statRow(
      t.longestStreak
        ? stat(
            `${num(t.longestStreak.days)} days`,
            'longest streak',
            `${t.longestStreak.startDate} – ${t.longestStreak.endDate}`,
          )
        : stat('—', 'longest streak'),
      t.longestSilence
        ? stat(
            `${num(t.longestSilence.days)} days`,
            'longest silence',
            `${t.longestSilence.startDate} – ${t.longestSilence.endDate}`,
          )
        : stat('—', 'longest silence'),
      t.busiestDay
        ? stat(num(t.busiestDay.count), 'commits in one day', t.busiestDay.date)
        : stat('—', 'busiest day'),
    ),
  );
};

// --- code age -------------------------------------------------------------

export const codeAge: SectionBuilder = (r) => {
  const c = r.code;
  if (c.codeAge.length === 0) return null;

  const chart = strataChart(
    c.codeAge.map((a) => a.year),
    c.codeAge.map((a) => a.lines),
  );
  const total = c.codeAge.reduce((sum, a) => sum + a.lines, 0);

  return section(
    'code-age',
    'Code age',
    subhead(
      `Which year each surviving line was last written in, sampled from ${num(c.codeAgeSampledFiles)} of ${num(c.codeAgeTotalFiles)} text files.`,
    ),
    c.survivingFromFirstYear !== null
      ? statRow(
          stat(pct(c.survivingFromFirstYear), 'of sampled lines', "date from the repository's first year"),
        )
      : null,
    figure(chart, {
      title: 'Surviving lines by year last modified',
      desc: `A single band divided into ${c.codeAge.length} strata, one per year, sized by its share of the ${num(total)} sampled lines.`,
      columns: ['Year', 'Lines', 'Share'],
      rows: c.codeAge.map((a) => ({
        label: String(a.year),
        values: [a.lines, pct(a.lines / (total || 1))],
      })),
    }),
  );
};

// --- hotspots -------------------------------------------------------------

export const hotspots: SectionBuilder = (r) => {
  const c = r.code;
  const churn = r.social.churn;
  if (c.mostTouchedFiles.length === 0 && churn.length === 0) return null;

  const children: (HTMLElement | null)[] = [
    subhead('The files this repository keeps coming back to.'),
    statRow(
      stat(num(c.totalAdded), 'lines added'),
      stat(num(c.totalDeleted), 'lines deleted'),
      stat(String(c.averageCommitSize), 'average commit', `median ${c.medianCommitSize}`),
    ),
  ];

  if (c.mostTouchedFiles.length > 0) {
    children.push(
      el('h3', { class: 'subsection-heading' }, 'Most modified files'),
      table(
        ['File', 'Commits', 'Added', 'Deleted'],
        c.mostTouchedFiles.map((f) => [
          el(
            'span',
            { class: 'path', title: f.path },
            shortPath(f.path),
            f.deletedFromHead ? el('span', { class: 'tag' }, 'removed') : null,
          ),
          num(f.commits),
          num(f.added),
          num(f.deleted),
        ]),
      ),
    );
  }

  if (churn.length > 0) {
    children.push(
      el('h3', { class: 'subsection-heading' }, 'Churn hotspots'),
      subhead('Files rewritten at least five times inside a single 30-day window.'),
      table(
        ['File', 'Peak in 30 days', 'Window from', 'Total commits'],
        churn.map((h) => [
          el('span', { class: 'path', title: h.path }, shortPath(h.path)),
          num(h.maxCommitsInWindow),
          isoDate(h.windowStart),
          num(h.totalCommits),
        ]),
      ),
    );
  }

  if (c.oldestUntouchedFile) {
    children.push(
      el(
        'p',
        { class: 'aside' },
        'Least recently modified file still in the tree: ',
        el('code', {}, c.oldestUntouchedFile.path),
        `, last changed ${isoDate(c.oldestUntouchedFile.lastModified)}.`,
      ),
    );
  }

  return section('hotspots', 'Hotspots', ...children);
};

// --- coupling -------------------------------------------------------------

export const coupling: SectionBuilder = (r) => {
  const pairs = r.social.coupling;
  if (pairs.length === 0) return null;

  const maxSupport = Math.max(...pairs.map((p) => p.support));

  const list = el(
    'ol',
    { class: 'coupling-list' },
    ...pairs.map((p) =>
      el(
        'li',
        { class: p.expected ? 'coupling-item coupling-expected' : 'coupling-item' },
        el(
          'div',
          { class: 'coupling-paths' },
          el('code', { title: p.a }, shortPath(p.a, 40)),
          el('span', { class: 'coupling-join' }, 'changes with'),
          el('code', { title: p.b }, shortPath(p.b, 40)),
          p.expected ? el('span', { class: 'tag' }, 'expected pair') : null,
        ),
        el(
          'div',
          { class: 'coupling-meter' },
          svgEl(
            'svg',
            { viewBox: '0 0 100 6', class: 'meter', 'aria-hidden': 'true' },
            svgEl('rect', { class: 'meter-track', x: 0, y: 0, width: 100, height: 6, rx: 3 }),
            svgEl('rect', {
              class: 'meter-value',
              x: 0,
              y: 0,
              width: (p.support / maxSupport) * 100,
              height: 6,
              rx: 3,
            }),
          ),
          el(
            'span',
            { class: 'coupling-numbers' },
            `${num(p.support)} commits together, ${pct(p.confidence, 0)} confidence`,
          ),
        ),
      ),
    ),
  );

  return section(
    'coupling',
    'Change coupling',
    subhead(
      'File pairs that keep changing in the same commit. Pairs sharing a name, such as a file and its test, are marked as expected.',
    ),
    list,
  );
};

// --- bus factor -----------------------------------------------------------

export const busFactor: SectionBuilder = (r) => {
  const s = r.social;
  if (s.busFactor === 0) return null;

  const children: (HTMLElement | null)[] = [
    subhead(
      'The smallest number of contributors accounting for at least half the commits. It measures concentration of knowledge, not anyone’s output.',
    ),
    statRow(stat(num(s.busFactor), 'repository bus factor')),
  ];

  if (s.directoryBusFactor.length > 0) {
    children.push(
      table(
        ['Directory', 'Bus factor', 'Contributors', 'Commits'],
        s.directoryBusFactor.map((d) => [
          el('code', {}, d.path),
          num(d.busFactor),
          num(d.contributors),
          num(d.commits),
        ]),
      ),
    );
  }

  if (s.knowledgeConcentration.length > 0) {
    children.push(
      el('h3', { class: 'subsection-heading' }, 'Knowledge concentration'),
      subhead('The share of each top-level directory held by its single largest contributor.'),
      table(
        ['Directory', 'Largest share', 'Contributors', ...(s.knowledgeConcentration[0].topContributor ? ['Top contributor'] : [])],
        s.knowledgeConcentration.map((k) => [
          el('code', {}, k.path),
          pct(k.largestShare, 0),
          num(k.contributors),
          ...(k.topContributor ? [k.topContributor] : []),
        ]),
      ),
    );
  }

  return section('bus-factor', 'Bus factor', ...children);
};

// --- messages -------------------------------------------------------------

export const messages: SectionBuilder = (r) => {
  const m = r.messages;
  const types = Object.entries(m.typeDistribution).sort((a, b) => b[1] - a[1]);
  if (types.length === 0) return null;

  const chart = barChart(
    types.map(([type]) => type),
    types.map(([, count]) => count),
    'commits',
  );

  const children: (HTMLElement | null)[] = [
    subhead(
      m.lowConfidence
        ? `Only ${pct(m.conventionalRatio, 0)} of subjects follow Conventional Commits, so the classification below is a low-confidence guess.`
        : `${pct(m.conventionalRatio, 0)} of subjects follow Conventional Commits.`,
    ),
    m.lowConfidence ? el('p', { class: 'confidence-flag' }, 'Low confidence classification') : null,
    figure(chart, {
      title: 'Commit type distribution',
      desc: `Commit counts across ${types.length} categories, from the Conventional Commits prefix where present and from keyword rules otherwise.`,
      columns: ['Type', 'Commits'],
      rows: types.map(([type, count]) => ({ label: type, values: [count] })),
    }),
    statRow(
      stat(num(m.shortMessages), 'placeholder messages', '"wip", "fix", "..."'),
      stat(num(m.revertCount), 'reverts'),
      stat(num(m.typoFixCount), 'typo fixes'),
      stat(num(m.emojiCommits), 'commits with emoji'),
      stat(String(m.averageSubjectLength), 'average subject length', 'characters'),
    ),
  ];

  if (m.topWords.length > 0) {
    const max = m.topWords[0].count;
    children.push(
      el('h3', { class: 'subsection-heading' }, 'What gets talked about'),
      el(
        'ul',
        { class: 'word-cloud' },
        ...m.topWords.map((w) =>
          el(
            'li',
            {
              class: 'word',
              style: `font-size:${(0.85 + (w.count / max) * 1.5).toFixed(2)}rem`,
              title: `${w.word}: ${w.count}`,
            },
            w.word,
            el('span', { class: 'word-count' }, String(w.count)),
          ),
        ),
      ),
    );
  }

  if (m.topEmoji.length > 0) {
    children.push(
      el(
        'p',
        { class: 'aside' },
        'Most used emoji: ',
        ...m.topEmoji.map((e) => el('span', { class: 'emoji-chip' }, `${e.emoji} ${num(e.count)}`)),
      ),
    );
  }

  if (m.longestSubject) {
    children.push(
      el(
        'p',
        { class: 'aside' },
        `Longest subject, at ${num(m.longestSubject.length)} characters: `,
        el('q', {}, m.longestSubject.subject),
      ),
    );
  }

  return section('messages', 'Messages', ...children);
};

// --- notables -------------------------------------------------------------

export const notables: SectionBuilder = (r) => {
  const n = r.notables;
  const cards: HTMLElement[] = [];

  const card = (title: string, body: (Node | string | null)[]) =>
    el('div', { class: 'card' }, el('h3', {}, title), ...body);

  cards.push(card('Weekend work', [el('p', { class: 'card-figure' }, pct(n.weekendRatio))]));
  cards.push(card('Merge commits', [el('p', { class: 'card-figure' }, num(n.mergeCount))]));

  if (n.holidayCommits > 0) {
    cards.push(
      card('Holiday commits', [
        el('p', { class: 'card-figure' }, num(n.holidayCommits)),
        el('p', {}, 'on 25 December or 1 January'),
      ]),
    );
  }

  if (n.latestNightCommit) {
    cards.push(
      card('Deepest into the night', [
        el('p', { class: 'card-figure' }, isoDateTime(n.latestNightCommit.date).slice(-5)),
        el('p', {}, n.latestNightCommit.subject),
        el('p', { class: 'card-meta' }, isoDate(n.latestNightCommit.date)),
      ]),
    );
  }

  if (n.earliestMorningCommit) {
    cards.push(
      card('Earliest start', [
        el('p', { class: 'card-figure' }, isoDateTime(n.earliestMorningCommit.date).slice(-5)),
        el('p', {}, n.earliestMorningCommit.subject),
        el('p', { class: 'card-meta' }, isoDate(n.earliestMorningCommit.date)),
      ]),
    );
  }

  if (n.firstCommitSubject) {
    cards.push(card('It began with', [el('p', { class: 'card-quote' }, n.firstCommitSubject)]));
  }

  if (n.timezoneSpread.length > 0) {
    cards.push(
      card('Timezones', [
        el('p', { class: 'card-figure' }, num(n.timezoneSpread.length)),
        el(
          'ul',
          { class: 'card-list' },
          ...n.timezoneSpread.map((z) =>
            el('li', {}, `${offsetLabel(z.offsetMinutes)} — ${num(z.commits)}`),
          ),
        ),
      ]),
    );
  }

  if (n.bulkCommits.length > 0) {
    cards.push(
      card('Bulk commits', [
        el(
          'p',
          {},
          'Left out of line metrics because one import would otherwise define every distribution.',
        ),
        el(
          'ul',
          { class: 'card-list' },
          ...n.bulkCommits.map((b) =>
            el('li', {}, `${isoDate(b.date)} — ${num(b.linesChanged)} lines — ${b.subject}`),
          ),
        ),
      ]),
    );
  }

  if (cards.length === 0) return null;
  return section('notables', 'Notables', el('div', { class: 'card-grid' }, ...cards));
};

// --- contributors ---------------------------------------------------------

export const contributors: SectionBuilder = (r) => {
  if (!r.perAuthor || r.perAuthor.authors.length === 0) return null;

  return section(
    'contributors',
    'Contributors',
    subhead('Listed in order of first commit, not by volume. These figures describe participation, not performance.'),
    table(
      ['Contributor', 'First commit', 'Last commit', 'Commits', 'Active days', 'Files touched'],
      r.perAuthor.authors.map((a) => [
        a.displayName,
        isoDate(a.firstCommit),
        isoDate(a.lastCommit),
        num(a.commits),
        num(a.activeDays),
        num(a.filesTouched),
      ]),
    ),
  );
};

// --- footer ---------------------------------------------------------------

export const footer: SectionBuilder = () =>
  el(
    'footer',
    { class: 'page-footer' },
    el('p', {}, 'Generated by Commitography — MIT licensed.'),
    el('p', {}, 'No data left this machine. The tool reads the repository and writes this file; it makes no network requests.'),
  );

export const dashboardSections: SectionBuilder[] = [
  pulse,
  chronotype,
  rhythm,
  codeAge,
  hotspots,
  coupling,
  busFactor,
  messages,
  notables,
  contributors,
];
