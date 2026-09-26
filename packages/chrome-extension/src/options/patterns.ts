// Checks the excluded repository patterns typed on the options page. The
// accepted shapes are the ones isExcluded (shared/settings.ts) matches.

import { isOwnerName, isRepoName } from '../content/page.js';

/** A textarea line that is not a pattern. line counts from 1, blank lines included. */
export interface InvalidLine {
  line: number;
  text: string;
}

export interface CheckedPatterns {
  /** The valid patterns, trimmed, in textarea order. */
  patterns: string[];
  invalid: InvalidLine[];
}

/**
 * isValidPattern reports whether p is "owner/name", "owner/*", or "*",
 * where owner and name follow the rules parsePullUrl (content/page.ts)
 * applies to pull request URLs.
 */
export function isValidPattern(p: string): boolean {
  if (p === '*') return true;
  const parts = p.split('/');
  if (parts.length !== 2) return false;
  const [owner = '', name = ''] = parts;
  return isOwnerName(owner) && (name === '*' || isRepoName(name));
}

/** checkPatterns splits the textarea into lines and sorts them into patterns and invalid lines. */
export function checkPatterns(text: string): CheckedPatterns {
  const out: CheckedPatterns = { patterns: [], invalid: [] };
  text.split('\n').forEach((raw, i) => {
    const p = raw.trim();
    if (p === '') return;
    if (isValidPattern(p)) out.patterns.push(p);
    else out.invalid.push({ line: i + 1, text: p });
  });
  return out;
}
