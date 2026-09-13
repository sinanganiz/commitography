import type { Report } from '../types';

/**
 * A complete report in which every dashboard section has something to show.
 * Tests remove data from it to check that sections disappear.
 */
export function sampleReport(overrides: Partial<Report> = {}): Report {
  const hourHistogram = Array.from({ length: 24 }, (_, hour) => ({ 9: 5, 14: 3, 23: 2 })[hour] ?? 0);
  const hourWeekdayGrid = Array.from({ length: 7 }, (_, day) =>
    Array.from({ length: 24 }, (_, hour) => (day === 0 && hour === 9 ? 5 : day === 1 && hour === 23 ? 2 : day === 2 && hour === 14 ? 3 : 0)),
  );
  return {
    schemaVersion: 1,
    generatedAt: '2026-09-13T10:00:00Z',
    toolVersion: 'test',
    repository: {
      name: 'sample',
      defaultBranch: 'main',
      headCommit: '0123456789abcdef',
      firstCommit: '2025-01-06T09:00:00+01:00',
      lastCommit: '2025-03-10T14:00:00+01:00',
      ageDays: 63,
      commitsTotal: 10,
      commitsAnalyzed: 10,
      commitsExcluded: { merges: 0, bots: 0, total: 0 },
      contributors: 2,
      trackedFiles: 3,
      trackedLines: 120,
      isShallow: false,
    },
    temporal: {
      hourHistogram,
      weekdayHistogram: [3, 2, 1, 2, 1, 0, 1],
      hourWeekdayGrid,
      braveDeploys: 1,
      nightOwlRatio: 0.2,
      busiestDay: { date: '2025-01-06', count: 3 },
      longestStreak: { days: 3, startDate: '2025-01-06', endDate: '2025-01-08' },
      longestSilence: { days: 20, startDate: '2025-02-01', endDate: '2025-02-21' },
      commitsPerMonth: [
        { month: '2025-01', count: 5 },
        { month: '2025-02', count: 2 },
        { month: '2025-03', count: 3 },
      ],
      firstCommit: '2025-01-06T09:00:00+01:00',
      lastCommit: '2025-03-10T14:00:00+01:00',
    },
    code: {
      mostTouchedFiles: [
        { path: 'src/main.go', commits: 6, added: 80, deleted: 10, deletedFromHead: false },
        { path: 'docs/old.md', commits: 2, added: 5, deleted: 5, deletedFromHead: true },
      ],
      largestCommit: null,
      averageCommitSize: 12,
      medianCommitSize: 8,
      totalAdded: 120,
      totalDeleted: 20,
      oldestUntouchedFile: { path: 'LICENSE', lastModified: '2025-01-06T09:00:00+01:00' },
      fileTypeDistribution: [],
      codeAge: [{ year: 2025, lines: 100 }],
      survivingFromFirstYear: 1,
      codeAgeSampledFiles: 3,
      codeAgeTotalFiles: 3,
      trackedFiles: 3,
      trackedLines: 120,
    },
    messages: {
      typeDistribution: { feat: 6, fix: 4 },
      conventionalRatio: 0.9,
      lowConfidence: false,
      shortMessages: 1,
      longestSubject: { hash: 'abc', length: 40, subject: 'feat: add the first real feature for users' },
      averageSubjectLength: 24,
      emojiCommits: 0,
      topEmoji: [],
      revertCount: 0,
      typoFixCount: 1,
      topWords: [
        { word: 'feature', count: 3 },
        { word: 'parser', count: 2 },
      ],
    },
    social: {
      busFactor: 1,
      directoryBusFactor: [{ path: 'src', busFactor: 1, contributors: 2, commits: 8 }],
      coupling: [{ a: 'src/main.go', b: 'src/main_test.go', support: 5, confidence: 0.8, expected: true }],
      churn: [
        { path: 'src/main.go', maxCommitsInWindow: 5, windowStart: '2025-01-06T09:00:00+01:00', totalCommits: 6, added: 80, deleted: 10 },
      ],
      knowledgeConcentration: [{ path: 'src', largestShare: 0.7, contributors: 2, commits: 8 }],
    },
    notables: {
      bulkCommits: [],
      latestNightCommit: {
        hash: 'n1',
        subject: 'fix: late night repair',
        date: '2025-01-07T23:40:00+01:00',
        linesChanged: 4,
        added: 2,
        deleted: 2,
        files: 1,
      },
      earliestMorningCommit: null,
      weekendRatio: 0.1,
      holidayCommits: 0,
      firstCommitSubject: 'chore: initial commit',
      mergeCount: 0,
      timezoneSpread: [{ offsetMinutes: 60, commits: 10 }],
    },
    perAuthor: {
      authors: [
        {
          identityId: 'a1',
          displayName: 'Ada Lovelace',
          emails: null,
          commits: 6,
          added: 80,
          deleted: 10,
          filesTouched: 3,
          hourHistogram,
          firstCommit: '2025-01-06T09:00:00+01:00',
          lastCommit: '2025-03-10T14:00:00+01:00',
          activeDays: 4,
        },
      ],
    },
    warnings: [],
    ...overrides,
  };
}

/** Section IDs in reading order, for a report where every section has data. */
export const allSectionIds = [
  'pulse',
  'chronotype',
  'rhythm',
  'code-age',
  'hotspots',
  'coupling',
  'bus-factor',
  'messages',
  'notables',
  'contributors',
];

/** The ID and text of every rendered report section under root. */
export function renderedSections(root: ParentNode): { id: string; text: string }[] {
  return [...root.querySelectorAll('section[id]')].map((section) => ({
    id: section.id,
    text: section.textContent ?? '',
  }));
}

export function jsonResponse(body: unknown, status = 200): Response {
  return new Response(JSON.stringify(body), { status, headers: { 'Content-Type': 'application/json' } });
}
