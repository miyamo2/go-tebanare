// Processes one file of the diff for the controller (plan 6.2, 6.4): the
// cache lookup, the source fetches on a miss, the analyze request, and the
// checks that decide whether the file hides anything.

import { analysisKey } from '../../shared/cachekey.js';
import { lineHash } from '../../shared/hash.js';
import type { AnalysisRecord, BgRequest, BgResponse, ChangeInput } from '../../shared/messages.js';
import { applyFile, clearFile, type ApplyContext, type ApplySummary } from '../apply.js';
import type { PullRequestContext } from '../context.js';
import type { FileStatus, RowRef } from '../dom/variant.js';
import type { FetchResult, SourceFetcher } from '../fetcher.js';
import { planVisibility } from '../plan.js';
import type { Notice } from '../ui/banner.js';
import { verifyHiddenRows } from '../verify.js';

/** SendFn delivers a request to the background, as shared/messages.ts send() does. */
export type SendFn = <R extends BgRequest>(req: R) => Promise<BgResponse<R['type']>>;

/** A file of the diff as the page shows it. oldPath is set for renames only. */
export interface DiffFile {
  path: string;
  oldPath?: string;
  status: FileStatus;
}

/** The compiled config that every file of one run is analyzed with. */
export interface RunConfig {
  ctx: PullRequestContext;
  yaml: string;
  configKey: string;
  engineVersion: string;
}

/** The outcome of analyzeFile. A null record hides nothing, and notice says why. */
export interface FileAnalysis {
  record: AnalysisRecord | null;
  notice?: Notice;
}

interface Side {
  sha: string;
  path: string;
}

/**
 * sides returns the commits and paths of the sides the file has: an added
 * file has no old side and a deleted file has no new side.
 */
function sides(file: DiffFile, ctx: PullRequestContext): { old?: Side; new?: Side } {
  const out: { old?: Side; new?: Side } = {};
  if (file.status !== 'added') out.old = { sha: ctx.baseSha, path: file.oldPath ?? file.path };
  if (file.status !== 'deleted') out.new = { sha: ctx.headSha, path: file.path };
  return out;
}

/** isGoFile reports whether every side of file is a .go file, so the engine can analyze it. */
export function isGoFile(file: DiffFile): boolean {
  const paths = file.status === 'modified' ? [file.oldPath ?? file.path, file.path] : [file.path];
  return paths.every((p) => p.endsWith('.go'));
}

/** fileKey identifies a file within one pull request. */
export function fileKey(file: DiffFile): string {
  return JSON.stringify([file.status, file.oldPath ?? '', file.path]);
}

/**
 * rowsKey identifies the rows a container shows by their kinds, line
 * numbers, and the lineHash of their text, so a row whose code changes in
 * place is checked against the analysis again (plan 6.4).
 */
export function rowsKey(rows: readonly RowRef[]): string {
  return rows.map((r) => `${r.kind}:${r.oldLine ?? ''}:${r.newLine ?? ''}:${r.text === undefined ? '' : lineHash(r.text)}`).join(',');
}

/** cacheKeyOf returns the analysis cache key of file under config (plan 6.4). */
export function cacheKeyOf(file: DiffFile, config: RunConfig): string {
  const s = sides(file, config.ctx);
  return analysisKey({
    engineVersion: config.engineVersion,
    configKey: config.configKey,
    oldSha: s.old?.sha,
    oldPath: s.old?.path,
    newSha: s.new?.sha,
    newPath: s.new?.path,
  });
}

/**
 * analyzeFile returns the analysis of file: the cached record when there is
 * one, or else the record the background builds from both fetched sides.
 * A failed fetch or analysis gives a null record with a notice; a failed
 * lookup counts as a cache miss. When stale returns true before a request,
 * analyzeFile stops with a null record. It never rejects.
 */
export async function analyzeFile(
  send: SendFn,
  fetcher: SourceFetcher,
  config: RunConfig,
  file: DiffFile,
  stale: () => boolean = () => false,
): Promise<FileAnalysis> {
  if (stale()) return { record: null };
  const cacheKey = cacheKeyOf(file, config);
  const cached = await send({ type: 'lookup', cacheKey });
  if (cached.ok && cached.record) return checked(file, cached.record);
  if (stale()) return { record: null };

  const { ctx } = config;
  const s = sides(file, ctx);
  const get = (side: Side | undefined): Promise<FetchResult | null> =>
    side ? fetcher.fetchText(ctx.owner, ctx.repo, side.sha, side.path).catch((): FetchResult => ({ ok: false, reason: 'network' })) : Promise.resolve(null);
  const [oldRes, newRes] = await Promise.all([get(s.old), get(s.new)]);
  if (stale()) return { record: null };
  for (const res of [oldRes, newRes]) {
    if (res && !res.ok) return { record: null, notice: { kind: 'fetch-failed', path: file.path, reason: res.reason } };
  }

  const change: ChangeInput = { old: oldRes?.ok ? oldRes.text : null, new: newRes?.ok ? newRes.text : null };
  if (s.old) change.oldPath = s.old.path;
  if (s.new) change.newPath = s.new.path;
  const analyzed = await send({ type: 'analyze', yaml: config.yaml, cacheKey, change });
  if (!analyzed.ok) return { record: null, notice: { kind: 'analysis-failed', path: file.path, error: analyzed.error } };
  return checked(file, analyzed.record);
}

/**
 * checked turns an engine crash, which the engine reports as a result with
 * skipped "engine-error", into a notice. Other skip reasons (parse errors,
 * size and depth limits) hide nothing without a notice, as plan 4.10 says.
 */
function checked(file: DiffFile, record: AnalysisRecord): FileAnalysis {
  if (record.result.skipped !== 'engine-error') return { record };
  const error = record.result.diagnostics.find((d) => d.severity === 'error')?.message ?? 'engine-error';
  return { record: null, notice: { kind: 'analysis-failed', path: file.path, error } };
}

/** What showFile did with one container. */
export type ShowResult = { verified: true; summary: ApplySummary } | { verified: false };

/**
 * showFile plans record over rows and checks the rows to hide against the
 * record's line hashes. On a mismatch it clears the container. Otherwise
 * it applies the plan when enabled is true and clears the container when
 * it is false.
 */
export function showFile(
  container: HTMLElement,
  rows: readonly RowRef[],
  record: AnalysisRecord,
  enabled: boolean,
  ctx: Omit<ApplyContext, 'result'>,
): ShowResult {
  const plan = planVisibility(rows, record.result);
  if (!verifyHiddenRows(rows, plan, record)) {
    clearFile(container);
    return { verified: false };
  }
  if (!enabled) {
    clearFile(container);
    return { verified: true, summary: { folds: 0, lines: 0 } };
  }
  return { verified: true, summary: applyFile(container, rows, plan, { ...ctx, result: record.result }) };
}
