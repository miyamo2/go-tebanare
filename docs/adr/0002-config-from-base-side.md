# 0002 Read the configuration from the base side of a pull request

Status: Accepted

## Context

A `.gotebanare.yml` decides which code a reviewer does not see. If the
extension read it from the head of a pull request, the author could change
it to hide their own change in the same pull request.

## Decision

The extension reads the configuration at the commit of the old side of the
diff. It runs only on the full "Files changed" view, where the old side is
the merge base, so the author of the pull request cannot change that
commit. The React page's embedded data was checked on a saved page. When
GitHub's React app reaches "Files changed" without a page load, the page
keeps the embedded data of the page it loaded with, so the extension
fetches the "Files changed" URL and reads the embedded data of the copy the
server returns. The classic page's hidden inputs and diff URLs still need
spike S3 (ADR 0003) to confirm that they name the merge base.

It requests `.gotebanare.yml` and `.gotebanare.yaml` at once and uses the
first that exists in that order. When both exist, the page banner warns
that `.gotebanare.yml` is used. The order is the core constant
`tebanare.ConfigFileNames`.

The popup offers a "preview with head config" switch for people who edit
the configuration. It belongs to one tab and one pull request: it stays on
across reloads until it is turned off, the tab or the browser closes, or
the tab shows another pull request. While it is on, the popup says so, and
the page banner warns that the pull request author controls the
configuration once the head configuration file is found.

## Consequences

- When the base side has no configuration file, the page hides nothing and
  shows no banner, even when the pull request adds one. The head preview
  shows what it would do.
- When the configuration cannot be fetched (other than a 404), the page
  hides nothing and the banner shows the reason.
- A pull request that changes the configuration file, with a base
  configuration in use, shows a banner saying that the base configuration
  applies. Only Go files are analyzed, so the configuration file's own diff
  is never hidden.
- A configuration change on the base branch after the pull request branched
  does not apply until the pull request merges the base branch.
