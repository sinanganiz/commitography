// Stage identifiers mirror internal/analysis. Unknown stages are shown as-is so
// a newer server never renders an empty label.
const stageLabels: Record<string, string> = {
  preflight: 'Checking the repository',
  collecting: 'Reading history',
  identity: 'Resolving contributors',
  filtering: 'Filtering commits',
  temporal: 'Activity over time',
  code: 'Code and ownership',
  messages: 'Commit messages',
  social: 'Collaboration',
  notables: 'Notable moments',
  finalizing: 'Finishing the report',
};

export function stageLabel(stage: string): string {
  return stageLabels[stage] ?? stage;
}

/**
 * Returns a progress detail only when it carries a count, such as "2512 of
 * 2632 commits". The other details restate the stage ("code", "reading
 * history") or, for "analysis complete", arrive before the final consistency
 * check and would announce a result the job has not reached yet.
 */
export function countedDetail(detail: string): string {
  return /\d/.test(detail) ? detail : '';
}

/** Formats a duration as m:ss, or h:mm:ss from one hour. */
export function formatDuration(milliseconds: number): string {
  const total = Math.max(0, Math.floor(milliseconds / 1000));
  const hours = Math.floor(total / 3600);
  const minutes = Math.floor((total % 3600) / 60);
  const seconds = String(total % 60).padStart(2, '0');
  return hours > 0 ? `${hours}:${String(minutes).padStart(2, '0')}:${seconds}` : `${minutes}:${seconds}`;
}

/** A job timestamp in the reader's own locale and timezone. */
export function formatDateTime(iso: string): string {
  const date = new Date(iso);
  return Number.isNaN(date.getTime()) ? iso : date.toLocaleString(undefined, { dateStyle: 'medium', timeStyle: 'medium' });
}

export function plural(count: number, noun: string): string {
  return `${count.toLocaleString()} ${noun}${count === 1 ? '' : 's'}`;
}

/** Capitalizes a server message and ends it with a full stop. */
export function sentence(message: string): string {
  const trimmed = message.trim();
  if (!trimmed) return '';
  const capitalized = trimmed[0].toUpperCase() + trimmed.slice(1);
  return /[.!?]$/.test(capitalized) ? capitalized : `${capitalized}.`;
}
