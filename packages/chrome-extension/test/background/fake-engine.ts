// A fake Engine for the background tests. compile() treats YAML containing
// "invalid" as a bad config; analyzeChange() answers from a replaceable
// function.

import {
  ConfigError,
  type ChangeResult,
  type CompiledRuleset,
  type Engine,
  type FileChangeInput,
  type PresetInfo,
  type Range,
} from '@go-tebanare/engine';

export const FAKE_ENGINE_VERSION = 'v0.0.0-test';

export const badConfigDiagnostic = { severity: 'error', code: 'config-invalid', message: 'unknown preset "nope"', field: 'presets[0](nope)', line: 3 } as const;

export function range(start: number, end: number): Range {
  return { start, end, hits: [{ ruleId: 'getter', target: 'func', node: 'FuncDecl', label: `func ${start}` }] };
}

export function result(old: Range[], nu: Range[], skipped: ChangeResult['skipped'] = ''): ChangeResult {
  return { old, new: nu, diagnostics: [], skipped };
}

export class FakeEngine implements Engine {
  readonly engineVersion = FAKE_ENGINE_VERSION;
  readonly apiVersion = 1;
  readonly configFileNames: readonly string[] = ['.gotebanare.yml', '.gotebanare.yaml'];
  /** One entry per call: the method name and its main argument. */
  readonly calls: string[] = [];
  disposed = false;
  onAnalyze: (rs: CompiledRuleset, change: FileChangeInput) => ChangeResult = () => result([], []);

  async compile(yaml: string): Promise<CompiledRuleset> {
    this.calls.push(`compile ${yaml}`);
    if (yaml.includes('invalid')) throw new ConfigError(badConfigDiagnostic.message, [{ ...badConfigDiagnostic }]);
    return { key: `key-${yaml.length}`, yaml, diagnostics: [], rules: [{ id: 'getter', target: 'func', preset: 'getter' }] };
  }

  async analyzeChange(rs: CompiledRuleset, change: FileChangeInput): Promise<ChangeResult> {
    this.calls.push(`analyze ${change.newPath ?? change.oldPath ?? ''}`);
    return this.onAnalyze(rs, change);
  }

  async presets(): Promise<PresetInfo[]> {
    return [];
  }

  dispose(): void {
    this.disposed = true;
  }
}
