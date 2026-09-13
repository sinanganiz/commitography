// A small Chrome DevTools Protocol client on Node's built-in WebSocket, so the
// browser audit needs no automation dependency.
import { execFileSync, spawn } from 'node:child_process';
import { existsSync, mkdtempSync, readFileSync, rmSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';

export const sleep = (ms) => new Promise((resolve) => setTimeout(resolve, ms));

/** Returns CHROME_PATH, or the first Chrome, Chromium or Edge installation found. */
export function findChrome() {
  if (process.env.CHROME_PATH) return process.env.CHROME_PATH;
  const programFiles = process.env.PROGRAMFILES ?? 'C:\\Program Files';
  const programFilesX86 = process.env['PROGRAMFILES(X86)'] ?? 'C:\\Program Files (x86)';
  const candidates = {
    win32: [
      join(programFiles, 'Google', 'Chrome', 'Application', 'chrome.exe'),
      join(programFilesX86, 'Google', 'Chrome', 'Application', 'chrome.exe'),
      join(process.env.LOCALAPPDATA ?? '', 'Google', 'Chrome', 'Application', 'chrome.exe'),
      join(programFilesX86, 'Microsoft', 'Edge', 'Application', 'msedge.exe'),
    ],
    darwin: [
      '/Applications/Google Chrome.app/Contents/MacOS/Google Chrome',
      '/Applications/Chromium.app/Contents/MacOS/Chromium',
    ],
    linux: ['/usr/bin/google-chrome', '/usr/bin/google-chrome-stable', '/usr/bin/chromium', '/usr/bin/chromium-browser'],
  }[process.platform] ?? [];
  const found = candidates.find((path) => path && existsSync(path));
  if (!found) throw new Error('no Chrome, Chromium or Edge installation was found; set CHROME_PATH');
  return found;
}

function killTree(child) {
  try {
    if (process.platform === 'win32') {
      execFileSync('taskkill', ['/PID', String(child.pid), '/T', '/F'], { stdio: 'ignore' });
    } else {
      process.kill(-child.pid, 'SIGKILL');
    }
  } catch {
    // Already gone.
  }
}

/** Starts headless Chrome and connects to its page. */
export async function launch({ width = 1440, height = 900 } = {}) {
  const profile = mkdtempSync(join(tmpdir(), 'commitography-chrome-'));
  const args = [
    '--headless=new',
    '--remote-debugging-port=0',
    `--user-data-dir=${profile}`,
    '--no-first-run',
    '--no-default-browser-check',
    `--window-size=${width},${height}`,
    'about:blank',
  ];
  if (process.platform === 'linux' && process.getuid?.() === 0) args.unshift('--no-sandbox');
  const chrome = spawn(findChrome(), args, { stdio: 'ignore', detached: process.platform !== 'win32' });

  // Chrome chooses a free port and writes it to its profile.
  let port = 0;
  for (let i = 0; i < 150 && !port; i++) {
    await sleep(100);
    try {
      port = Number(readFileSync(join(profile, 'DevToolsActivePort'), 'utf8').split('\n')[0]) || 0;
    } catch {
      // Not written yet.
    }
  }
  if (!port) {
    killTree(chrome);
    throw new Error('Chrome did not open a DevTools port');
  }
  let target;
  for (let i = 0; i < 50 && !target; i++) {
    try {
      const list = await (await fetch(`http://127.0.0.1:${port}/json/list`)).json();
      target = list.find((entry) => entry.type === 'page');
    } catch {
      // Not ready yet.
    }
    if (!target) await sleep(100);
  }
  if (!target) {
    killTree(chrome);
    throw new Error('Chrome has no page to control');
  }

  const socket = new WebSocket(target.webSocketDebuggerUrl);
  await new Promise((resolve, reject) => {
    socket.onopen = resolve;
    socket.onerror = reject;
  });

  let sequence = 0;
  const pending = new Map();
  const page = {
    consoleErrors: [],
    exceptions: [],
    /** Every request the page made, as { method, url }. */
    requests: [],
    send(method, params = {}) {
      const id = ++sequence;
      socket.send(JSON.stringify({ id, method, params }));
      return new Promise((resolve, reject) => pending.set(id, { resolve, reject }));
    },
    async eval(expression) {
      const response = await page.send('Runtime.evaluate', { expression, awaitPromise: true, returnByValue: true });
      if (response.exceptionDetails) {
        throw new Error(response.exceptionDetails.exception?.description ?? response.exceptionDetails.text);
      }
      return response.result.value;
    },
    async goto(url) {
      await page.send('Page.navigate', { url });
      await sleep(500);
    },
    async waitFor(expression, timeout = 20000, label = expression) {
      const started = Date.now();
      while (Date.now() - started < timeout) {
        try {
          const value = await page.eval(expression);
          if (value) return value;
        } catch {
          // The page may be navigating.
        }
        await sleep(100);
      }
      throw new Error(`timed out waiting for ${label}`);
    },
    async screenshot(path) {
      const { data } = await page.send('Page.captureScreenshot', { format: 'png', captureBeyondViewport: true });
      writeFileSync(path, Buffer.from(data, 'base64'));
    },
    async viewport(width, height) {
      await page.send('Emulation.setDeviceMetricsOverride', { width, height, deviceScaleFactor: 1, mobile: width < 600 });
    },
    async offline(enabled) {
      await page.send('Network.emulateNetworkConditions', {
        offline: enabled,
        latency: 0,
        downloadThroughput: -1,
        uploadThroughput: -1,
      });
    },
    /** Presses a key. Keys that activate controls need text, such as '\r' for Enter. */
    async key(key, keyCode, text) {
      const event = { key, code: key, windowsVirtualKeyCode: keyCode, text, unmodifiedText: text };
      await page.send('Input.dispatchKeyEvent', { type: 'keyDown', ...event });
      await page.send('Input.dispatchKeyEvent', { type: 'keyUp', key, code: key, windowsVirtualKeyCode: keyCode });
    },
    async close() {
      try {
        socket.close();
      } catch {
        // Already closed.
      }
      killTree(chrome);
      await sleep(300);
      rmSync(profile, { recursive: true, force: true });
    },
  };

  socket.onmessage = (event) => {
    const message = JSON.parse(event.data);
    if (message.id && pending.has(message.id)) {
      const { resolve, reject } = pending.get(message.id);
      pending.delete(message.id);
      if (message.error) reject(new Error(message.error.message));
      else resolve(message.result);
      return;
    }
    if (message.method === 'Runtime.consoleAPICalled' && message.params.type === 'error') {
      page.consoleErrors.push(message.params.args.map((arg) => arg.value ?? arg.description).join(' '));
    } else if (message.method === 'Runtime.exceptionThrown') {
      const details = message.params.exceptionDetails;
      page.exceptions.push(details.exception?.description ?? details.text);
    } else if (message.method === 'Network.requestWillBeSent') {
      page.requests.push({ method: message.params.request.method, url: message.params.request.url });
    }
  };

  await page.send('Page.enable');
  await page.send('Runtime.enable');
  await page.send('Network.enable');
  return page;
}
