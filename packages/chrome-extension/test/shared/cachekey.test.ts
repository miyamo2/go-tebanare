import { describe, expect, it } from 'vitest';
import { ANALYSIS_KEY_PREFIX, analysisKey } from '../../src/shared/cachekey.js';

const old = 'a'.repeat(40);
const neu = 'b'.repeat(40);

describe('analysisKey', () => {
  it('joins every part in order', () => {
    const key = analysisKey({ engineVersion: 'v1.2.3', configKey: 'cfg', oldSha: old, oldPath: 'x/a.go', newSha: neu, newPath: 'x/b.go' });
    expect(key).toBe(`analysis:v1:v1.2.3:cfg:${old}:x/a.go:${neu}:x/b.go`);
    expect(key.startsWith(ANALYSIS_KEY_PREFIX)).toBe(true);
  });

  it('leaves absent sides empty', () => {
    expect(analysisKey({ engineVersion: 'dev', configKey: 'cfg', newSha: neu, newPath: 'a.go' })).toBe(
      `analysis:v1:dev:cfg:::${neu}:a.go`,
    );
    expect(analysisKey({ engineVersion: 'dev', configKey: 'cfg', oldSha: old, oldPath: 'a.go' })).toBe(
      `analysis:v1:dev:cfg:${old}:a.go::`,
    );
  });

  it('keeps paths with ":" from colliding with other changes', () => {
    const deleted = analysisKey({ engineVersion: 'dev', configKey: 'c', oldSha: old, oldPath: `a:${neu}:b` });
    const modified = analysisKey({ engineVersion: 'dev', configKey: 'c', oldSha: old, oldPath: 'a', newSha: neu, newPath: 'b' });
    expect(deleted).not.toBe(modified);
    expect(analysisKey({ engineVersion: 'dev', configKey: 'c', newPath: '50%:x' })).toBe('analysis:v1:dev:c::::50%25%3Ax');
  });
});
