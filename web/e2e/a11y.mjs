// Responsive, keyboard and contrast checks, evaluated inside the page.

/** Installed into the page as window.__audit. */
function pageHelpers() {
  window.__audit = {
    describe(el) {
      const classes =
        typeof el.className === 'string'
          ? el.className.trim().split(/\s+/).filter((c) => c && !c.startsWith('css-')).slice(0, 2).join('.')
          : '';
      const label = (el.getAttribute('aria-label') || el.textContent || '').trim().replace(/\s+/g, ' ').slice(0, 40);
      return `${el.tagName.toLowerCase()}${classes ? `.${classes}` : ''} "${label}"`;
    },
    visible(el) {
      if (el.closest('[hidden]')) return false;
      const style = getComputedStyle(el);
      if (style.visibility !== 'visible' || style.display === 'none') return false;
      const box = el.getBoundingClientRect();
      return box.width > 0 && box.height > 0;
    },
    overflow() {
      const width = document.documentElement.clientWidth;
      const scrollWidth = Math.max(document.documentElement.scrollWidth, document.body.scrollWidth);
      const clipped = (el) => {
        for (let parent = el.parentElement; parent && parent !== document.documentElement; parent = parent.parentElement) {
          if (['auto', 'scroll', 'hidden', 'clip'].includes(getComputedStyle(parent).overflowX)) return true;
        }
        return false;
      };
      const offenders = [];
      for (const el of document.body.querySelectorAll('*')) {
        const box = el.getBoundingClientRect();
        if (box.width === 0 || (box.right <= width + 1 && box.left >= -1) || clipped(el)) continue;
        offenders.push(`${this.visible(el) ? '' : 'hidden '}${this.describe(el)}`);
      }
      return { width, scrollWidth, offenders: offenders.slice(0, 10) };
    },
    contrast() {
      const parse = (color) => {
        const match = /rgba?\(([^)]+)\)/.exec(color || '');
        if (!match) return null;
        const parts = match[1].split(/[\s,/]+/).filter(Boolean).map(Number);
        return { r: parts[0], g: parts[1], b: parts[2], a: parts.length > 3 ? parts[3] : 1 };
      };
      const blend = (top, bottom) => ({
        r: top.r * top.a + bottom.r * (1 - top.a),
        g: top.g * top.a + bottom.g * (1 - top.a),
        b: top.b * top.a + bottom.b * (1 - top.a),
        a: 1,
      });
      const luminance = (c) => {
        const channel = (v) => {
          const s = v / 255;
          return s <= 0.03928 ? s / 12.92 : ((s + 0.055) / 1.055) ** 2.4;
        };
        return 0.2126 * channel(c.r) + 0.7152 * channel(c.g) + 0.0722 * channel(c.b);
      };
      const background = (el) => {
        const layers = [];
        for (let node = el; node; node = node.parentElement) {
          const color = parse(getComputedStyle(node).backgroundColor);
          if (color && color.a > 0) {
            layers.push(color);
            if (color.a >= 1) break;
          }
        }
        let result = { r: 255, g: 255, b: 255, a: 1 };
        for (let i = layers.length - 1; i >= 0; i--) result = blend(layers[i], result);
        return result;
      };
      const opacity = (el) => {
        let value = 1;
        for (let node = el; node; node = node.parentElement) value *= Number(getComputedStyle(node).opacity);
        return value;
      };
      const failures = new Set();
      let checked = 0;
      for (const el of document.body.querySelectorAll('*')) {
        if (![...el.childNodes].some((n) => n.nodeType === 3 && n.textContent.trim())) continue;
        if (!this.visible(el) || el.closest('[disabled], .Mui-disabled, [aria-disabled="true"], title, desc')) continue;
        const box = el.getBoundingClientRect();
        if (box.width <= 1 || box.height <= 1) continue;
        const style = getComputedStyle(el);
        const svg = el instanceof SVGElement;
        const foreground = parse(svg ? style.fill : style.color);
        if (!foreground) continue;
        const alpha = foreground.a * opacity(el);
        // Fully transparent text, such as MUI's outline notch, is not rendered.
        if (alpha < 0.05) continue;
        const back = background(svg ? el.closest('div, figure, section, main, body') : el);
        const text = blend({ ...foreground, a: alpha }, back);
        const ratio = (Math.max(luminance(text), luminance(back)) + 0.05) / (Math.min(luminance(text), luminance(back)) + 0.05);
        const size = parseFloat(style.fontSize);
        const required = size >= 24 || (size >= 18.66 && Number(style.fontWeight) >= 700) ? 3 : 4.5;
        checked++;
        if (ratio < required) failures.add(`${this.describe(el)} ${ratio.toFixed(2)}:1`);
      }
      return { checked, failures: [...failures].slice(0, 10) };
    },
    tabbables() {
      const list = [...document.querySelectorAll('a[href], button, input, select, textarea, summary, [tabindex]')].filter(
        (el) => !el.disabled && el.tabIndex >= 0 && this.visible(el) && !el.closest('[inert]'),
      );
      list.forEach((el, index) => {
        el.dataset.auditIndex = String(index);
      });
      return list.map((el) => this.describe(el));
    },
    focused() {
      const el = document.activeElement;
      if (!el || el === document.body || el.id === '__audit_start') return null;
      const style = getComputedStyle(el);
      const outline = style.outlineStyle !== 'none' && parseFloat(style.outlineWidth) > 0;
      const shadow = !!style.boxShadow && style.boxShadow !== 'none';
      return {
        index: el.dataset.auditIndex ?? null,
        description: this.describe(el),
        top: Math.round(el.getBoundingClientRect().top + window.scrollY),
        indicator: outline || shadow || !!el.closest('.Mui-focusVisible, .Mui-focused'),
      };
    },
  };
}

const sleep = (ms) => new Promise((resolve) => setTimeout(resolve, ms));

async function install(page) {
  await page.eval(`(${pageHelpers.toString()})()`);
}

async function tabWalk(page, scope, check) {
  const tabbables = await page.eval('__audit.tabbables()');
  await page.eval(`(() => {
    const start = document.createElement('div');
    start.id = '__audit_start';
    start.tabIndex = -1;
    document.body.prepend(start);
    start.focus();
  })()`);
  const seen = [];
  // Scrollable regions, such as a wide table, become tab stops of their own.
  for (let i = 0; i < tabbables.length * 2 + 20; i++) {
    await page.key('Tab', 9);
    await sleep(30);
    const focused = await page.eval('__audit.focused()');
    if (!focused) continue;
    if (seen.some((entry) => entry.index === focused.index && entry.description === focused.description)) break;
    seen.push(focused);
  }
  await page.eval(`document.getElementById('__audit_start')?.remove()`);
  const reached = new Set(seen.map((entry) => entry.index));
  const missing = tabbables.filter((_, index) => !reached.has(String(index)));
  check(missing.length === 0, `${scope}: every control is reachable with Tab${missing.length ? ` (missing ${missing.join('; ')})` : ''}`);
  const outOfOrder = seen.filter((entry, i) => i > 0 && entry.index !== null && seen[i - 1].index !== null && Number(entry.index) < Number(seen[i - 1].index));
  check(outOfOrder.length === 0, `${scope}: focus follows document order`);
  const noIndicator = seen.filter((entry) => !entry.indicator);
  check(noIndicator.length === 0, `${scope}: focus is visible on every stop${noIndicator.length ? ` (not on ${noIndicator.map((e) => e.description).join('; ')})` : ''}`);
}

/**
 * Checks a page for horizontal overflow at phone, tablet and desktop widths,
 * keyboard reachability at the narrowest and widest, and text contrast in
 * both themes when toggleTheme is given.
 */
export async function auditPage(page, scope, check, { keyboard = true, toggleTheme = null } = {}) {
  for (const width of [360, 768, 1440]) {
    await page.viewport(width, 900);
    await sleep(400);
    await install(page);
    const { width: viewport, scrollWidth, offenders } = await page.eval('__audit.overflow()');
    check(scrollWidth <= viewport && offenders.length === 0, `${scope} at ${width}px: no horizontal overflow${offenders.length ? ` (${offenders.join('; ')})` : ''}`);
    if (keyboard && width !== 768) await tabWalk(page, `${scope} at ${width}px`, check);
  }
  await page.viewport(1440, 900);
  await sleep(300);
  for (const theme of toggleTheme ? ['first theme', 'other theme'] : ['page theme']) {
    await install(page);
    const { checked, failures } = await page.eval('__audit.contrast()');
    check(failures.length === 0, `${scope}, ${theme}: ${checked} text elements meet WCAG AA contrast${failures.length ? ` (${failures.join('; ')})` : ''}`);
    if (toggleTheme && theme === 'first theme') {
      await page.eval(toggleTheme);
      await sleep(400);
    }
  }
  if (toggleTheme) {
    await page.eval(toggleTheme);
    await sleep(300);
  }
}
