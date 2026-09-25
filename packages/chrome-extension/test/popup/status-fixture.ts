import type { PageState, PageStatus } from '../../src/shared/messages.js';

export const STATES: readonly PageState[] = ['inactive', 'excluded', 'loading', 'no-config', 'config-error', 'unsupported-ui', 'ready', 'error'];

/** pageStatus returns a ready status of octo/repo#7 with overrides applied. */
export function pageStatus(overrides: Partial<PageStatus> = {}): PageStatus {
  return {
    state: 'ready',
    repo: 'octo/repo',
    pr: 7,
    configPath: '.gotebanare.yml',
    configSource: 'base',
    rules: 3,
    files: 5,
    filesWithFolds: 2,
    linesHidden: 57,
    messages: [],
    enabled: true,
    headPreview: false,
    ...overrides,
  };
}
