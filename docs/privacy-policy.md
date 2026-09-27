# Privacy Policy for go-tebanare

Last updated: 2026-09-28

go-tebanare ("the extension") is a Chrome extension that hides
reviewer-agreed Go code in the "Files changed" tab of GitHub pull requests.

## Summary

- The extension does not collect, transmit, or sell any personal data.
- It does not use analytics, tracking, advertising, or crash-reporting SDKs.
- It does not communicate with any server other than `github.com`, using the
  browser's existing session on pages the user already has access to.
- All analysis runs locally in the browser.

## What the extension accesses

The extension's content script runs only on `https://github.com/*`. On a
pull request's "Files changed" page it:

- Reads the diff rows on the page.
- Fetches the repository's `.gotebanare.yml` (or `.yaml`) configuration file
  and the full old and new version of each changed file, from
  `https://github.com/<owner>/<repo>/raw/<sha>/<path>`.
- Uses the browser's existing GitHub session for these requests
  (`credentials: "same-origin"`); the extension requests no separate token
  and can read only what the signed-in user could already read on GitHub.
- Hides nothing where the configuration or a file isn't accessible, or a
  fetch fails (no access, SSO required, network error, or a file over
  1 MiB).

The extension requests only the `storage` permission. It has no
`host_permissions` beyond the content script's match on `github.com`, and it
does not request access to any other site.

## Where processing happens

- Diff matching runs inside the extension itself, in a WebAssembly module
  loaded by its background service worker.
- The only network requests the extension makes are the `github.com`
  fetches described above.

## What is stored, and where

- **Options** (`chrome.storage.sync`): the repositories excluded from
  hiding and a debug toggle. These are settings the user enters in the
  extension's options page. Chrome Sync may store this data on Google's
  servers as part of the user's own Chrome sync account, under Google's
  privacy policy; the developer has no access to it.
- **Analysis cache** (`chrome.storage.session`): hidden-line ranges and line
  hashes for files already analyzed on the current page, keyed by the
  engine version, a hash of the configuration, and the commit and path of
  each file side.
  - It never holds a whole source file, though a diagnostic message may
    quote up to 60 characters of normalized code for display.
  - Chrome clears this storage area when the browser closes; the developer
    has no access to it.
- **Per-tab hiding state** (`chrome.storage.session`): whether hiding is
  currently on or off for a given browser tab. Like the analysis cache,
  this never leaves the browser.

## Third parties

The extension does not share data with, or receive data from, any third
party.

## Changes to this policy

If this policy changes, the update will be committed to this file in the
project's repository, and the "Last updated" date above will change
accordingly.

## Contact

Raise questions or concerns as an issue at
<https://github.com/miyamo2/go-tebanare/issues>.
