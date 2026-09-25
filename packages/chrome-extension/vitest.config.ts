import { fileURLToPath } from 'node:url';
import { defineConfig } from 'vitest/config';

export default defineConfig({
  resolve: {
    // Tests run against the engine sources, so they do not need a built
    // engine package. tsconfig.json maps the same import for tsc and esbuild.
    alias: {
      '@go-tebanare/engine': fileURLToPath(new URL('../engine/src/index.ts', import.meta.url)),
    },
  },
  test: {
    include: ['test/**/*.test.ts'],
    // Tests that need a DOM opt in with a "// @vitest-environment happy-dom" comment.
    environment: 'node',
  },
});
