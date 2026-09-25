import { describe, expect, it } from 'vitest';
import { ConfigError, type FileChangeInput } from '@go-tebanare/engine';
import { analyze, buildRecord, hashHiddenLines, sourceLines } from '../../src/background/analysis.js';
import { lineHash } from '../../src/shared/hash.js';
import { FakeEngine, range, result } from './fake-engine.js';

const oldSrc = ['package a', '', 'func (u *User) Name() string {', '\treturn u.name', '}', ''].join('\n');
const newSrc = ['package a', '', '// Name returns the name.', 'func (u *User) Name() string {', '\treturn u.name', '}', ''].join('\n');

describe('hashHiddenLines', () => {
  it('hashes the 1-based lines inside the ranges only', () => {
    expect(hashHiddenLines([range(3, 5)], oldSrc)).toEqual({
      '3': lineHash('func (u *User) Name() string {'),
      '4': lineHash('\treturn u.name'),
      '5': lineHash('}'),
    });
  });

  it('hashes CRLF lines like the rendered text, without the carriage return', () => {
    const crlf = 'package a\r\nfunc f() {}  \r\n';
    expect(hashHiddenLines([range(2, 2)], crlf)).toEqual({ '2': lineHash('func f() {}') });
  });

  it('skips line numbers outside the source and absent sides', () => {
    expect(hashHiddenLines([range(0, 1), range(5, 9)], oldSrc)).toEqual({ '1': lineHash('package a'), '5': lineHash('}') });
    expect(hashHiddenLines([range(1, 2)], null)).toEqual({});
    expect(hashHiddenLines([], oldSrc)).toEqual({});
  });

  it('gives no entry after the final newline, which ends the last line', () => {
    expect(Object.keys(hashHiddenLines([range(1, 3)], 'a\nb\n'))).toEqual(['1', '2']);
    expect(Object.keys(hashHiddenLines([range(1, 3)], 'a\nb'))).toEqual(['1', '2']);
    expect(hashHiddenLines([range(1, 3)], 'a\n\n')).toEqual({ '1': lineHash('a'), '2': lineHash('') });
    expect(hashHiddenLines([range(1, 1)], '')).toEqual({});
  });
});

describe('sourceLines', () => {
  it('counts lines like the engine', () => {
    expect(sourceLines('')).toEqual([]);
    expect(sourceLines('\n')).toEqual(['']);
    expect(sourceLines('a')).toEqual(['a']);
    expect(sourceLines('a\r\nb\r\n')).toEqual(['a\r', 'b\r']);
    expect(sourceLines(oldSrc)).toHaveLength(5);
  });
});

describe('buildRecord', () => {
  it('keeps the result and hashes each side from its own source', () => {
    const res = result([range(3, 5)], [range(3, 6)]);
    const rec = buildRecord(res, { old: oldSrc, new: newSrc });
    expect(rec.result).toBe(res);
    expect(Object.keys(rec.oldLines)).toEqual(['3', '4', '5']);
    expect(Object.keys(rec.newLines)).toEqual(['3', '4', '5', '6']);
    expect(rec.newLines['3']).toBe(lineHash('// Name returns the name.'));
    expect(rec.oldLines['3']).toBe(lineHash('func (u *User) Name() string {'));
  });
});

describe('analyze', () => {
  it('compiles, analyzes, and hashes the hidden lines of each side', async () => {
    const engine = new FakeEngine();
    let seen: FileChangeInput | undefined;
    engine.onAnalyze = (_rs, change) => {
      seen = change;
      return result([range(3, 5)], [range(4, 6)]);
    };
    const rec = await analyze(engine, 'version: 1', { oldPath: 'a.go', newPath: 'b.go', old: oldSrc, new: newSrc });
    expect(seen).toEqual({ oldPath: 'a.go', newPath: 'b.go', old: oldSrc, new: newSrc });
    expect(engine.calls).toEqual(['compile version: 1', 'analyze b.go']);
    expect(rec.newLines).toEqual({
      '4': lineHash('func (u *User) Name() string {'),
      '5': lineHash('\treturn u.name'),
      '6': lineHash('}'),
    });
  });

  it('keeps hit labels, which quote hidden code, and no full source', async () => {
    const src = [
      'package a',
      '',
      'func (r *Repo) Find(ctx context.Context, id string) (T, error) {',
      '\tlog.Printf("find %s", id)',
      '\tif err := r.check(ctx); err != nil {',
      '\t\treturn *new(T), fmt.Errorf("check %s: %w", id, err)',
      '\t}',
      '\treturn r.load(id)',
      '}',
      '',
    ].join('\n');
    // Labels as the engine builds them (testdata/analyze/canon-conversions):
    // the normalized node text, cut to 60 runes and "..." when longer.
    const logLabel = 'log.Printf("find %s", id)';
    const ifLabel = 'if err := r.check(ctx); err != nil { return *new(T), fmt.Err...';
    const engine = new FakeEngine();
    engine.onAnalyze = () =>
      result([], [
        { start: 4, end: 4, hits: [{ ruleId: 'trace', target: 'stmt', node: 'ExprStmt', label: logLabel }] },
        { start: 5, end: 7, hits: [{ ruleId: 'iferr', target: 'stmt', node: 'IfStmt', label: ifLabel, preset: 'iferr' }] },
      ]);
    const rec = await analyze(engine, 'version: 1', { newPath: 'a.go', old: null, new: src });
    expect(rec.result.new.flatMap((r) => r.hits.map((h) => h.label))).toEqual([logLabel, ifLabel]);
    expect(Object.keys(rec.newLines)).toEqual(['4', '5', '6', '7']);
    // The labels quote hidden code: all of line 4, the start of line 6.
    const stored = JSON.stringify(rec);
    expect(stored).toContain(JSON.stringify(logLabel));
    expect(stored).toContain(JSON.stringify(ifLabel));
    expect(stored).not.toContain('check %s: %w');
    expect(stored).not.toContain('return r.load(id)');
    expect(stored).not.toContain('func (r *Repo) Find');
  });

  it('passes an added file with no old side and no old path', async () => {
    const engine = new FakeEngine();
    let seen: FileChangeInput | undefined;
    engine.onAnalyze = (_rs, change) => {
      seen = change;
      return result([], [range(1, 1)]);
    };
    const rec = await analyze(engine, 'version: 1', { newPath: 'a.go', old: null, new: newSrc });
    expect(seen).toEqual({ newPath: 'a.go', old: null, new: newSrc });
    expect(rec.oldLines).toEqual({});
    expect(rec.newLines).toEqual({ '1': lineHash('package a') });
  });

  it('rejects with ConfigError for an invalid config without analyzing', async () => {
    const engine = new FakeEngine();
    await expect(analyze(engine, 'invalid', { newPath: 'a.go', old: null, new: newSrc })).rejects.toBeInstanceOf(ConfigError);
    expect(engine.calls).toEqual(['compile invalid']);
  });
});
