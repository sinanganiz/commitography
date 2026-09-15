import type { Report } from '../types';

const DATA_ID = 'commitography-data';

/** Reads the report the Go renderer inlined into a static page. */
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

/** Reads the Wrapped year and the preceding year's commit count, which the
 *  renderer omits rather than zeroes when that year was empty. */
export function readWrappedOptions(): { year: number; previousYearCommits: number | null } {
  const dataset = document.documentElement.dataset;
  const year = Number(dataset.year || '0');
  const previousRaw = dataset.previousYearCommits;
  const previousYearCommits = previousRaw === undefined || previousRaw === '' ? null : Number(previousRaw);
  return { year, previousYearCommits };
}
