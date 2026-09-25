// Loads and compiles the config of one controller run (plan 6.2 step 3,
// plan 6.5) and adds the page notices that the config calls for.

import type { ConfigSource } from '../../shared/messages.js';
import { loadConfig } from '../config.js';
import type { PullRequestContext } from '../context.js';
import type { SourceFetcher } from '../fetcher.js';
import type { SendFn } from './file.js';
import type { Compiled } from './scan.js';
import type { Report } from './status.js';

/** The outcome of setUpConfig: a compiled config, or the state the page ends in. */
export type Setup =
  | { state: 'ready'; configPath: string; config: Compiled }
  | { state: 'no-config' | 'config-error' | 'error'; configPath?: string };

/**
 * setUpConfig loads the config of ctx from source and compiles it through
 * the background. A missing config gives no-config. A fetch failure gives
 * error and a config with errors gives config-error, each with a notice in
 * report. It returns null when stale returns true after a request.
 */
export async function setUpConfig(
  fetcher: SourceFetcher,
  send: SendFn,
  ctx: PullRequestContext,
  source: ConfigSource,
  report: Report,
  stale: () => boolean,
): Promise<Setup | null> {
  const cfg = await loadConfig(fetcher, ctx, source);
  if (stale()) return null;
  if (cfg === null) return { state: 'no-config' };
  const configPath = cfg.path;
  // Plan 6.5: the preview banner stays while the preview is on.
  if (source === 'head') report.addPage({ kind: 'head-preview', path: configPath });
  if (!cfg.ok) {
    report.addPage({ kind: 'config-fetch-failed', path: configPath, reason: cfg.reason });
    return { state: 'error', configPath };
  }
  if (cfg.warnings.length > 0) report.addPage({ kind: 'both-configs' });
  const res = await send({ type: 'compile', yaml: cfg.yaml });
  if (stale()) return null;
  if (!res.ok) {
    // A Failure without diagnostics comes from the transport or an engine crash.
    if (res.diagnostics) {
      report.addPage({ kind: 'config-error', path: configPath, diagnostics: res.diagnostics });
      return { state: 'config-error', configPath };
    }
    report.addPage({ kind: 'analysis-failed', path: configPath, error: res.error });
    return { state: 'error', configPath };
  }
  const config: Compiled = { ctx, yaml: cfg.yaml, configKey: res.configKey, engineVersion: res.engineVersion, rules: res.rules };
  return { state: 'ready', configPath, config };
}
