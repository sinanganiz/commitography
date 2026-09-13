// Builds the repositories the browser audit analyzes, so it depends on nothing
// outside the checkout.
import { execFileSync } from 'node:child_process';
import { mkdirSync, readdirSync, rmSync, writeFileSync } from 'node:fs';
import { join } from 'node:path';

const identity = [
  '-c', 'user.name=Audit',
  '-c', 'user.email=audit@example.com',
  '-c', 'commit.gpgsign=false',
  '-c', 'core.autocrlf=false',
];

export function git(cwd, ...args) {
  return execFileSync('git', [...identity, ...args], { cwd, encoding: 'utf8', stdio: ['pipe', 'pipe', 'pipe'] }).trim();
}

function init(dir) {
  mkdirSync(dir, { recursive: true });
  git(dir, 'init', '-q', '-b', 'main');
}

function commit(dir, message, date) {
  execFileSync('git', [...identity, 'commit', '-q', '-m', message], {
    cwd: dir,
    env: { ...process.env, GIT_AUTHOR_DATE: date, GIT_COMMITTER_DATE: date },
    stdio: 'ignore',
  });
}

/** A dozen commits in January 2025, enough for a dashboard and a Wrapped year. */
export function smallRepository(dir) {
  init(dir);
  for (let i = 0; i < 12; i++) {
    writeFileSync(join(dir, `file-${i % 3}.txt`), `line ${i}\n`);
    git(dir, 'add', '-A');
    const day = String(1 + i).padStart(2, '0');
    const hour = String(9 + (i % 8)).padStart(2, '0');
    commit(dir, `feat: change ${i}`, `2025-01-${day}T${hour}:00:00+01:00`);
  }
}

/**
 * A long history written with git fast-import, so an analysis runs for several
 * seconds: long enough to cancel it, or to change the repository under it.
 */
export function largeRepository(dir, { commits = 15000, files = 400 } = {}) {
  init(dir);
  const start = Date.parse('2020-01-01T00:00:00Z') / 1000;
  const parts = [];
  for (let i = 1; i <= commits; i++) {
    const author = `Dev ${i % 7} <dev${i % 7}@example.com> ${start + i * 3600} +0100`;
    const message = `feat: update module ${i % files}\n`;
    const content = `module ${i % files}\nrevision ${i}\n`;
    parts.push(
      `commit refs/heads/main\nmark :${i}\nauthor ${author}\ncommitter ${author}\n` +
        `data ${Buffer.byteLength(message)}\n${message}` +
        (i > 1 ? `from :${i - 1}\n` : '') +
        `M 100644 inline src/module-${i % files}.txt\ndata ${Buffer.byteLength(content)}\n${content}\n`,
    );
  }
  execFileSync('git', ['fast-import', '--quiet'], { cwd: dir, input: parts.join(''), stdio: ['pipe', 'ignore', 'pipe'] });
  git(dir, 'reset', '-q', '--hard', 'main');
}

/** A repository whose configuration is not valid YAML: the job is rejected. */
export function badConfigRepository(dir) {
  init(dir);
  writeFileSync(join(dir, 'a.txt'), 'hello\n');
  git(dir, 'add', '-A');
  commit(dir, 'chore: initial commit', '2025-01-01T09:00:00+01:00');
  writeFileSync(join(dir, '.commitography.yml'), 'exclude_paths: [unclosed\n');
}

/** A repository missing an object its history needs: the analysis fails. */
export function brokenRepository(dir) {
  init(dir);
  writeFileSync(join(dir, 'a.txt'), 'one\n');
  git(dir, 'add', '-A');
  commit(dir, 'feat: one', '2025-01-01T09:00:00+01:00');
  writeFileSync(join(dir, 'a.txt'), 'two\n');
  git(dir, 'add', '-A');
  commit(dir, 'feat: two', '2025-01-02T09:00:00+01:00');
  const blob = git(dir, 'rev-parse', 'HEAD~1:a.txt');
  const objectDir = join(dir, '.git', 'objects', blob.slice(0, 2));
  rmSync(join(objectDir, blob.slice(2)), { force: true });
  if (readdirSync(objectDir).length === 0) rmSync(objectDir, { recursive: true, force: true });
}
