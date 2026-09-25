# 0002 Read the configuration from the base side of a pull request

Status: Accepted

## Context

A `.gotebanare.yml` decides which code a reviewer does not see. If the
extension read it from the head of a pull request, the author could change
it to hide their own change in the same pull request.

## Decision

The extension reads the configuration at the commit of the old side of the
diff, which is the merge base for the full "Files changed" view. The author
of the pull request cannot change that commit. It looks for
`.gotebanare.yml`, then `.gotebanare.yaml`, and warns when both exist.

The popup offers a temporary "preview with head config" switch for people
who edit the configuration. While it is on, the page shows a banner that
says so. The switch belongs to one tab and one pull request.

## Consequences

- A pull request that changes the configuration file shows a banner saying
  that the base configuration applies. The configuration file's own diff is
  never hidden.
- A configuration change on the base branch after the pull request branched
  does not apply until the pull request merges the base branch.
