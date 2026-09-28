// Loads the repository config for a pull request (plan 6.5). The base side
// is the default because the pull request author controls the head side.

import { CodeConfigIgnored, ConfigFileNames, type Diagnostic } from '@go-tebanare/engine';
import type { ConfigSource } from '../shared/messages.js';
import type { PullRequestContext } from './context.js';
import type { FetchFailureReason, SourceFetcher } from './fetcher.js';

export interface LoadedConfig {
  ok: true;
  path: string;
  yaml: string;
  /** A config-ignored warning for each later file name that also exists. */
  warnings: Diagnostic[];
}

/** The config could not be read, so the page hides nothing and shows reason. */
export interface ConfigFetchError {
  ok: false;
  path: string;
  reason: Exclude<FetchFailureReason, 'not-found'>;
  status?: number;
}

/** isConfigPath reports whether path, relative to the repository root, is a config file. */
export function isConfigPath(path: string): boolean {
  return ConfigFileNames.includes(path);
}

/**
 * loadConfig reads the first of ConfigFileNames that exists at the base
 * or head commit of ctx. It returns null when none exists. It requests
 * every name, as tebanare.SelectConfigFile checks every name, and adds a
 * warning for each later name that also exists. A fetch failure other than
 * not-found on a name that comes before the chosen one returns an error,
 * because that file might exist and win. A failure on a later name only
 * drops its warning.
 */
export async function loadConfig(
  fetcher: SourceFetcher,
  ctx: PullRequestContext,
  source: ConfigSource,
): Promise<LoadedConfig | ConfigFetchError | null> {
  const sha = source === 'head' ? ctx.headSha : ctx.baseSha;
  const results = await Promise.all(ConfigFileNames.map((name) => fetcher.fetchText(ctx.owner, ctx.repo, sha, name)));
  let found: LoadedConfig | null = null;
  for (const [i, res] of results.entries()) {
    const path = ConfigFileNames[i] ?? '';
    if (found) {
      if (res.ok) found.warnings.push(ignored(path, found.path));
      continue;
    }
    if (res.ok) {
      found = { ok: true, path, yaml: res.text, warnings: [] };
      continue;
    }
    if (res.reason !== 'not-found') {
      const err: ConfigFetchError = { ok: false, path, reason: res.reason };
      if (res.status !== undefined) err.status = res.status;
      return err;
    }
  }
  return found;
}

function ignored(path: string, used: string): Diagnostic {
  return { severity: 'warning', code: CodeConfigIgnored, message: `${path} is ignored because ${used} exists` };
}
