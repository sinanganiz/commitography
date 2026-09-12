import type { ReactElement, ReactNode } from 'react';

import type { Report } from '../types';
import { AreaChart, BarChart, Heatmap, RadialHistogram, StrataChart } from './charts';
import { Figure } from './Figure';
import { isoDate, isoDateTime, num, offsetLabel, pct, shortPath } from './format';
import { H } from './heading';
import { Listing, Section, Stat, StatRow, Subhead, Subsection } from './parts';

const WEEKDAYS = ['Mon', 'Tue', 'Wed', 'Thu', 'Fri', 'Sat', 'Sun'];

/** A section is omitted entirely when it has nothing to say. Rendering null is
 *  how a section declines to appear. */
export type ReportSection = (props: { report: Report }) => ReactElement | null;

// --- pulse ----------------------------------------------------------------

export function Pulse({ report }: { report: Report }): ReactElement | null {
  const series = report.temporal.commitsPerMonth;
  if (series.length === 0) return null;

  return (
    <Section id="pulse" title="Pulse">
      <Subhead>Commits per month across the whole history, with quiet months left visible.</Subhead>
      <Figure
        title="Commits per month"
        desc={`Commit counts for each of the ${series.length} months between ${series[0].month} and ${series[series.length - 1].month}.`}
        columns={['Month', 'Commits']}
        rows={series.map((m) => ({ label: m.month, values: [m.count] }))}
      >
        <AreaChart labels={series.map((m) => m.month)} values={series.map((m) => m.count)} />
      </Figure>
    </Section>
  );
}

// --- chronotype -----------------------------------------------------------

export function Chronotype({ report }: { report: Report }): ReactElement | null {
  const t = report.temporal;
  if (t.hourHistogram.every((v) => v === 0)) return null;

  return (
    <Section id="chronotype" title="Chronotype">
      <Subhead>When this repository is awake, in each author's own local time.</Subhead>
      <StatRow>
        <Stat value={pct(t.nightOwlRatio)} label="commits at night" note="between 22:00 and 06:00" />
        <Stat value={num(t.braveDeploys)} label="brave deploys" note="Friday, after 17:00" />
      </StatRow>
      <Figure
        title="Commits by hour of day"
        desc="A twenty-four segment ring; each segment extends outward in proportion to the commits made in that local hour."
        columns={['Hour', 'Commits']}
        rows={t.hourHistogram.map((v, hour) => ({ label: `${String(hour).padStart(2, '0')}:00`, values: [v] }))}
      >
        <RadialHistogram values={t.hourHistogram} />
      </Figure>
      <Figure
        title="Commits by weekday and hour"
        desc="A seven by twenty-four grid; each cell is shaded in five steps according to how many commits fall in that weekday and hour."
        columns={['Weekday', ...Array.from({ length: 24 }, (_, h) => String(h).padStart(2, '0'))]}
        rows={t.hourWeekdayGrid.map((row, day) => ({ label: WEEKDAYS[day], values: row }))}
      >
        <Heatmap grid={t.hourWeekdayGrid} weekdays={WEEKDAYS} />
      </Figure>
    </Section>
  );
}

// --- rhythm ---------------------------------------------------------------

export function Rhythm({ report }: { report: Report }): ReactElement | null {
  const t = report.temporal;
  if (t.weekdayHistogram.every((v) => v === 0)) return null;

  return (
    <Section id="rhythm" title="Rhythm">
      <Subhead>The working week, and the longest stretches of noise and quiet.</Subhead>
      <Figure
        title="Commits by weekday"
        desc="A bar per weekday, Monday through Sunday, with the count printed beside each bar."
        columns={['Weekday', 'Commits']}
        rows={WEEKDAYS.map((d, i) => ({ label: d, values: [t.weekdayHistogram[i]] }))}
      >
        <BarChart labels={WEEKDAYS} values={t.weekdayHistogram} unit="commits" />
      </Figure>
      <StatRow>
        {t.longestStreak ? (
          <Stat
            value={`${num(t.longestStreak.days)} days`}
            label="longest streak"
            note={`${t.longestStreak.startDate} – ${t.longestStreak.endDate}`}
          />
        ) : (
          <Stat value="—" label="longest streak" />
        )}
        {t.longestSilence ? (
          <Stat
            value={`${num(t.longestSilence.days)} days`}
            label="longest silence"
            note={`${t.longestSilence.startDate} – ${t.longestSilence.endDate}`}
          />
        ) : (
          <Stat value="—" label="longest silence" />
        )}
        {t.busiestDay ? (
          <Stat value={num(t.busiestDay.count)} label="commits in one day" note={t.busiestDay.date} />
        ) : (
          <Stat value="—" label="busiest day" />
        )}
      </StatRow>
    </Section>
  );
}

// --- code age -------------------------------------------------------------

export function CodeAge({ report }: { report: Report }): ReactElement | null {
  const c = report.code;
  if (c.codeAge.length === 0) return null;

  const total = c.codeAge.reduce((sum, a) => sum + a.lines, 0);

  return (
    <Section id="code-age" title="Code age">
      <Subhead>
        {`Which year each surviving line was last written in, sampled from ${num(c.codeAgeSampledFiles)} of ${num(c.codeAgeTotalFiles)} text files.`}
      </Subhead>
      {c.survivingFromFirstYear !== null ? (
        <StatRow>
          <Stat
            value={pct(c.survivingFromFirstYear)}
            label="of sampled lines"
            note="date from the repository's first year"
          />
        </StatRow>
      ) : null}
      <Figure
        title="Surviving lines by year last modified"
        desc={`A single band divided into ${c.codeAge.length} strata, one per year, sized by its share of the ${num(total)} sampled lines.`}
        columns={['Year', 'Lines', 'Share']}
        rows={c.codeAge.map((a) => ({ label: String(a.year), values: [a.lines, pct(a.lines / (total || 1))] }))}
      >
        <StrataChart years={c.codeAge.map((a) => a.year)} lines={c.codeAge.map((a) => a.lines)} />
      </Figure>
    </Section>
  );
}

// --- hotspots -------------------------------------------------------------

function PathName({ path, max, children }: { path: string; max?: number; children?: ReactNode }): ReactElement {
  return (
    <span className="path" title={path}>
      {shortPath(path, max)}
      {children}
    </span>
  );
}

export function Hotspots({ report }: { report: Report }): ReactElement | null {
  const c = report.code;
  const churn = report.social.churn;
  if (c.mostTouchedFiles.length === 0 && churn.length === 0) return null;

  return (
    <Section id="hotspots" title="Hotspots">
      <Subhead>The files this repository keeps coming back to.</Subhead>
      <StatRow>
        <Stat value={num(c.totalAdded)} label="lines added" />
        <Stat value={num(c.totalDeleted)} label="lines deleted" />
        <Stat value={String(c.averageCommitSize)} label="average commit" note={`median ${c.medianCommitSize}`} />
      </StatRow>

      {c.mostTouchedFiles.length > 0 ? (
        <>
          <Subsection>Most modified files</Subsection>
          <Listing
            columns={['File', 'Commits', 'Added', 'Deleted']}
            rows={c.mostTouchedFiles.map((f) => [
              <PathName path={f.path}>{f.deletedFromHead ? <span className="tag">removed</span> : null}</PathName>,
              num(f.commits),
              num(f.added),
              num(f.deleted),
            ])}
          />
        </>
      ) : null}

      {churn.length > 0 ? (
        <>
          <Subsection>Churn hotspots</Subsection>
          <Subhead>Files rewritten at least five times inside a single 30-day window.</Subhead>
          <Listing
            columns={['File', 'Peak in 30 days', 'Window from', 'Total commits']}
            rows={churn.map((h) => [
              <PathName path={h.path} />,
              num(h.maxCommitsInWindow),
              isoDate(h.windowStart),
              num(h.totalCommits),
            ])}
          />
        </>
      ) : null}

      {c.oldestUntouchedFile ? (
        <p className="aside">
          Least recently modified file still in the tree: <code>{c.oldestUntouchedFile.path}</code>
          {`, last changed ${isoDate(c.oldestUntouchedFile.lastModified)}.`}
        </p>
      ) : null}
    </Section>
  );
}

// --- coupling -------------------------------------------------------------

export function Coupling({ report }: { report: Report }): ReactElement | null {
  const pairs = report.social.coupling;
  if (pairs.length === 0) return null;

  const maxSupport = Math.max(...pairs.map((p) => p.support));

  return (
    <Section id="coupling" title="Change coupling">
      <Subhead>
        File pairs that keep changing in the same commit. Pairs sharing a name, such as a file and its test, are marked
        as expected.
      </Subhead>
      <ol className="coupling-list">
        {pairs.map((p, index) => (
          <li key={index} className={p.expected ? 'coupling-item coupling-expected' : 'coupling-item'}>
            <div className="coupling-paths">
              <code title={p.a}>{shortPath(p.a, 40)}</code>
              <span className="coupling-join">changes with</span>
              <code title={p.b}>{shortPath(p.b, 40)}</code>
              {p.expected ? <span className="tag">expected pair</span> : null}
            </div>
            <div className="coupling-meter">
              <svg viewBox="0 0 100 6" className="meter" aria-hidden="true">
                <rect className="meter-track" x={0} y={0} width={100} height={6} rx={3} />
                <rect className="meter-value" x={0} y={0} width={(p.support / maxSupport) * 100} height={6} rx={3} />
              </svg>
              <span className="coupling-numbers">
                {`${num(p.support)} commits together, ${pct(p.confidence, 0)} confidence`}
              </span>
            </div>
          </li>
        ))}
      </ol>
    </Section>
  );
}

// --- bus factor -----------------------------------------------------------

export function BusFactor({ report }: { report: Report }): ReactElement | null {
  const s = report.social;
  if (s.busFactor === 0) return null;

  const namesTopContributor = s.knowledgeConcentration.length > 0 && !!s.knowledgeConcentration[0].topContributor;

  return (
    <Section id="bus-factor" title="Bus factor">
      <Subhead>
        The smallest number of contributors accounting for at least half the commits. It measures concentration of
        knowledge, not anyone’s output.
      </Subhead>
      <StatRow>
        <Stat value={num(s.busFactor)} label="repository bus factor" />
      </StatRow>

      {s.directoryBusFactor.length > 0 ? (
        <Listing
          columns={['Directory', 'Bus factor', 'Contributors', 'Commits']}
          rows={s.directoryBusFactor.map((d) => [<code>{d.path}</code>, num(d.busFactor), num(d.contributors), num(d.commits)])}
        />
      ) : null}

      {s.knowledgeConcentration.length > 0 ? (
        <>
          <Subsection>Knowledge concentration</Subsection>
          <Subhead>The share of each top-level directory held by its single largest contributor.</Subhead>
          <Listing
            columns={['Directory', 'Largest share', 'Contributors', ...(namesTopContributor ? ['Top contributor'] : [])]}
            rows={s.knowledgeConcentration.map((k) => [
              <code>{k.path}</code>,
              pct(k.largestShare, 0),
              num(k.contributors),
              ...(k.topContributor ? [k.topContributor] : []),
            ])}
          />
        </>
      ) : null}
    </Section>
  );
}

// --- messages -------------------------------------------------------------

export function Messages({ report }: { report: Report }): ReactElement | null {
  const m = report.messages;
  const types = Object.entries(m.typeDistribution).sort((a, b) => b[1] - a[1]);
  if (types.length === 0) return null;

  const maxWord = m.topWords.length > 0 ? m.topWords[0].count : 1;

  return (
    <Section id="messages" title="Messages">
      <Subhead>
        {m.lowConfidence
          ? `Only ${pct(m.conventionalRatio, 0)} of subjects follow Conventional Commits, so the classification below is a low-confidence guess.`
          : `${pct(m.conventionalRatio, 0)} of subjects follow Conventional Commits.`}
      </Subhead>
      {m.lowConfidence ? <p className="confidence-flag">Low confidence classification</p> : null}
      <Figure
        title="Commit type distribution"
        desc={`Commit counts across ${types.length} categories, from the Conventional Commits prefix where present and from keyword rules otherwise.`}
        columns={['Type', 'Commits']}
        rows={types.map(([type, count]) => ({ label: type, values: [count] }))}
      >
        <BarChart labels={types.map(([type]) => type)} values={types.map(([, count]) => count)} unit="commits" />
      </Figure>
      <StatRow>
        <Stat value={num(m.shortMessages)} label="placeholder messages" note={'"wip", "fix", "..."'} />
        <Stat value={num(m.revertCount)} label="reverts" />
        <Stat value={num(m.typoFixCount)} label="typo fixes" />
        <Stat value={num(m.emojiCommits)} label="commits with emoji" />
        <Stat value={String(m.averageSubjectLength)} label="average subject length" note="characters" />
      </StatRow>

      {m.topWords.length > 0 ? (
        <>
          <Subsection>What gets talked about</Subsection>
          <ul className="word-cloud">
            {m.topWords.map((w, index) => (
              <li
                key={index}
                className="word"
                style={{ fontSize: `${(0.85 + (w.count / maxWord) * 1.5).toFixed(2)}rem` }}
                title={`${w.word}: ${w.count}`}
              >
                {w.word}
                <span className="word-count">{String(w.count)}</span>
              </li>
            ))}
          </ul>
        </>
      ) : null}

      {m.topEmoji.length > 0 ? (
        <p className="aside">
          Most used emoji:{' '}
          {m.topEmoji.map((e, index) => (
            <span key={index} className="emoji-chip">{`${e.emoji} ${num(e.count)}`}</span>
          ))}
        </p>
      ) : null}

      {m.longestSubject ? (
        <p className="aside">
          {`Longest subject, at ${num(m.longestSubject.length)} characters: `}
          <q>{m.longestSubject.subject}</q>
        </p>
      ) : null}
    </Section>
  );
}

// --- notables -------------------------------------------------------------

function Card({ title, children }: { title: string; children: ReactNode }): ReactElement {
  return (
    <div className="card">
      <H depth={2} className="card-title">
        {title}
      </H>
      {children}
    </div>
  );
}

export function Notables({ report }: { report: Report }): ReactElement | null {
  const n = report.notables;
  const cards: ReactElement[] = [];

  cards.push(
    <Card key="weekend" title="Weekend work">
      <p className="card-figure">{pct(n.weekendRatio)}</p>
    </Card>,
    <Card key="merges" title="Merge commits">
      <p className="card-figure">{num(n.mergeCount)}</p>
    </Card>,
  );

  if (n.holidayCommits > 0) {
    cards.push(
      <Card key="holiday" title="Holiday commits">
        <p className="card-figure">{num(n.holidayCommits)}</p>
        <p>on 25 December or 1 January</p>
      </Card>,
    );
  }

  if (n.latestNightCommit) {
    cards.push(
      <Card key="night" title="Deepest into the night">
        <p className="card-figure">{isoDateTime(n.latestNightCommit.date).slice(-5)}</p>
        <p>{n.latestNightCommit.subject}</p>
        <p className="card-meta">{isoDate(n.latestNightCommit.date)}</p>
      </Card>,
    );
  }

  if (n.earliestMorningCommit) {
    cards.push(
      <Card key="morning" title="Earliest start">
        <p className="card-figure">{isoDateTime(n.earliestMorningCommit.date).slice(-5)}</p>
        <p>{n.earliestMorningCommit.subject}</p>
        <p className="card-meta">{isoDate(n.earliestMorningCommit.date)}</p>
      </Card>,
    );
  }

  if (n.firstCommitSubject) {
    cards.push(
      <Card key="first" title="It began with">
        <p className="card-quote">{n.firstCommitSubject}</p>
      </Card>,
    );
  }

  if (n.timezoneSpread.length > 0) {
    cards.push(
      <Card key="timezones" title="Timezones">
        <p className="card-figure">{num(n.timezoneSpread.length)}</p>
        <ul className="card-list">
          {n.timezoneSpread.map((z, index) => (
            <li key={index}>{`${offsetLabel(z.offsetMinutes)} — ${num(z.commits)}`}</li>
          ))}
        </ul>
      </Card>,
    );
  }

  if (n.bulkCommits.length > 0) {
    cards.push(
      <Card key="bulk" title="Bulk commits">
        <p>Left out of line metrics because one import would otherwise define every distribution.</p>
        <ul className="card-list">
          {n.bulkCommits.map((b, index) => (
            <li key={index}>{`${isoDate(b.date)} — ${num(b.linesChanged)} lines — ${b.subject}`}</li>
          ))}
        </ul>
      </Card>,
    );
  }

  if (cards.length === 0) return null;
  return (
    <Section id="notables" title="Notables">
      <div className="card-grid">{cards}</div>
    </Section>
  );
}

// --- contributors ---------------------------------------------------------

export function Contributors({ report }: { report: Report }): ReactElement | null {
  if (!report.perAuthor || report.perAuthor.authors.length === 0) return null;

  return (
    <Section id="contributors" title="Contributors">
      <Subhead>Listed in order of first commit, not by volume. These figures describe participation, not performance.</Subhead>
      <Listing
        columns={['Contributor', 'First commit', 'Last commit', 'Commits', 'Active days', 'Files touched']}
        rows={report.perAuthor.authors.map((a) => [
          a.displayName,
          isoDate(a.firstCommit),
          isoDate(a.lastCommit),
          num(a.commits),
          num(a.activeDays),
          num(a.filesTouched),
        ])}
      />
    </Section>
  );
}

/** Dashboard sections in reading order. */
export const dashboardSections: ReportSection[] = [
  Pulse,
  Chronotype,
  Rhythm,
  CodeAge,
  Hotspots,
  Coupling,
  BusFactor,
  Messages,
  Notables,
  Contributors,
];
