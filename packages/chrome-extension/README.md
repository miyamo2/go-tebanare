# @go-tebanare/chrome-extension

The Chrome extension (Manifest V3) that folds reviewer-agreed Go code in the "Files changed" tab of GitHub pull requests. The content script reads the diff rows, the service worker runs the wasm engine from `@go-tebanare/engine`, and a row is hidden only when its text matches the source the engine analyzed.

## Build

1. Run `make wasm` at the repository root. It writes `packages/engine/wasm/engine.wasm`.
2. Run `bun install` and then `bun run --filter @go-tebanare/chrome-extension build` at the repository root. It writes the unpacked extension to `dist/`, with a copy of `engine.wasm`. The `zip` script also writes `dist.zip`.

To try it, turn on Developer mode in `chrome://extensions` and load `dist/` with "Load unpacked".

## Release

Merge a change to `main` that raises `version` in `package.json`, for example to `0.2.0`. The Chrome Web Store needs a plain version, higher than the last upload. You need not edit `manifest.json`; the workflow commits it.

1. `.github/workflows/tag-chrome-extension.yml` checks the version with `scripts/release-version.mjs`, commits it to `manifest.json` on `main` with `scripts/sync-manifest-version.mjs`, and tags that commit `packages/chrome-extension/v0.2.0`.
2. The tag starts `.github/workflows/release-chrome-extension.yml`. After all of CI passes on the tag, it attaches the CI build of `dist.zip` as `go-tebanare-chrome-extension-v0.2.0.zip`, with `SHA256SUMS`, to the GitHub release of the tag. For a prerelease, the GitHub release is marked as a prerelease and its notes start at the previous release of any kind; otherwise they start at the previous plain release.
3. For a plain version only, upload that zip in the Chrome Web Store developer dashboard.

To try a version before the store gets it, raise `version` to a prerelease such as `0.2.0-rc.1` first. The workflows run the same steps, mark the GitHub release as a prerelease, and skip the Chrome Web Store. The zip holds manifest `version` `0.2.0` and `version_name` `0.2.0-rc.1`, so you can load it unpacked. Each release must be higher than every earlier one in semver order: `0.2.0-rc.2` and then `0.2.0` can follow `0.2.0-rc.1`, but no prerelease of `0.2.0` can follow `0.2.0`. The check rejects build metadata such as `+build.1`.

The tag workflow pushes as the GitHub App `go-tebanare-release`. It needs the variable `RELEASE_APP_ID` and the secret `RELEASE_APP_PRIVATE_KEY`, Contents: Read and write, and permission to bypass any protection of `main`.

A push to `main` that leaves the version as it was tags nothing. To tag the version already on `main`, for the first release for example, run the tag workflow by hand from the Actions tab.

If CI fails on the tag, no release exists yet: fix the cause on `main`, delete the tag, and run the tag workflow by hand again. A release run that failed after CI passed can be retried by running the release workflow by hand on the tag; it finishes a draft and leaves a published release alone. The repository uses immutable releases, so a published release keeps its tag and assets. To replace one, raise the version. A tag pushed by hand also starts the release workflow, which checks that the tag is on `main`, matches `package.json` and `manifest.json` (without a prerelease suffix), and is higher than every other release.

The release workflow holds a commented-out `publish` job that uploads the zip with the Chrome Web Store API and submits it for review. It stays commented out until someone uploads the first zip by hand in the developer dashboard, which creates the store item. The comment above the job lists the settings it needs, and its `if` condition skips prereleases.

## Tests

| Command | Runs |
|---|---|
| `bun run --filter @go-tebanare/chrome-extension typecheck` | `tsc` over `src/`, then over `test/`, `e2e/`, and the configs |
| `bun run --filter @go-tebanare/chrome-extension lint` | ESLint over the package |
| `bun run --filter @go-tebanare/chrome-extension test` | the unit tests in `test/` (vitest, happy-dom for DOM tests) |
| `bun run --filter @go-tebanare/chrome-extension e2e` | the end-to-end tests in `e2e/` (Playwright) |

## End-to-end tests

`e2e/global-setup.ts` builds `dist/` before the tests and stops when `dist/engine.wasm` is missing, so run `make wasm` once first. Each test starts Chromium with a new profile and loads `dist/` as an unpacked extension, in the new headless mode of Playwright's `chromium` channel.

Playwright needs the Chromium build that matches `@playwright/test` 1.56.1. Install it once:

```sh
bun run --cwd packages/chrome-extension playwright install chromium
```

Skip this step where `PLAYWRIGHT_BROWSERS_PATH` already points to a directory with that build.

To watch the browser, pass `--headed`. Without a display, run it under Xvfb:

```sh
xvfb-run -a bun run --cwd packages/chrome-extension e2e --headed
```

The tests never contact github.com. `context.route` answers every `https://github.com/**` request from `e2e/fixtures/`, after the test has changed the page or the files where it needs to:

| Request | Answer |
|---|---|
| `/acme/widgets/pull/7/files` | `pull-7-files.html` |
| `/acme/widgets/raw/<sha>/<path>` | the file under `testdata/base/` or `testdata/head/`, chosen by the commit the page names |
| anything else, including `.gotebanare.yaml` | 404 |

`extension.spec.ts` checks that:

- the getter's rows and the 3-line `if err != nil` block are hidden under fold rows that summarize the rules on one line (their tooltips give the details), and the other new code stays visible;
- a fold row shows its rows and hides them again, and the button in the file header shows every fold and hides them all again;
- turning hiding off with the popup's `set-tab-state` request shows every row, and the `toggle-hiding` command hides them again.

The popup request is sent from `popup.html` opened in a tab. The command is fired in the service worker with `chrome.commands.onCommand.dispatch`, which Chromium exposes but the typed API leaves out.

`failsafe.spec.ts` checks that the extension hides nothing when:

- the base commit has no config, even though the head commit adds one;
- the config has a YAML syntax error (`broken.gotebanare.yml`) or names an unknown preset, and a banner lists the errors;
- the page lacks the `diff-table` class, so its diff UI is unknown, and a banner says so;
- a raw source differs from the page by one line, and a banner names the file.

### Synthetic fixtures

The page and the sources are synthetic. `pull-7-files.html` follows the same model of GitHub's classic (server-rendered) diff markup as `test/fixtures/classic-*.html`, and its rows are the `git diff` of the two `store.go` files. That markup is confirmed against a saved "Files changed" page; the page and raw files are the regression fixture and are not regenerated from a live one. The Go sources live under `testdata/` so that the Go tool skips them.
