# 0003 Fetch sources with the browser's GitHub session

Status: Accepted, pending spike S3

## Context

The analyzer needs the full old and new file, since a diff hunk cannot be
parsed on its own. Private repositories need authentication.

## Decision

The content script fetches `https://github.com/<owner>/<repo>/raw/<sha>/<path>`
from the page's origin, so the browser sends the user's GitHub session. The
configuration file (ADR 0002) comes the same way. The extension asks for no
token and no host permissions. The fetch code sits behind the `SourceFetcher`
interface, and at most four requests run at once.

## Consequences

- The extension reads only what the user can already read on GitHub.
- A failed fetch (not signed in, no access, SSO required, network error)
  hides nothing in that file and shows the reason in a banner.
- Results are cached in `chrome.storage.session` by configuration hash,
  commit, and path. Source text is never stored.
- Spike S3 has not run: this environment cannot reach github.com. It must
  confirm how the raw URL redirects for private repositories, whether CORS
  allows reading the redirected response with `credentials: "same-origin"`,
  and how SSO failures look. If the session is not enough, a personal access
  token becomes an optional setting.
