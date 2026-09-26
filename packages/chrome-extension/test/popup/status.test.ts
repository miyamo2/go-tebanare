import { describe, expect, it } from 'vitest';
import { controlsFor, parsePageStatus, stateKey } from '../../src/popup/status.js';
import { localeMessages } from '../fakes/locale.js';
import { pageStatus, STATES } from './status-fixture.js';

describe('parsePageStatus', () => {
  it('accepts a full status and one without the optional fields', () => {
    const full = pageStatus({ messages: ['a', 'b'] });
    expect(parsePageStatus(full)).toEqual(full);
    const bare = { ...pageStatus({ state: 'inactive' }) } as Record<string, unknown>;
    delete bare['repo'];
    delete bare['pr'];
    delete bare['configPath'];
    expect(parsePageStatus(bare)).toEqual(bare);
  });

  it.each([
    ['no answer', undefined],
    ['a string', 'ready'],
    ['null', null],
    ['an unknown state', { ...pageStatus(), state: 'done' }],
    ['an inherited state name', { ...pageStatus(), state: 'toString' }],
    ['an unknown config source', { ...pageStatus(), configSource: 'fork' }],
    ['a numeric repo', { ...pageStatus(), repo: 5 }],
    ['pull request 0', { ...pageStatus(), pr: 0 }],
    ['a fractional pull request', { ...pageStatus(), pr: 1.5 }],
    ['a numeric config path', { ...pageStatus(), configPath: 1 }],
    ['a negative count', { ...pageStatus(), linesHidden: -1 }],
    ['a missing count', { ...pageStatus(), rules: undefined }],
    ['a count that is a string', { ...pageStatus(), files: '3' }],
    ['non-string messages', { ...pageStatus(), messages: ['a', 2] }],
    ['messages that are not an array', { ...pageStatus(), messages: 'a' }],
    ['enabled that is not a boolean', { ...pageStatus(), enabled: 'yes' }],
    ['a missing headPreview', { ...pageStatus(), headPreview: undefined }],
  ])('rejects %s', (_, v) => {
    expect(parsePageStatus(v)).toBeNull();
  });
});

describe('stateKey', () => {
  it.each(STATES)('names %s in every locale', (state) => {
    const key = stateKey({ state, configSource: 'base' });
    for (const locale of ['en', 'ja']) expect(localeMessages(locale)[key.toLowerCase()]?.message, locale).toBeTruthy();
  });

  it('says where the config is missing', () => {
    expect(stateKey({ state: 'no-config', configSource: 'base' })).toBe('popupStateNoConfig');
    expect(stateKey({ state: 'no-config', configSource: 'head' })).toBe('popupStateNoConfigHead');
    expect(stateKey({ state: 'ready', configSource: 'head' })).toBe('popupStateReady');
  });
});

describe('controlsFor', () => {
  it.each([
    ['inactive', false, false],
    ['excluded', false, false],
    ['loading', true, true],
    ['no-config', true, true],
    ['config-error', true, true],
    ['unsupported-ui', false, false],
    ['ready', true, true],
    ['error', false, false],
  ] as const)('offers the right buttons for %s', (state, hiding, headPreview) => {
    expect(controlsFor(pageStatus({ state }))).toEqual({ hiding, headPreview });
  });

  it('needs the repository and number for the preview', () => {
    expect(controlsFor(pageStatus({ repo: undefined }))).toEqual({ hiding: true, headPreview: false });
    expect(controlsFor(pageStatus({ pr: undefined }))).toEqual({ hiding: true, headPreview: false });
  });

  it('always offers to turn hiding back on and to stop the preview', () => {
    for (const state of STATES) {
      expect(controlsFor(pageStatus({ state, enabled: false })).hiding, state).toBe(true);
      expect(controlsFor(pageStatus({ state, headPreview: true, repo: undefined })).headPreview, state).toBe(true);
    }
  });
});
