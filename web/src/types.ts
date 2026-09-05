// Mirrors internal/aggregate. Kept in sync by hand: the report is a documented
// stable artifact, so its shape changes rarely and never within a schema
// version.

export interface ExclusionBreakdown {
  merges: number;
  bots: number;
  total: number;
}

export interface RepositorySummary {
  name: string;
  defaultBranch: string;
  headCommit: string;
  firstCommit: string | null;
  lastCommit: string | null;
  ageDays: number;
  commitsTotal: number;
  commitsAnalyzed: number;
  commitsExcluded: ExclusionBreakdown;
  contributors: number;
  trackedFiles: number;
  trackedLines: number;
  isShallow: boolean;
}

export interface DateCount {
  date: string;
  count: number;
}

export interface Span {
  days: number;
  startDate: string;
  endDate: string;
}

export interface MonthCount {
  month: string;
  count: number;
}

export interface TemporalMetrics {
  hourHistogram: number[];
  weekdayHistogram: number[];
  hourWeekdayGrid: number[][];
  braveDeploys: number;
  nightOwlRatio: number;
  busiestDay: DateCount | null;
  longestStreak: Span | null;
  longestSilence: Span | null;
  commitsPerMonth: MonthCount[];
  firstCommit: string | null;
  lastCommit: string | null;
}

export interface TouchedFile {
  path: string;
  commits: number;
  added: number;
  deleted: number;
  deletedFromHead: boolean;
}

export interface CommitRef {
  hash: string;
  subject: string;
  date: string;
  linesChanged: number;
  added: number;
  deleted: number;
  files: number;
}

export interface FileAge {
  path: string;
  lastModified: string;
}

export interface FileTypeShare {
  extension: string;
  changes: number;
  added: number;
  deleted: number;
}

export interface YearLines {
  year: number;
  lines: number;
}

export interface CodeMetrics {
  mostTouchedFiles: TouchedFile[];
  largestCommit: CommitRef | null;
  averageCommitSize: number;
  medianCommitSize: number;
  totalAdded: number;
  totalDeleted: number;
  oldestUntouchedFile: FileAge | null;
  fileTypeDistribution: FileTypeShare[];
  codeAge: YearLines[];
  survivingFromFirstYear: number | null;
  codeAgeSampledFiles: number;
  codeAgeTotalFiles: number;
  trackedFiles: number;
  trackedLines: number;
}

export interface LongestSubject {
  hash: string;
  length: number;
  subject: string;
}

export interface WordCount {
  word: string;
  count: number;
}

export interface EmojiCount {
  emoji: string;
  count: number;
}

export interface MessageMetrics {
  typeDistribution: Record<string, number>;
  conventionalRatio: number;
  lowConfidence: boolean;
  shortMessages: number;
  longestSubject: LongestSubject | null;
  averageSubjectLength: number;
  emojiCommits: number;
  topEmoji: EmojiCount[];
  revertCount: number;
  typoFixCount: number;
  topWords: WordCount[];
}

export interface DirectoryBusFactor {
  path: string;
  busFactor: number;
  contributors: number;
  commits: number;
}

export interface CoupledPair {
  a: string;
  b: string;
  support: number;
  confidence: number;
  expected: boolean;
}

export interface ChurnHotspot {
  path: string;
  maxCommitsInWindow: number;
  windowStart: string;
  totalCommits: number;
  added: number;
  deleted: number;
}

export interface KnowledgeShare {
  path: string;
  largestShare: number;
  contributors: number;
  commits: number;
  topContributor?: string;
}

export interface SocialMetrics {
  busFactor: number;
  directoryBusFactor: DirectoryBusFactor[];
  coupling: CoupledPair[];
  churn: ChurnHotspot[];
  knowledgeConcentration: KnowledgeShare[];
}

export interface BulkCommit {
  hash: string;
  subject: string;
  date: string;
  linesChanged: number;
}

export interface TimezoneShare {
  offsetMinutes: number;
  commits: number;
}

export interface Notables {
  bulkCommits: BulkCommit[];
  latestNightCommit: CommitRef | null;
  earliestMorningCommit: CommitRef | null;
  weekendRatio: number;
  holidayCommits: number;
  firstCommitSubject: string | null;
  mergeCount: number;
  timezoneSpread: TimezoneShare[];
}

export interface AuthorSummary {
  identityId: string;
  displayName: string;
  emails: string[] | null;
  commits: number;
  added: number;
  deleted: number;
  filesTouched: number;
  hourHistogram: number[];
  firstCommit: string;
  lastCommit: string;
  activeDays: number;
}

export interface PerAuthor {
  authors: AuthorSummary[];
}

export interface Report {
  schemaVersion: number;
  generatedAt: string;
  toolVersion: string;
  repository: RepositorySummary;
  temporal: TemporalMetrics;
  code: CodeMetrics;
  messages: MessageMetrics;
  social: SocialMetrics;
  notables: Notables;
  perAuthor?: PerAuthor;
  warnings: string[];
}

/** Rendering mode, chosen by the Go renderer via a data attribute. */
export type Mode = 'dashboard' | 'wrapped';
