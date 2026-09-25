// ESLint for @go-tebanare/engine. typescript-eslint supports TypeScript up
// to 6.0, so the workspace root installs typescript 6.0.3 for the linter,
// while `tsc` in this package runs TypeScript 7. The rules need no type
// information.
import js from '@eslint/js';
import { defineConfig } from 'eslint/config';
import tseslint from 'typescript-eslint';

export default defineConfig(
  { ignores: ['dist/', 'wasm/'] },
  js.configs.recommended,
  tseslint.configs.recommended,
  tseslint.configs.stylistic,
  {
    files: ['scripts/**/*.mjs'],
    languageOptions: {
      globals: { console: 'readonly', process: 'readonly', WebAssembly: 'readonly' },
    },
  },
);
