# @go-tebanare/chrome-extension

The Chrome extension (Manifest V3) that folds reviewer-agreed Go code in the "Files changed" tab of GitHub pull requests. The content script reads the diff rows, the service worker runs the wasm engine from `@go-tebanare/engine`, and a row is hidden only when its text matches the source the engine analyzed.

## Build

1. Run `make wasm` at the repository root. It writes `packages/engine/wasm/engine.wasm`.
2. Run `bun install` and then `bun run --filter @go-tebanare/chrome-extension build` at the repository root. It writes the unpacked extension to `dist/`, with a copy of `engine.wasm`. The `zip` script also writes `dist.zip`.

To try it, turn on Developer mode in `chrome://extensions` and load `dist/` with "Load unpacked".

## Release

1. Set `version` in `package.json` to the new version, for example `0.2.0`. The Chrome Web Store needs a plain version, higher than the last upload.
2. Push the tag `v0.2.0` from the commit on `main`.

`.github/workflows/release.yml` checks that the tag matches `package.json`, runs the whole of CI, and publishes the tested `dist.zip` as `go-tebanare-chrome-extension-v0.2.0.zip`, with `SHA256SUMS`, on the GitHub release `v0.2.0`. Upload that zip in the Chrome Web Store developer dashboard.

The workflow also holds a `publish` job that uploads the zip with the Chrome Web Store API and submits it for review. It stays commented out until the first release creates the store item; the comment above it lists the settings it needs.

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

- the getter's rows and the 3-line `if err != nil` block are hidden under fold rows whose tooltips name the rules, and the other new code stays visible;
- a fold row shows its rows and hides them again, and the button in the file header shows every fold and hides them all again;
- turning hiding off with the popup's `set-tab-state` request shows every row, and the `toggle-hiding` command hides them again.

The popup request is sent from `popup.html` opened in a tab. The command is fired in the service worker with `chrome.commands.onCommand.dispatch`, which Chromium exposes but the typed API leaves out.

`failsafe.spec.ts` checks that the extension hides nothing when:

- the base commit has no config, even though the head commit adds one;
- the config has a YAML syntax error (`broken.gotebanare.yml`) or names an unknown preset, and a banner lists the errors;
- the page lacks the `diff-table` class, so its diff UI is unknown, and a banner says so;
- a raw source differs from the page by one line, and a banner names the file.

### Synthetic fixtures

The page and the sources are synthetic. `pull-7-files.html` follows the same model of GitHub's classic (server-rendered) diff markup as `test/fixtures/classic-*.html`, and its rows are the `git diff` of the two `store.go` files. github.com could not be loaded where they were written, so no selector has been checked against a real page. Spike S2 must replace the page with a saved "Files changed" page and the raw files with the matching sources, then update `src/content/dom/classic.ts` where the markup differs. The Go sources live under `testdata/` so that the Go tool skips them.
