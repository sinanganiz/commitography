import { useId, useState } from 'react';
import type { ReactElement, ReactNode } from 'react';

import { ChartLabelContext } from './charts';
import { num } from './format';

/** One row of the accessible table that accompanies every visualization. */
export interface DataRow {
  label: string;
  values: (string | number)[];
}

interface FigureProps {
  title: string;
  desc: string;
  columns: string[];
  rows: DataRow[];
  /** The chart. It takes its accessible name from this figure. */
  children: ReactNode;
}

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
export function Figure({ title, desc, columns, rows, children }: FigureProps): ReactElement {
  const id = useId();
  const tableId = `${id}-table`;
  const [open, setOpen] = useState(false);

  return (
    <figure className="figure">
      <div className="figure-graphic">
        <ChartLabelContext.Provider value={{ titleId: `${id}-title`, descId: `${id}-desc`, title, desc }}>
          {children}
        </ChartLabelContext.Provider>
      </div>
      <button
        type="button"
        className="data-toggle"
        aria-expanded={open}
        aria-controls={tableId}
        onClick={() => setOpen(!open)}
      >
        {open ? 'Hide data' : 'Show data'}
      </button>
      <div className="data-table-wrap" hidden={!open}>
        <table className="data-table" id={tableId}>
          <caption>{title}</caption>
          <thead>
            <tr>
              {columns.map((column, index) => (
                <th key={index} scope="col">
                  {column}
                </th>
              ))}
            </tr>
          </thead>
          <tbody>
            {rows.map((row, index) => (
              <tr key={index}>
                <th scope="row">{row.label}</th>
                {row.values.map((value, column) => (
                  <td key={column}>{typeof value === 'number' ? num(value) : value}</td>
                ))}
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </figure>
  );
}
