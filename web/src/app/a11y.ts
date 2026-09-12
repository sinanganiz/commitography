/**
 * Hides content visually while keeping it available to assistive technology.
 * Pixel strings, not numbers: MUI's sx reads 1 as 100% and -1 as a spacing unit.
 */
export const visuallyHidden = {
  position: 'absolute',
  width: '1px',
  height: '1px',
  margin: '-1px',
  padding: 0,
  overflow: 'hidden',
  clip: 'rect(0 0 0 0)',
  whiteSpace: 'nowrap',
  border: 0,
} as const;
