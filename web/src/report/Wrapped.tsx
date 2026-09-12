import { useEffect, useRef } from 'react';
import type { KeyboardEvent, ReactElement, TouchEvent } from 'react';

import type { Report } from '../types';
import { num, pct, shortPath } from './format';

/**
 * Repo Wrapped: a vertical sequence of full-viewport cards, one statistic each.
 *
 * Navigation is scroll, arrow keys and swipe. Each card exports itself to a PNG
 * entirely in the browser — the page never talks to a network, so the export
 * has to be drawn with canvas rather than requested from a service.
 */

interface Card {
  kicker: string;
  headline: string;
  detail?: string;
  footnote?: string;
}

function cardsFor(r: Report, year: number, previousYearCommits: number | null): Card[] {
  const t = r.temporal;
  const cards: Card[] = [];

  const delta =
    previousYearCommits && previousYearCommits > 0
      ? `${r.repository.commitsAnalyzed >= previousYearCommits ? '+' : ''}${(
          ((r.repository.commitsAnalyzed - previousYearCommits) / previousYearCommits) *
          100
        ).toFixed(0)}% on ${year - 1}`
      : undefined;

  cards.push({
    kicker: `${year} in commits`,
    headline: num(r.repository.commitsAnalyzed),
    detail: 'commits',
    ...(delta ? { footnote: delta } : {}),
  });

  const peakHour = t.hourHistogram.indexOf(Math.max(...t.hourHistogram));
  cards.push({
    kicker: 'Your chronotype',
    headline: `${String(peakHour).padStart(2, '0')}:00`,
    detail: 'the hour this repository was most alive',
    footnote: `${pct(t.nightOwlRatio, 0)} of the year's commits landed between 22:00 and 06:00`,
  });

  if (t.busiestDay) {
    cards.push({
      kicker: 'The busiest day',
      headline: t.busiestDay.date,
      detail: `${num(t.busiestDay.count)} commits in one day`,
    });
  }

  if (t.longestStreak) {
    cards.push({
      kicker: 'The longest streak',
      headline: `${num(t.longestStreak.days)} days`,
      detail: 'in a row with at least one commit',
      footnote: `${t.longestStreak.startDate} to ${t.longestStreak.endDate}`,
    });
  }

  const topFile = r.code.mostTouchedFiles[0];
  if (topFile) {
    cards.push({
      kicker: 'The file you could not leave alone',
      headline: shortPath(topFile.path, 32),
      detail: `${num(topFile.commits)} commits touched it`,
    });
  }

  const types = Object.entries(r.messages.typeDistribution).sort((a, b) => b[1] - a[1]);
  if (types.length > 0) {
    cards.push({
      kicker: 'Mostly, you were doing',
      headline: types[0][0],
      detail: `${num(types[0][1])} commits`,
      ...(r.messages.lowConfidence ? { footnote: 'Classified by keyword; treat as a rough guide.' } : {}),
    });
  }

  const m = r.messages;
  cards.push(
    m.typoFixCount >= m.shortMessages
      ? {
          kicker: 'Attention to detail',
          headline: num(m.typoFixCount),
          detail: 'commits fixed a typo',
        }
      : {
          kicker: 'Said with feeling',
          headline: num(m.shortMessages),
          detail: 'commits said little more than "wip"',
        },
  );

  cards.push({
    kicker: `${r.repository.name}, ${year}`,
    headline: 'That was the year',
    detail: `${num(r.repository.commitsAnalyzed)} commits, ${num(r.repository.contributors)} contributors, ${num(r.code.totalAdded)} lines added`,
    footnote: 'Generated locally by Commitography. Nothing left this machine.',
  });

  return cards;
}

/** Draws one card onto a canvas and hands the viewer a PNG. */
function exportCard(card: Card, index: number, repoName: string): void {
  const scale = 2;
  const width = 540;
  const height = 960;
  const canvas = document.createElement('canvas');
  canvas.width = width * scale;
  canvas.height = height * scale;

  const ctx = canvas.getContext('2d');
  if (!ctx) return;
  ctx.scale(scale, scale);

  const styles = getComputedStyle(document.documentElement);
  const bg = styles.getPropertyValue('--wrapped-bg').trim() || '#141018';
  const fg = styles.getPropertyValue('--wrapped-fg').trim() || '#f5f1ea';
  const accent = styles.getPropertyValue('--wrapped-accent').trim() || '#e8b04b';

  ctx.fillStyle = bg;
  ctx.fillRect(0, 0, width, height);

  const font = (size: number, weight = '400') =>
    `${weight} ${size}px ui-sans-serif, system-ui, -apple-system, "Segoe UI", Roboto, sans-serif`;

  ctx.textAlign = 'center';

  ctx.fillStyle = accent;
  ctx.font = font(20, '600');
  wrapText(ctx, card.kicker, width / 2, 300, width - 80, 28);

  ctx.fillStyle = fg;
  ctx.font = font(card.headline.length > 14 ? 42 : 68, '700');
  wrapText(ctx, card.headline, width / 2, 400, width - 80, 64);

  if (card.detail) {
    ctx.font = font(22);
    ctx.globalAlpha = 0.85;
    wrapText(ctx, card.detail, width / 2, 500, width - 100, 30);
    ctx.globalAlpha = 1;
  }

  if (card.footnote) {
    ctx.font = font(16);
    ctx.globalAlpha = 0.6;
    wrapText(ctx, card.footnote, width / 2, 580, width - 120, 22);
    ctx.globalAlpha = 1;
  }

  ctx.font = font(14);
  ctx.globalAlpha = 0.5;
  ctx.fillText(`${repoName} · commitography`, width / 2, height - 48);
  ctx.globalAlpha = 1;

  canvas.toBlob((blob) => {
    if (!blob) return;
    const url = URL.createObjectURL(blob);
    const link = document.createElement('a');
    link.href = url;
    link.download = `${repoName}-wrapped-${index + 1}.png`;
    link.click();
    URL.revokeObjectURL(url);
  }, 'image/png');
}

function wrapText(
  ctx: CanvasRenderingContext2D,
  value: string,
  x: number,
  y: number,
  maxWidth: number,
  lineHeight: number,
): void {
  const words = value.split(' ');
  let line = '';
  let cursor = y;
  for (const word of words) {
    const candidate = line ? `${line} ${word}` : word;
    if (ctx.measureText(candidate).width > maxWidth && line) {
      ctx.fillText(line, x, cursor);
      line = word;
      cursor += lineHeight;
    } else {
      line = candidate;
    }
  }
  if (line) ctx.fillText(line, x, cursor);
}

interface WrappedProps {
  report: Report;
  year: number;
  previousYearCommits: number | null;
}

export function Wrapped({ report, year, previousYearCommits }: WrappedProps): ReactElement {
  const cards = cardsFor(report, year, previousYearCommits);
  const repoName = report.repository.name || 'repository';
  const sections = useRef<(HTMLElement | null)[]>([]);
  const current = useRef(0);
  const touchStart = useRef(0);

  useEffect(() => {
    const root = document.documentElement;
    root.classList.add('wrapped-mode');
    return () => root.classList.remove('wrapped-mode');
  }, []);

  // Keep the current card honest when the reader simply scrolls.
  useEffect(() => {
    const nodes = sections.current.filter((node): node is HTMLElement => node !== null);
    const observer = new IntersectionObserver(
      (entries) => {
        for (const entry of entries) {
          if (entry.isIntersecting) {
            const index = nodes.indexOf(entry.target as HTMLElement);
            if (index >= 0) current.current = index;
          }
        }
      },
      { threshold: 0.6 },
    );
    nodes.forEach((node) => observer.observe(node));
    return () => observer.disconnect();
  }, [cards.length]);

  const go = (next: number) => {
    current.current = Math.max(0, Math.min(cards.length - 1, next));
    const node = sections.current[current.current];
    node?.scrollIntoView({ behavior: 'smooth', block: 'start' });
    node?.focus({ preventScroll: true });
  };

  const onKeyDown = (event: KeyboardEvent<HTMLElement>) => {
    switch (event.key) {
      case 'ArrowDown':
      case 'PageDown':
      case ' ':
        event.preventDefault();
        go(current.current + 1);
        break;
      case 'ArrowUp':
      case 'PageUp':
        event.preventDefault();
        go(current.current - 1);
        break;
      case 'Home':
        event.preventDefault();
        go(0);
        break;
      case 'End':
        event.preventDefault();
        go(cards.length - 1);
        break;
      default:
        break;
    }
  };

  // Swipe, for the phone this is most likely to be read on.
  const onTouchStart = (event: TouchEvent<HTMLElement>) => {
    touchStart.current = event.changedTouches[0].clientY;
  };
  const onTouchEnd = (event: TouchEvent<HTMLElement>) => {
    const delta = touchStart.current - event.changedTouches[0].clientY;
    if (Math.abs(delta) > 60) go(current.current + (delta > 0 ? 1 : -1));
  };

  return (
    <main
      className="cg-report wrapped-deck"
      tabIndex={0}
      aria-label={`${report.repository.name} wrapped ${year}`}
      onKeyDown={onKeyDown}
      onTouchStart={onTouchStart}
      onTouchEnd={onTouchEnd}
    >
      {cards.map((card, index) => (
        <section
          key={index}
          ref={(node) => {
            sections.current[index] = node;
          }}
          className="wrapped-card"
          aria-label={card.kicker}
          tabIndex={-1}
        >
          <p className="wrapped-kicker">{card.kicker}</p>
          <p className="wrapped-headline">{card.headline}</p>
          {card.detail ? <p className="wrapped-detail">{card.detail}</p> : null}
          {card.footnote ? <p className="wrapped-footnote">{card.footnote}</p> : null}
          <p className="wrapped-progress">{`${index + 1} of ${cards.length}`}</p>
          <button type="button" className="wrapped-export" onClick={() => exportCard(card, index, repoName)}>
            Export as image
          </button>
        </section>
      ))}
    </main>
  );
}
