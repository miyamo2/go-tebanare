// Data types exchanged with the wasm engine. They mirror the JSON that the Go
// packages internal/result, internal/presets, and the root package tebanare
// produce. Change both sides together; the contract test
// compares them against the Go golden files. The constants and string types
// come from generated/constants.ts, which dev/tsconstgen writes from the Go
// declarations.

import type { SideNew, SideOld, Severity, SkipReason as GoSkipReason, Target } from './generated/constants.js';

export type { Severity, Target };

/** Why a file was not analyzed. "engine-error" exists only on the TS side. */
export type SkipReason = GoSkipReason | 'engine-error';

/** One rule match that produced (part of) a hidden range. */
export interface Hit {
  ruleId: string;
  target: Target;
  /** go/ast type name of the matched node, such as "FuncDecl". */
  node: string;
  label: string;
  preset?: string;
}

/** A closed, 1-based line range. */
export interface Range {
  start: number;
  end: number;
  hits: Hit[];
}

export interface Diagnostic {
  severity: Severity;
  code: string;
  message: string;
  field?: string;
  side?: typeof SideOld | typeof SideNew;
  ruleId?: string;
  line?: number;
  column?: number;
}

/** The outcome of analyzing one changed file. */
export interface ChangeResult {
  old: Range[];
  new: Range[];
  diagnostics: Diagnostic[];
  skipped: SkipReason;
}

export interface RuleInfo {
  id: string;
  description?: string;
  target: Target;
  preset?: string;
}

export interface SettingInfo {
  name: string;
  type: string;
  default: string;
  description: string;
  enum?: string[];
}

export interface PresetExample {
  settings?: string;
  code: string;
  match: boolean;
  note?: string;
}

export interface PresetInfo {
  name: string;
  summary: string;
  criteria: string[];
  kind: string;
  settings: SettingInfo[];
  examples: PresetExample[];
}

export interface EngineInfo {
  apiVersion: number;
  engineVersion: string;
  /** tebanare.ConfigFileNames: the configuration file names in lookup order. */
  configFileNames: string[];
}

/** A configuration that compiled without errors. */
export interface CompiledRuleset {
  /** Lowercase hex SHA-256 of the UTF-8 YAML bytes. */
  readonly key: string;
  readonly yaml: string;
  /** Warnings. Errors reject compile() with a ConfigError instead. */
  readonly diagnostics: Diagnostic[];
  readonly rules: RuleInfo[];
}

/** One changed file. A side that is null or undefined is absent. */
export interface FileChangeInput {
  oldPath?: string;
  newPath?: string;
  old?: string | Uint8Array | null;
  new?: string | Uint8Array | null;
}

/** Rejection reason of Engine.compile for an invalid configuration. */
export class ConfigError extends Error {
  readonly diagnostics: Diagnostic[];

  constructor(message: string, diagnostics: Diagnostic[]) {
    super(message);
    this.name = 'ConfigError';
    this.diagnostics = diagnostics;
  }
}

/** An error reported by the engine for a valid call, such as a bad handle. */
export class EngineError extends Error {
  constructor(message: string) {
    super(message);
    this.name = 'EngineError';
  }
}
