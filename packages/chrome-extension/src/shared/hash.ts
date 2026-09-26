// Line hashes let the content script check that the text GitHub renders
// matches the source the engine analyzed (plan 6.4), without storing source.

/** normalizeLine removes trailing carriage returns, spaces, and tabs. */
export function normalizeLine(s: string): string {
  let end = s.length;
  while (end > 0) {
    const c = s.charCodeAt(end - 1);
    // A loop instead of /[ \t\r]+$/ keeps long runs of blanks linear.
    if (c !== 0x0d && c !== 0x20 && c !== 0x09) break;
    end--;
  }
  return end === s.length ? s : s.slice(0, end);
}

/** fnv1a32 returns FNV-1a over the UTF-16 code units of s as 8 lowercase hex digits. */
export function fnv1a32(s: string): string {
  let h = 0x811c9dc5;
  for (let i = 0; i < s.length; i++) {
    h ^= s.charCodeAt(i);
    h = Math.imul(h, 0x01000193);
  }
  return (h >>> 0).toString(16).padStart(8, '0');
}

/** lineHash hashes a line after normalizeLine. */
export function lineHash(s: string): string {
  return fnv1a32(normalizeLine(s));
}
