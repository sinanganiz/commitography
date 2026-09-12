/** Formats an integer with thin thousands separators. */
export function num(value: number): string {
  return value.toLocaleString('en-US');
}

export function pct(ratio: number, digits = 1): string {
  return `${(ratio * 100).toFixed(digits)}%`;
}

/** Renders an ISO timestamp as a plain calendar date, keeping the original
 *  offset rather than shifting it into the reader's timezone. */
export function isoDate(iso: string | null): string {
  if (!iso) return '—';
  const match = /^(\d{4})-(\d{2})-(\d{2})/.exec(iso);
  return match ? `${match[1]}-${match[2]}-${match[3]}` : iso;
}

/** Renders an ISO timestamp as date plus local wall-clock time, again without
 *  converting away from the author's own offset. */
export function isoDateTime(iso: string | null): string {
  if (!iso) return '—';
  const match = /^(\d{4}-\d{2}-\d{2})T(\d{2}:\d{2})/.exec(iso);
  return match ? `${match[1]} ${match[2]}` : iso;
}

export function offsetLabel(minutes: number): string {
  const sign = minutes < 0 ? '-' : '+';
  const abs = Math.abs(minutes);
  const hh = String(Math.floor(abs / 60)).padStart(2, '0');
  const mm = String(abs % 60).padStart(2, '0');
  return `UTC${sign}${hh}:${mm}`;
}

/** Shortens a path from the left, so the filename stays readable. */
export function shortPath(path: string, max = 48): string {
  if (path.length <= max) return path;
  return `…${path.slice(path.length - max + 1)}`;
}
