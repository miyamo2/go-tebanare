// ESLint for @go-tebanare/chrome-extension. typescript-eslint supports
// TypeScript up to 6.0, so the workspace root installs typescript 6.0.3 for
// the linter, while `tsc` in this package runs TypeScript 7. The rules need
// no type information.
import js from '@eslint/js';
import { defineConfig } from 'eslint/config';
import tseslint from 'typescript-eslint';

export default defineConfig(
  { ignores: ['dist/', 'dist.zip', 'test-results/', 'playwright-report/'] },
  js.configs.recommended,
  tseslint.configs.recommended,
  tseslint.configs.stylistic,
  {
    rules: {
      '@typescript-eslint/no-unused-vars': [
        'error',
        { argsIgnorePattern: '^_', varsIgnorePattern: '^_', destructuredArrayIgnorePattern: '^_' },
      ],
    },
  },
  {
    // Test fakes stand in for Chrome APIs with empty functions on purpose.
    files: ['test/**/*.ts', 'e2e/**/*.ts', 'e2e-live/**/*.ts'],
    rules: { '@typescript-eslint/no-empty-function': 'off' },
  },
  {
    files: ['scripts/**/*.mjs'],
    languageOptions: {
      globals: { Buffer: 'readonly', console: 'readonly', process: 'readonly', URL: 'readonly' },
    },
  },
);
