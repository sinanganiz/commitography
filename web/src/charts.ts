import { el, svgEl, num } from './dom';

/** One row of the accessible table that accompanies every visualization. */
export interface DataRow {
  label: string;
  values: (string | number)[];
}

export interface FigureOptions {
  title: string;
  desc: string;
  columns: string[];
  rows: DataRow[];
}

let figureSeq = 0;

/**
 * Wraps a visualization together with the same numbers in a table.
 *
 * Colour never carries meaning on its own here: the table is the authoritative
 * reading of every chart, and a per-figure toggle reveals it. The table uses the
 * `hidden` attribute, so it leaves the accessibility tree along with the layout
 * while collapsed; the toggle carries `aria-expanded` and `aria-controls` so it
 * announces as the disclosure it is. Printing overrides `hidden` in CSS, so a
 * printed page still carries every number.
 */
export function figure(graphic: SVGElement, options: FigureOptions): HTMLElement {
  const id = `fig-${++figureSeq}`;
  const tableId = `${id}-table`;

  graphic.setAttribute('role', 'img');
  graphic.setAttribute('aria-labelledby', `${id}-title ${id}-desc`);
  graphic.insertBefore(svgEl('desc', { id: `${id}-desc` }, options.desc), graphic.firstChild);
  graphic.insertBefore(svgEl('title', { id: `${id}-title` }, options.title), graphic.firstChild);

  const table = el(
    'table',
    { class: 'data-table', id: tableId },
    el('caption', {}, options.title),
    el('thead', {}, el('tr', {}, ...options.columns.map((c) => el('th', { scope: 'col' }, c)))),
    el(
      'tbody',
      {},
      ...options.rows.map((row) =>
        el(
          'tr',
          {},
          el('th', { scope: 'row' }, row.label),
          ...row.values.map((v) => el('td', {}, typeof v === 'number' ? num(v) : v)),
        ),
      ),
    ),
  );

  const wrapper = el('div', { class: 'data-table-wrap', hidden: true }, table);

  const toggle = el(
    'button',
    { type: 'button', class: 'data-toggle', 'aria-expanded': 'false', 'aria-controls': tableId },
    'Show data',
  );
  toggle.addEventListener('click', () => {
    const open = wrapper.hasAttribute('hidden');
    if (open) {
      wrapper.removeAttribute('hidden');
    } else {
      wrapper.setAttribute('hidden', '');
    }
    toggle.setAttribute('aria-expanded', String(open));
    toggle.textContent = open ? 'Hide data' : 'Show data';
  });

  return el('figure', { class: 'figure' }, el('div', { class: 'figure-graphic' }, graphic), toggle, wrapper);
}

function scaleMax(values: number[]): number {
  const max = Math.max(0, ...values);
  return max === 0 ? 1 : max;
}

/** Area chart over an ordered series, used for the commit pulse. */
export function areaChart(labels: string[], values: number[]): SVGElement {
  const width = 960;
  const height = 260;
  const pad = { top: 16, right: 16, bottom: 34, left: 52 };
  const plotW = width - pad.left - pad.right;
  const plotH = height - pad.top - pad.bottom;
  const max = scaleMax(values);

  const x = (i: number) => (values.length <= 1 ? plotW / 2 : (i / (values.length - 1)) * plotW);
  const y = (v: number) => plotH - (v / max) * plotH;

  const line = values.map((v, i) => `${i === 0 ? 'M' : 'L'}${x(i).toFixed(2)},${y(v).toFixed(2)}`).join(' ');
  const area = `${line} L${x(values.length - 1).toFixed(2)},${plotH} L${x(0).toFixed(2)},${plotH} Z`;

  const plot = svgEl('g', { transform: `translate(${pad.left},${pad.top})` });

  // Horizontal guides, labelled, so a value can be read without a tooltip.
  for (let i = 0; i <= 4; i++) {
    const value = Math.round((max / 4) * i);
    const gy = y(value);
    plot.appendChild(svgEl('line', { class: 'grid', x1: 0, x2: plotW, y1: gy, y2: gy }));
    plot.appendChild(
      svgEl('text', { class: 'axis-label', x: -8, y: gy + 4, 'text-anchor': 'end' }, num(value)),
    );
  }

  plot.appendChild(svgEl('path', { class: 'area-fill', d: area }));
  plot.appendChild(svgEl('path', { class: 'area-line', d: line }));

  // Roughly a dozen x labels, whatever the series length.
  const step = Math.max(1, Math.ceil(labels.length / 12));
  labels.forEach((label, i) => {
    if (i % step !== 0 && i !== labels.length - 1) return;
    plot.appendChild(
      svgEl(
        'text',
        { class: 'axis-label', x: x(i), y: plotH + 20, 'text-anchor': 'middle' },
        label,
      ),
    );
  });

  return svgEl('svg', { viewBox: `0 0 ${width} ${height}`, class: 'chart chart-area' }, plot);
}

/** Horizontal bar chart, used wherever a ranking needs to stay readable. */
export function barChart(labels: string[], values: number[], unit: string): SVGElement {
  const rowH = 26;
  const width = 960;
  const labelW = 220;
  const height = Math.max(rowH, labels.length * rowH) + 8;
  const barW = width - labelW - 90;
  const max = scaleMax(values);

  const root = svgEl('svg', { viewBox: `0 0 ${width} ${height}`, class: 'chart chart-bar' });

  labels.forEach((label, i) => {
    const y = i * rowH + 4;
    const w = (values[i] / max) * barW;
    root.appendChild(
      svgEl('text', { class: 'bar-label', x: labelW - 10, y: y + 14, 'text-anchor': 'end' }, label),
    );
    root.appendChild(
      svgEl('rect', { class: 'bar-track', x: labelW, y, width: barW, height: rowH - 8, rx: 3 }),
    );
    root.appendChild(
      svgEl('rect', {
        class: 'bar-value',
        x: labelW,
        y,
        width: Math.max(w, values[i] > 0 ? 2 : 0),
        height: rowH - 8,
        rx: 3,
      }),
    );
    // The number is printed beside every bar, so length is a convenience
    // rather than the only way to read the chart.
    root.appendChild(
      svgEl(
        'text',
        { class: 'bar-number', x: labelW + barW + 8, y: y + 14 },
        `${num(values[i])} ${unit}`,
      ),
    );
  });

  return root;
}

/** Radial 24-hour histogram: the chronotype ring. */
export function radialHistogram(values: number[]): SVGElement {
  const size = 420;
  const cx = size / 2;
  const cy = size / 2;
  const inner = 62;
  const outer = 176;
  const max = scaleMax(values);

  const root = svgEl('svg', { viewBox: `0 0 ${size} ${size}`, class: 'chart chart-radial' });
  const g = svgEl('g');

  values.forEach((value, hour) => {
    const a0 = (hour / 24) * Math.PI * 2 - Math.PI / 2;
    const a1 = ((hour + 1) / 24) * Math.PI * 2 - Math.PI / 2;
    const r = inner + (value / max) * (outer - inner);

    const p = (angle: number, radius: number) =>
      `${(cx + Math.cos(angle) * radius).toFixed(2)},${(cy + Math.sin(angle) * radius).toFixed(2)}`;

    const d = [
      `M${p(a0, inner)}`,
      `L${p(a0, r)}`,
      `A${r},${r} 0 0 1 ${p(a1, r)}`,
      `L${p(a1, inner)}`,
      `A${inner},${inner} 0 0 0 ${p(a0, inner)}`,
      'Z',
    ].join(' ');

    g.appendChild(
      svgEl('path', {
        class: hour >= 22 || hour < 6 ? 'radial-wedge radial-night' : 'radial-wedge',
        d,
      }),
    );

    if (hour % 3 === 0) {
      const mid = (a0 + a1) / 2;
      g.appendChild(
        svgEl(
          'text',
          {
            class: 'axis-label',
            x: cx + Math.cos(mid) * (outer + 20),
            y: cy + Math.sin(mid) * (outer + 20) + 4,
            'text-anchor': 'middle',
          },
          `${String(hour).padStart(2, '0')}`,
        ),
      );
    }
  });

  root.appendChild(g);
  return root;
}

/** 7 x 24 heatmap of weekday against hour. */
export function heatmap(grid: number[][], weekdays: string[]): SVGElement {
  const cell = 34;
  const left = 46;
  const top = 22;
  const width = left + 24 * cell + 8;
  const height = top + 7 * cell + 8;
  const max = scaleMax(grid.flat());

  const root = svgEl('svg', { viewBox: `0 0 ${width} ${height}`, class: 'chart chart-heatmap' });

  for (let hour = 0; hour < 24; hour += 2) {
    root.appendChild(
      svgEl(
        'text',
        { class: 'axis-label', x: left + hour * cell + cell / 2, y: 14, 'text-anchor': 'middle' },
        String(hour).padStart(2, '0'),
      ),
    );
  }

  grid.forEach((row, day) => {
    root.appendChild(
      svgEl(
        'text',
        { class: 'axis-label', x: left - 8, y: top + day * cell + cell / 2 + 4, 'text-anchor': 'end' },
        weekdays[day],
      ),
    );
    row.forEach((value, hour) => {
      // Five discrete steps rather than a continuous ramp: distinguishable
      // without fine colour discrimination, and each step is spelled out in
      // the accompanying table.
      const step = value === 0 ? 0 : Math.min(4, Math.ceil((value / max) * 4));
      root.appendChild(
        svgEl('rect', {
          class: `heat heat-${step}`,
          x: left + hour * cell,
          y: top + day * cell,
          width: cell - 3,
          height: cell - 3,
          rx: 3,
        }),
      );
    });
  });

  return root;
}

/** Stacked horizontal bar showing what share of surviving lines each year holds. */
export function strataChart(years: number[], lines: number[]): SVGElement {
  const width = 960;
  const height = 96;
  const total = lines.reduce((a, b) => a + b, 0) || 1;

  const root = svgEl('svg', { viewBox: `0 0 ${width} ${height}`, class: 'chart chart-strata' });
  let x = 0;

  years.forEach((year, i) => {
    const w = (lines[i] / total) * width;
    root.appendChild(
      svgEl('rect', {
        class: `stratum stratum-${i % 6}`,
        x,
        y: 0,
        width: Math.max(w, 1),
        height: 44,
      }),
    );
    // Labels go below the band so a narrow stratum is still identifiable.
    if (w > 46) {
      root.appendChild(
        svgEl('text', { class: 'strata-label', x: x + w / 2, y: 62, 'text-anchor': 'middle' }, String(year)),
      );
      root.appendChild(
        svgEl(
          'text',
          { class: 'strata-sub', x: x + w / 2, y: 78, 'text-anchor': 'middle' },
          `${((lines[i] / total) * 100).toFixed(0)}%`,
        ),
      );
    }
    x += w;
  });

  return root;
}
