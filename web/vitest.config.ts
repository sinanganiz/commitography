import { defineConfig } from 'vitest/config';

// Component tests run in jsdom. They are kept out of vite.config.ts, whose
// library build produces the embedded bundle and must not change for tests.
export default defineConfig({
  esbuild: {
    jsx: 'automatic',
  },
  test: {
    environment: 'jsdom',
    include: ['src/**/*.test.{ts,tsx}'],
    restoreMocks: true,
    unstubGlobals: true,
  },
});
