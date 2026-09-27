# 0003 Fetch sources with the browser's GitHub session

Status: Accepted

## Context

The analyzer needs the full old and new file, since a diff hunk cannot be
parsed on its own. Private repositories need authentication.

## Decision

The content script fetches `https://github.com/<owner>/<repo>/raw/<sha>/<path>`
from the page's origin, so the browser sends the user's GitHub session. The
configuration file (ADR 0002) comes the same way. The extension asks for no
token; its only permission is `storage`, and its content script runs on
`https://github.com/*`. The fetch code sits behind the `SourceFetcher`
interface. Each tab runs at most four requests at once and skips files over
1 MiB.

## Consequences

- The extension reads only what the user can already read on GitHub.
- A failed source fetch (not signed in, no access, SSO required, network
  error, too large) hides nothing in that file and shows the reason in a
  banner. A failed configuration fetch hides nothing on the page.
- The background caches analysis results in `chrome.storage.session`, keyed
  by engine version, the SHA-256 of the configuration, and the commit and
  path of each side. A record holds line ranges and line hashes, never a
  whole source, but its labels and diagnostics can quote up to 60
  characters of normalized hidden code. Chrome drops the area when the
  browser closes; when it is full, the extension clears every analysis
  record.
- Spike S3 confirmed the design against a private repository: the raw URL
  redirects the same way as for a public one, `credentials: "same-origin"`
  lets CORS through on the redirected response, and a required SSO
  authorization surfaces as a failed fetch rather than a silent success. The
  session is enough; no personal access token is needed.
