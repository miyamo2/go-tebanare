/** Every analysis cache key starts with this prefix. */
export const ANALYSIS_KEY_PREFIX = 'analysis:';

export interface AnalysisKeyParts {
  engineVersion: string;
  /** CompiledRuleset.key: SHA-256 of the config YAML. */
  configKey: string;
  oldSha?: string;
  oldPath?: string;
  newSha?: string;
  newPath?: string;
}

/**
 * analysisKey builds the chrome.storage.session key of one analysis result
 * (plan 6.4): "analysis:v1:" followed by the parts joined with ":". Absent
 * parts are empty. "%" and ":" inside a part are percent-encoded so that a
 * path containing ":" cannot produce the key of a different change.
 */
export function analysisKey(p: AnalysisKeyParts): string {
  const parts = [p.engineVersion, p.configKey, p.oldSha, p.oldPath, p.newSha, p.newPath];
  return `${ANALYSIS_KEY_PREFIX}v1:${parts.map(escapePart).join(':')}`;
}

function escapePart(s: string | undefined): string {
  return (s ?? '').replaceAll('%', '%25').replaceAll(':', '%3A');
}
