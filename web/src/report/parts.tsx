import type { ReactElement, ReactNode } from 'react';

import { H } from './heading';

export function Section({ id, title, children }: { id: string; title: string; children: ReactNode }): ReactElement {
  return (
    <section className="section" id={id} aria-labelledby={`${id}-heading`}>
      <H depth={1} className="section-heading" id={`${id}-heading`}>
        {title}
      </H>
      {children}
    </section>
  );
}

export function Subhead({ children }: { children: ReactNode }): ReactElement {
  return <p className="section-sub">{children}</p>;
}

export function Subsection({ children }: { children: ReactNode }): ReactElement {
  return (
    <H depth={2} className="subsection-heading">
      {children}
    </H>
  );
}

export function Stat({ value, label, note }: { value: string; label: string; note?: string }): ReactElement {
  return (
    <div className="stat">
      <div className="stat-value">{value}</div>
      <div className="stat-label">{label}</div>
      {note ? <div className="stat-note">{note}</div> : null}
    </div>
  );
}

export function StatRow({ children }: { children: ReactNode }): ReactElement {
  return <div className="stat-row">{children}</div>;
}

/** A plain listing table. Rows never reorder, so their index is a stable key. */
export function Listing({ columns, rows }: { columns: string[]; rows: ReactNode[][] }): ReactElement {
  return (
    <div className="table-wrap">
      <table className="listing">
        <thead>
          <tr>
            {columns.map((column) => (
              <th key={column} scope="col">
                {column}
              </th>
            ))}
          </tr>
        </thead>
        <tbody>
          {rows.map((cells, row) => (
            <tr key={row}>
              {cells.map((cell, column) => (
                <td key={column}>{cell}</td>
              ))}
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}
