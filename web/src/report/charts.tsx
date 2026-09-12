import { createContext, Fragment, useContext } from 'react';
import type { ReactElement, ReactNode } from 'react';

import { num } from './format';

/** The accessible name a figure gives the chart it wraps. */
export interface ChartLabel {
  titleId: string;
  descId: string;
  title: string;
  desc: string;
}

export const ChartLabelContext = createContext<ChartLabel | null>(null);

/**
 * The shared SVG root. Inside a figure it becomes an image named by its own
 * title and description, which the figure's data table repeats in full.
 */
function ChartSvg({
  width,
  height,
  className,
  children,
}: {
  width: number;
  height: number;
  className: string;
  children: ReactNode;
}): ReactElement {
  const label = useContext(ChartLabelContext);
  return (
    <svg
      viewBox={`0 0 ${width} ${height}`}
      className={`chart ${className}`}
      role={label ? 'img' : undefined}
      aria-labelledby={label ? `${label.titleId} ${label.descId}` : undefined}
    >
      {label ? <title id={label.titleId}>{label.title}</title> : null}
      {label ? <desc id={label.descId}>{label.desc}</desc> : null}
      {children}
    </svg>
  );
}

function scaleMax(values: number[]): number {
  const max = Math.max(0, ...values);
  return max === 0 ? 1 : max;
}

/** Area chart over an ordered series, used for the commit pulse. */
export function AreaChart({ labels, values }: { labels: string[]; values: number[] }): ReactElement {
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
  // Roughly a dozen x labels, whatever the series length.
  const step = Math.max(1, Math.ceil(labels.length / 12));

  return (
    <ChartSvg width={width} height={height} className="chart-area">
      <g transform={`translate(${pad.left},${pad.top})`}>
        {/* Horizontal guides, labelled, so a value can be read without a tooltip. */}
        {[0, 1, 2, 3, 4].map((i) => {
          const value = Math.round((max / 4) * i);
          const gy = y(value);
          return (
            <Fragment key={i}>
              <line className="grid" x1={0} x2={plotW} y1={gy} y2={gy} />
              <text className="axis-label" x={-8} y={gy + 4} textAnchor="end">
                {num(value)}
              </text>
            </Fragment>
          );
        })}
        <path className="area-fill" d={area} />
        <path className="area-line" d={line} />
        {labels.map((label, i) =>
          i % step !== 0 && i !== labels.length - 1 ? null : (
            <text key={i} className="axis-label" x={x(i)} y={plotH + 20} textAnchor="middle">
              {label}
            </text>
          ),
        )}
      </g>
    </ChartSvg>
  );
}

/** Horizontal bar chart, used wherever a ranking needs to stay readable. */
export function BarChart({ labels, values, unit }: { labels: string[]; values: number[]; unit: string }): ReactElement {
  const rowH = 26;
  const width = 960;
  const labelW = 220;
  const height = Math.max(rowH, labels.length * rowH) + 8;
  const barW = width - labelW - 90;
  const max = scaleMax(values);

  return (
    <ChartSvg width={width} height={height} className="chart-bar">
      {labels.map((label, i) => {
        const y = i * rowH + 4;
        const w = (values[i] / max) * barW;
        return (
          <Fragment key={i}>
            <text className="bar-label" x={labelW - 10} y={y + 14} textAnchor="end">
              {label}
            </text>
            <rect className="bar-track" x={labelW} y={y} width={barW} height={rowH - 8} rx={3} />
            <rect
              className="bar-value"
              x={labelW}
              y={y}
              width={Math.max(w, values[i] > 0 ? 2 : 0)}
              height={rowH - 8}
              rx={3}
            />
            {/* The number is printed beside every bar, so length is a convenience
                rather than the only way to read the chart. */}
            <text className="bar-number" x={labelW + barW + 8} y={y + 14}>
              {`${num(values[i])} ${unit}`}
            </text>
          </Fragment>
        );
      })}
    </ChartSvg>
  );
}

/** Radial 24-hour histogram: the chronotype ring. */
export function RadialHistogram({ values }: { values: number[] }): ReactElement {
  const size = 420;
  const cx = size / 2;
  const cy = size / 2;
  const inner = 62;
  const outer = 176;
  const max = scaleMax(values);

  const point = (angle: number, radius: number) =>
    `${(cx + Math.cos(angle) * radius).toFixed(2)},${(cy + Math.sin(angle) * radius).toFixed(2)}`;

  return (
    <ChartSvg width={size} height={size} className="chart-radial">
      <g>
        {values.map((value, hour) => {
          const a0 = (hour / 24) * Math.PI * 2 - Math.PI / 2;
          const a1 = ((hour + 1) / 24) * Math.PI * 2 - Math.PI / 2;
          const r = inner + (value / max) * (outer - inner);
          const d = [
            `M${point(a0, inner)}`,
            `L${point(a0, r)}`,
            `A${r},${r} 0 0 1 ${point(a1, r)}`,
            `L${point(a1, inner)}`,
            `A${inner},${inner} 0 0 0 ${point(a0, inner)}`,
            'Z',
          ].join(' ');
          const mid = (a0 + a1) / 2;
          return (
            <Fragment key={hour}>
              <path className={hour >= 22 || hour < 6 ? 'radial-wedge radial-night' : 'radial-wedge'} d={d} />
              {hour % 3 === 0 ? (
                <text
                  className="axis-label"
                  x={cx + Math.cos(mid) * (outer + 20)}
                  y={cy + Math.sin(mid) * (outer + 20) + 4}
                  textAnchor="middle"
                >
                  {String(hour).padStart(2, '0')}
                </text>
              ) : null}
            </Fragment>
          );
        })}
      </g>
    </ChartSvg>
  );
}

/** 7 x 24 heatmap of weekday against hour. */
export function Heatmap({ grid, weekdays }: { grid: number[][]; weekdays: string[] }): ReactElement {
  const cell = 34;
  const left = 46;
  const top = 22;
  const width = left + 24 * cell + 8;
  const height = top + 7 * cell + 8;
  const max = scaleMax(grid.flat());
  const hourLabels = Array.from({ length: 12 }, (_, i) => i * 2);

  return (
    <ChartSvg width={width} height={height} className="chart-heatmap">
      {hourLabels.map((hour) => (
        <text key={`hour-${hour}`} className="axis-label" x={left + hour * cell + cell / 2} y={14} textAnchor="middle">
          {String(hour).padStart(2, '0')}
        </text>
      ))}
      {grid.map((row, day) => (
        <Fragment key={day}>
          <text className="axis-label" x={left - 8} y={top + day * cell + cell / 2 + 4} textAnchor="end">
            {weekdays[day]}
          </text>
          {row.map((value, hour) => {
            // Five discrete steps rather than a continuous ramp: distinguishable
            // without fine colour discrimination, and each step is spelled out in
            // the accompanying table.
            const step = value === 0 ? 0 : Math.min(4, Math.ceil((value / max) * 4));
            return (
              <rect
                key={hour}
                className={`heat heat-${step}`}
                x={left + hour * cell}
                y={top + day * cell}
                width={cell - 3}
                height={cell - 3}
                rx={3}
              />
            );
          })}
        </Fragment>
      ))}
    </ChartSvg>
  );
}

/** Stacked horizontal bar showing what share of surviving lines each year holds. */
export function StrataChart({ years, lines }: { years: number[]; lines: number[] }): ReactElement {
  const width = 960;
  const height = 96;
  const total = lines.reduce((a, b) => a + b, 0) || 1;

  let offset = 0;
  const strata = years.map((year, i) => {
    const w = (lines[i] / total) * width;
    const x = offset;
    offset += w;
    return { year, i, w, x };
  });

  return (
    <ChartSvg width={width} height={height} className="chart-strata">
      {strata.map(({ year, i, w, x }) => (
        <Fragment key={i}>
          <rect className={`stratum stratum-${i % 6}`} x={x} y={0} width={Math.max(w, 1)} height={44} />
          {/* Labels go below the band so a narrow stratum is still identifiable. */}
          {w > 46 ? (
            <>
              <text className="strata-label" x={x + w / 2} y={62} textAnchor="middle">
                {String(year)}
              </text>
              <text className="strata-sub" x={x + w / 2} y={78} textAnchor="middle">
                {`${((lines[i] / total) * 100).toFixed(0)}%`}
              </text>
            </>
          ) : null}
        </Fragment>
      ))}
    </ChartSvg>
  );
}
