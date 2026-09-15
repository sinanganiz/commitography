import { createContext, useContext } from 'react';
import type { HTMLAttributes, ReactElement } from 'react';

/**
 * The heading level of the report's top title. The static page starts at h1;
 * inside the local application the job title already owns h2, so the report
 * starts one level lower and the outline stays unbroken either way.
 */
export const HeadingBase = createContext(1);

type HeadingTag = 'h1' | 'h2' | 'h3' | 'h4' | 'h5' | 'h6';

/** A heading `depth` levels below the report's base level. */
export function H({ depth, ...props }: HTMLAttributes<HTMLHeadingElement> & { depth: number }): ReactElement {
  const base = useContext(HeadingBase);
  const Tag = `h${Math.min(6, Math.max(1, base + depth))}` as HeadingTag;
  return <Tag {...props} />;
}
