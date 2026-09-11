import { defineConfig } from 'vite';

// The bundle is embedded into the Go binary and then inlined into a single
// HTML file, so the build must produce exactly one JS file and one CSS file
// with stable names. Hashed filenames and code splitting are both disabled for
// that reason.
export default defineConfig({
  build: {
    outDir: '../internal/render/assets',
    emptyOutDir: false,
    cssCodeSplit: false,
    // Targeting a recent baseline keeps the bundle small; the output is opened
    // in the developer's own browser, not shipped to the public web.
    target: 'es2020',
    lib: {
    entry: 'src/main.tsx',
      name: 'Commitography',
      formats: ['iife'],
      fileName: () => 'app.js',
    },
    rollupOptions: {
      output: {
        assetFileNames: 'app.[ext]',
      },
    },
  },
});
