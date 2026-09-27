// Recognizes the pull request pages where the content script starts.

/** A pull request "Files changed" page, identified by its URL. */
export interface PullPage {
  owner: string;
  repo: string;
  number: number;
}

const OWNER = /^[A-Za-z0-9][A-Za-z0-9-]{0,38}$/;
const REPO = /^[A-Za-z0-9._-]{1,100}$/;
const NUMBER = /^[1-9][0-9]{0,9}$/;
const TABS: ReadonlySet<string> = new Set(['files', 'changes']);

/**
 * isOwnerName reports whether s is a GitHub account name: alphanumerics and
 * hyphens, at most 39 characters, starting with an alphanumeric.
 */
export function isOwnerName(s: string): boolean {
  return OWNER.test(s);
}

/** isRepoName reports whether s is a repository name: alphanumerics, ".", "-", and "_", at most 100 characters. */
export function isRepoName(s: string): boolean {
  return REPO.test(s);
}

/**
 * parsePullUrl returns the pull request of a
 * https://github.com/<owner>/<repo>/pull/<n>/files or .../changes URL. A
 * trailing slash, a query, and a hash are allowed. Every other URL returns
 * null, including the commit range views /files/<a>..<b> and
 * /commits/<sha>, which plan Phase 4 adds.
 */
export function parsePullUrl(url: string): PullPage | null {
  let u: URL;
  try {
    u = new URL(url);
  } catch {
    return null;
  }
  if (u.protocol !== 'https:' || u.host !== 'github.com') return null;
  // "/o/r/pull/1/files" splits into 6 parts, the first one empty; a
  // trailing slash adds a 7th empty part.
  const parts = u.pathname.split('/');
  if (parts.length === 7 && parts[6] === '') parts.pop();
  if (parts.length !== 6) return null;
  const [, owner = '', repo = '', pull, num = '', tab = ''] = parts;
  if (pull !== 'pull' || !TABS.has(tab)) return null;
  if (!isOwnerName(owner) || !isRepoName(repo) || !NUMBER.test(num)) return null;
  return { owner, repo, number: Number(num) };
}

/**
 * isSignedOut reports whether doc is a github.com page for a signed-out
 * visitor. GitHub gives the body the class "logged-out" and an empty
 * user-login meta tag on those pages. A page with neither counts as signed
 * in, so the banner never tells a signed-in user to sign in.
 */
export function isSignedOut(doc: Document): boolean {
  if (doc.body?.classList.contains('logged-out')) return true;
  const login = doc.querySelector<HTMLMetaElement>('meta[name="user-login"]');
  return login !== null && login.content.trim() === '';
}
