// Browser audit of the local web dashboard: builds the binary and its own
// repositories, runs the server, and drives headless Chrome through every job
// flow, the recent jobs list, keyboard and responsive checks, and the legacy
// HTML pages, which ADR-0034 removes, opened offline.
//
//   npm run e2e
//
// Needs Node.js 22 or newer, Go, Git and Chrome (or CHROME_PATH). Screenshots
// go to E2E_OUT, or to a temporary folder that is printed at the end.
import { execFileSync, spawn } from 'node:child_process';
import { mkdirSync, mkdtempSync, rmSync } from 'node:fs';
import { createServer } from 'node:net';
import { tmpdir } from 'node:os';
import { dirname, join, resolve } from 'node:path';
import { fileURLToPath, pathToFileURL } from 'node:url';

import { auditPage } from './a11y.mjs';
import { launch, sleep } from './cdp.mjs';
import { badConfigRepository, brokenRepository, git, largeRepository, smallRepository } from './repositories.mjs';

if (typeof WebSocket === 'undefined') {
  console.error('The browser audit needs Node.js 22 or newer.');
  process.exit(2);
}

const checkout = resolve(dirname(fileURLToPath(import.meta.url)), '..', '..');
const out = process.env.E2E_OUT ?? mkdtempSync(join(tmpdir(), 'commitography-e2e-out-'));
mkdirSync(out, { recursive: true });
const work = mkdtempSync(join(tmpdir(), 'commitography-e2e-'));

let failures = 0;
function check(condition, message) {
  if (condition) {
    console.log(`ok - ${message}`);
  } else {
    failures++;
    console.log(`not ok - ${message}`);
  }
}

async function step(name, body) {
  console.log(`# ${name}`);
  try {
    await body();
  } catch (error) {
    failures++;
    console.log(`not ok - ${name}: ${error.message}`);
  }
}

function freePort() {
  return new Promise((resolvePort, reject) => {
    const probe = createServer();
    probe.once('error', reject);
    probe.listen(0, '127.0.0.1', () => {
      const { port } = probe.address();
      probe.close(() => resolvePort(port));
    });
  });
}

async function startServer(binary, port, allowedRoot) {
  const child = spawn(binary, ['serve', '--listen', `127.0.0.1:${port}`, '--allowed-root', allowedRoot], {
    stdio: ['ignore', 'pipe', 'pipe'],
  });
  let log = '';
  child.stdout.on('data', (chunk) => (log += chunk));
  child.stderr.on('data', (chunk) => (log += chunk));
  const base = `http://127.0.0.1:${port}/`;
  for (let i = 0; i < 100; i++) {
    try {
      if ((await fetch(base)).ok) return { child, log: () => log };
    } catch {
      // Not listening yet.
    }
    await sleep(100);
  }
  child.kill();
  throw new Error(`the server did not start: ${log}`);
}

async function stopServer(server) {
  if (server.child.exitCode !== null) return;
  const exited = new Promise((resolveExit) => server.child.once('exit', resolveExit));
  server.child.kill();
  await exited;
}

// --- setup ----------------------------------------------------------------------

const repos = join(work, 'repos');
const paths = {
  small: join(repos, 'small'),
  large: join(repos, 'large'),
  stale: join(repos, 'stale'),
  badConfig: join(repos, 'bad-config'),
  broken: join(repos, 'broken'),
};
console.log(`# building repositories in ${repos}`);
smallRepository(paths.small);
largeRepository(paths.large);
git(repos, 'clone', '-q', paths.large, paths.stale);
badConfigRepository(paths.badConfig);
brokenRepository(paths.broken);

console.log('# building the binary');
const binary = join(work, process.platform === 'win32' ? 'commitography.exe' : 'commitography');
execFileSync('go', ['build', '-o', binary, './cmd/commitography'], { cwd: checkout, stdio: 'inherit' });
const staticDir = join(work, 'static');
execFileSync(binary, [paths.small, '-o', staticDir, '--per-author', '-q'], { stdio: 'inherit' });
execFileSync(binary, [paths.small, '-o', staticDir, '--wrapped', '2025', '-q'], { stdio: 'inherit' });

const port = await freePort();
const base = `http://127.0.0.1:${port}/`;
let server = await startServer(binary, port, repos);
const page = await launch();

// --- page helpers ---------------------------------------------------------------

const alertTitle = (title) =>
  `[...document.querySelectorAll('.MuiAlertTitle-root')].some((n) => n.textContent === ${JSON.stringify(title)})`;
const jobRoute = `/^#\\/jobs\\/[0-9a-f]+$/.test(location.hash) && location.hash.slice(7)`;
const statusPolls = (id) => page.requests.filter((r) => r.url === `${base}api/v1/jobs/${id}`).length;

async function startFromForm(path) {
  await page.goto(`${base}#/`);
  await page.waitFor(`!document.querySelector('button[type=submit]')?.disabled`, 20000, 'the form to be ready');
  await page.eval(`(() => {
    const input = document.querySelector('form input:not([type=checkbox])');
    const setValue = Object.getOwnPropertyDescriptor(HTMLInputElement.prototype, 'value').set;
    setValue.call(input, ${JSON.stringify(path)});
    input.dispatchEvent(new Event('input', { bubbles: true }));
    document.querySelector('button[type=submit]').click();
  })()`);
  return page.waitFor(jobRoute, 20000, 'the job route');
}

async function startFromApi(path, options = {}) {
  const body = JSON.stringify({
    repoPath: path,
    options: { noBlame: false, perAuthor: false, anonymize: false, allowShallow: false, countMerges: false, since: '', until: '', ...options },
  });
  const created = await page.eval(`fetch('/api/v1/jobs', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: ${JSON.stringify(body)} })
    .then(async (response) => ({ status: response.status, body: await response.json() }))`);
  if (created.status !== 202) throw new Error(`creating a job for ${path} answered ${JSON.stringify(created)}`);
  return created.body.id;
}

const jobStatus = (id) => page.eval(`fetch('/api/v1/jobs/${id}').then((response) => response.json())`);

async function cancelFromApi(id) {
  await page.eval(`fetch('/api/v1/jobs/${id}/cancel', { method: 'POST' }).then((response) => response.status)`);
}

const APP_THEME = `document.querySelector('button[aria-label^="Switch to"]').click()`;
const STATIC_THEME = `document.querySelector('.theme-toggle').click()`;

let succeeded;
try {
  await step('empty history', async () => {
    await page.goto(`${base}#/recent`);
    await page.waitFor(`document.body.innerText.includes('No analyses yet')`, 20000, 'the empty history');
    check(true, 'a fresh server shows an intentional empty history');
  });

  await step('start, progress and success', async () => {
    succeeded = await startFromForm(paths.small);
    await page.eval('window.__noReload = true');
    await page.waitFor(alertTitle('Analysis complete'), 60000, 'the success outcome');
    await page.waitFor(`!!document.querySelector('.cg-report section#pulse')`, 20000, 'the report');
    check(await page.eval('window.__noReload === true'), 'the report appears after completion without a page reload');
    const announced = await page.eval(`document.querySelector('[role=status][aria-live=polite]')?.textContent`);
    check(announced === 'Analysis complete', `the outcome is announced (${announced})`);
    const polls = statusPolls(succeeded);
    await sleep(2500);
    check(statusPolls(succeeded) === polls, 'polling stops once the job has succeeded');
    await page.screenshot(join(out, 'success.png'));
  });

  await step('progress and cancellation', async () => {
    const id = await startFromForm(paths.large);
    await page.waitFor(`!!document.querySelector('[role=progressbar][aria-label="Analysis progress"]')`, 20000, 'the progress bar');
    await page.eval('window.__noReload = true');
    let estimated = false;
    let active = false;
    for (let i = 0; i < 60 && !estimated; i++) {
      const state = await page.eval(`(() => {
        const bar = document.querySelector('[role=progressbar][aria-label="Analysis progress"]');
        return { text: bar?.getAttribute('aria-valuetext') ?? '', spinner: !!document.querySelector('main .MuiCircularProgress-root') };
      })()`);
      active ||= state.spinner;
      estimated = /estimated/.test(state.text);
      await sleep(100);
    }
    check(active, 'a running stage shows activity');
    check(estimated, 'the percentage is labelled as an estimate');
    const stage = await page.eval(`document.querySelector('[role=status][aria-live=polite]')?.textContent`);
    check(!!stage, `the active stage is announced (${stage})`);
    await page.screenshot(join(out, 'running.png'));
    await page.eval(`[...document.querySelectorAll('button')].find((b) => b.textContent === 'Cancel analysis').click()`);
    await page.waitFor(alertTitle('Analysis cancelled'), 30000, 'the cancelled outcome');
    check(await page.eval('window.__noReload === true'), 'cancellation completes without a page reload');
    const polls = statusPolls(id);
    await sleep(2500);
    check(statusPolls(id) === polls, 'polling stops once the job is cancelled');
  });

  await step('rejected and failed analyses', async () => {
    await startFromForm(paths.badConfig);
    await page.waitFor(alertTitle('The analysis request was rejected'), 30000, 'the rejected outcome');
    check(true, 'an invalid repository configuration is reported as a rejected request');
    await startFromForm(paths.broken);
    await page.waitFor(alertTitle('The analysis failed'), 30000, 'the failed outcome');
    check(true, 'a Git failure is reported as a failed analysis');
    await page.screenshot(join(out, 'failed.png'));
  });

  await step('stale repository', async () => {
    const id = await startFromApi(paths.stale);
    await page.goto(`${base}#/jobs/${id}`);
    // The change has to land after the analysis read HEAD and before its
    // closing consistency check, so it is made during a stage that follows
    // history collection. Blame stays on to keep those stages long enough.
    const laterStages = ['identity', 'filtering', 'temporal', 'code', 'messages', 'social', 'notables'];
    let moved = false;
    for (let i = 0; i < 2400 && !moved; i++) {
      const status = await jobStatus(id);
      if (status.status !== 'queued' && status.status !== 'running') break;
      if (laterStages.includes(status.progress?.stage)) {
        git(paths.stale, 'commit', '--allow-empty', '-q', '-m', 'chore: moved on during the analysis');
        moved = true;
      }
      await sleep(25);
    }
    check(moved, 'the repository changed while the analysis was running');
    await page.waitFor(alertTitle('The repository changed during the analysis'), 120000, 'the stale outcome');
    await sleep(800);
    check(!(await page.eval(`!!document.querySelector('.cg-report')`)), 'a stale job never shows a report');
  });

  await step('recent jobs', async () => {
    await page.goto(`${base}#/`);
    await page.goto(`${base}#/recent`);
    await page.waitFor(`document.querySelectorAll('main li[aria-labelledby^="job-"]').length === 5`, 20000, 'five jobs');
    await page.waitFor(`document.body.innerText.includes('Rejected:')`, 10000, 'the failure details');
    const text = await page.eval('document.querySelector("main").innerText');
    for (const summary of [
      'Report ready.',
      'Cancelled before a report was produced.',
      'Rejected: the repository or its settings did not pass validation.',
      'Failed: Git or the analysis stopped before a report was produced.',
      'No report: the repository changed during the analysis.',
    ]) {
      check(text.includes(summary), `the list describes a job as "${summary}"`);
    }
    await page.screenshot(join(out, 'recent.png'));
  });

  await step('keyboard and responsive layout', async () => {
    await page.goto(`${base}#/`);
    await page.waitFor(`!document.querySelector('button[type=submit]')?.disabled`, 20000, 'the form');
    await auditPage(page, 'start view', check, { toggleTheme: APP_THEME });

    await page.goto(`${base}#/jobs/${succeeded}`);
    await page.waitFor(`!!document.querySelector('.cg-report section#pulse')`, 20000, 'the report');
    await auditPage(page, 'job report', check, { toggleTheme: APP_THEME });
    const charts = await page.eval(`[...document.querySelectorAll('.cg-report figure')].every((f) => f.querySelector('svg[role=img]') && f.querySelector('table caption'))`);
    check(charts, 'every chart keeps an accessible name and a data table');
    await page.eval(`[...document.querySelectorAll('nav a')].find((a) => a.textContent === 'Recent jobs').click()`);
    await sleep(600);
    check((await page.eval('document.activeElement.tagName')) === 'H2', 'moving to another view puts focus on its heading');

    await page.goto(`${base}#/recent`);
    await page.waitFor(`document.querySelectorAll('main li[aria-labelledby^="job-"]').length === 5`, 20000, 'the list');
    await auditPage(page, 'recent jobs', check, { toggleTheme: APP_THEME });
  });

  await step('static pages offline', async () => {
    await page.offline(true);
    const before = page.requests.length;
    await page.goto(pathToFileURL(join(staticDir, 'index.html')).href);
    await page.waitFor(`!!document.querySelector('section#pulse')`, 10000, 'the static dashboard');
    await auditPage(page, 'static dashboard', check, { toggleTheme: STATIC_THEME });
    await page.goto(pathToFileURL(join(staticDir, 'wrapped-2025.html')).href);
    await page.waitFor(`document.querySelectorAll('.wrapped-card').length > 0`, 10000, 'the Wrapped cards');
    await auditPage(page, 'static Wrapped', check);
    const external = page.requests.slice(before).filter((r) => !/^(file|data|blob|about):/.test(r.url));
    check(external.length === 0, `the static pages make no network request${external.length ? ` (${external.map((r) => r.url).join(', ')})` : ''}`);
    await page.offline(false);
  });

  await step('a lost server', async () => {
    await page.viewport(1440, 900);
    const id = await startFromForm(paths.large);
    await page.waitFor(`!!document.querySelector('[role=progressbar][aria-label="Analysis progress"]')`, 20000, 'the progress bar');
    const before = page.requests.length;
    await stopServer(server);
    await page.waitFor(`document.body.innerText.includes('Lost contact with the local server')`, 30000, 'the lost connection');
    await page.eval(`[...document.querySelectorAll('button')].find((b) => b.textContent === 'Retry').click()`);
    await sleep(500);
    check(!page.requests.slice(before).some((r) => r.method === 'POST'), 'Retry only reads status and cannot start a second job');
    server = await startServer(binary, port, repos);
    // The restarted server has a new session secret, so the next status read
    // is refused and the page asks for a reload. If both automatic attempts
    // failed before the restart, Retry is offered again and pressed.
    await page.waitFor(
      `(() => {
        if (document.body.innerText.includes('The local session has expired')) return true;
        [...document.querySelectorAll('button')].find((b) => b.textContent === 'Retry')?.click();
        return false;
      })()`,
      30000,
      'the expired session',
    );
    check(true, `after a restart, job ${id} reports an expired session instead of a blank page`);
  });

  check(page.exceptions.length === 0, `no uncaught exception${page.exceptions.length ? ` (${page.exceptions.join(' | ')})` : ''}`);
  check(page.consoleErrors.length === 0, `no console error${page.consoleErrors.length ? ` (${page.consoleErrors.join(' | ')})` : ''}`);
  const foreign = page.requests.filter((r) => !r.url.startsWith(base) && !/^(file|data|blob|about):/.test(r.url));
  check(foreign.length === 0, `no request leaves the local server's origin${foreign.length ? ` (${foreign.map((r) => r.url).join(', ')})` : ''}`);
} finally {
  await page.close();
  await stopServer(server);
  rmSync(work, { recursive: true, force: true, maxRetries: 5, retryDelay: 200 });
}

console.log(`# screenshots in ${out}`);
console.log(failures === 0 ? '# all checks passed' : `# ${failures} checks failed`);
process.exitCode = failures === 0 ? 0 : 1;
