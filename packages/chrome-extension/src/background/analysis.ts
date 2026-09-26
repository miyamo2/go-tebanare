// Analyzes one changed file for the content script and turns the result
// into an AnalysisRecord (plan 6.4). The record holds a hash of every
// hidden source line, so the content script can compare the rendered diff
// with the analyzed source, also when the record comes from the cache.
// The record holds no full source. Hit labels and some diagnostic messages
// in the engine result quote up to 60 runes of normalized hidden code, such
// as `log.Printf("find %s", id)`, for the fold tooltip, and AnalysisCache
// stores them with the record.

import type { ChangeResult, CompiledRuleset, FileChangeInput, Range } from '@go-tebanare/engine';
import { lineHash } from '../shared/hash.js';
import type { AnalysisRecord, ChangeInput } from '../shared/messages.js';

/** The engine calls that analyze() makes. EngineHost and Engine both provide them. */
export interface Analyzer {
  compile(yaml: string): Promise<CompiledRuleset>;
  analyzeChange(rs: CompiledRuleset, change: FileChangeInput): Promise<ChangeResult>;
}

/**
 * analyze compiles yaml, analyzes change, and builds its record. An invalid
 * config rejects with the engine's ConfigError.
 */
export async function analyze(engine: Analyzer, yaml: string, change: ChangeInput): Promise<AnalysisRecord> {
  const rs = await engine.compile(yaml);
  const input: FileChangeInput = { old: change.old, new: change.new };
  if (change.oldPath !== undefined) input.oldPath = change.oldPath;
  if (change.newPath !== undefined) input.newPath = change.newPath;
  const result = await engine.analyzeChange(rs, input);
  return buildRecord(result, change);
}

/** buildRecord pairs result with the hashes of the lines it hides on each side. */
export function buildRecord(result: ChangeResult, sources: Pick<ChangeInput, 'old' | 'new'>): AnalysisRecord {
  return {
    result,
    oldLines: hashHiddenLines(result.old, sources.old),
    newLines: hashHiddenLines(result.new, sources.new),
  };
}

/**
 * hashHiddenLines maps each 1-based line number inside ranges to the
 * lineHash of that line of src. A final "\n" ends the last line and does
 * not start a new one, as in the engine. Line numbers past the last line
 * get no entry, so the content script's check fails for them. An absent
 * side (null) yields no entries.
 */
export function hashHiddenLines(ranges: readonly Range[], src: string | null): Record<string, string> {
  const out: Record<string, string> = {};
  if (src === null || ranges.length === 0) return out;
  const lines = sourceLines(src);
  for (const r of ranges) {
    const end = Math.min(r.end, lines.length);
    for (let n = Math.max(r.start, 1); n <= end; n++) {
      out[String(n)] = lineHash(lines[n - 1] ?? '');
    }
  }
  return out;
}

/** sourceLines splits src into lines. "a\nb\n" and "a\nb" have 2 lines, "" has none. */
export function sourceLines(src: string): string[] {
  const lines = src.split('\n');
  if (lines[lines.length - 1] === '') lines.pop();
  return lines;
}
