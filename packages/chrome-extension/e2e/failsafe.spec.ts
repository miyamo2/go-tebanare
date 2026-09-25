// The pages in these tests give the extension a reason to doubt its
// analysis, or nothing to analyze, so it hides nothing (plan 0, 6.2, 6.5,
// and the Phase 3 completion criteria in plan 10).

import type { Page, Worker } from '@playwright/test';
import type { PageStatus } from '../src/shared/messages.js';
import { BASE_SHA, HEAD_SHA, PULL_URL, expect, fixtureCommits, pageStatus, readFixture, serveGitHub, tabIdOf, test } from './harness.js';

const marks = '[data-gotebanare-hidden], [data-gotebanare-fold], [data-gotebanare-badge]';

/**
 * expectNothingHidden waits until the status of the pull request tab
 * matches want with no lines hidden, then checks that the page has no row
 * marks, folds, or badges.
 */
async function expectNothingHidden(page: Page, worker: Worker, want: Partial<PageStatus>): Promise<void> {
  // Some states change nothing on the page, and the content script may not
  // answer yet, so this asks until the status matches.
  await expect(async () => {
    expect(await pageStatus(worker, await tabIdOf(worker))).toMatchObject({ ...want, linesHidden: 0 });
  }).toPass();
  await expect(page.locator(marks)).toHaveCount(0);
}

test('a base commit without a config hides nothing, even when the head commit adds one', async ({ context, page, serviceWorker }) => {
  const commits = fixtureCommits();
  // The config moves from the base commit to the head commit.
  commits[HEAD_SHA] = { ...commits[HEAD_SHA], '.gotebanare.yml': readFixture('testdata/base/.gotebanare.yml') };
  delete commits[BASE_SHA]?.['.gotebanare.yml'];
  const requested = await serveGitHub(context, commits);
  await page.goto(PULL_URL);

  await expectNothingHidden(page, serviceWorker, { state: 'no-config', configSource: 'base', rules: 0 });
  await expect(page.locator('[data-gotebanare-banner]')).toHaveCount(0);
  expect(requested).toContain(`/acme/widgets/raw/${BASE_SHA}/.gotebanare.yaml`);
  expect(requested).not.toContain(`/acme/widgets/raw/${HEAD_SHA}/.gotebanare.yml`);
});

test('a config with a YAML syntax error hides nothing and shows a banner', async ({ context, page, serviceWorker }) => {
  const commits = fixtureCommits();
  commits[BASE_SHA] = { ...commits[BASE_SHA], '.gotebanare.yml': readFixture('broken.gotebanare.yml') };
  await serveGitHub(context, commits);
  await page.goto(PULL_URL);

  await expect(page.locator('[data-gotebanare-banner] li[data-level="error"]')).toHaveText([
    '.gotebanare.yml has errors, so nothing is hidden.',
    /^line 6: the config has a YAML syntax error/,
  ]);
  await expectNothingHidden(page, serviceWorker, { state: 'config-error', configPath: '.gotebanare.yml' });
});

test('a config that names an unknown preset hides nothing and shows a banner', async ({ context, page, serviceWorker }) => {
  const commits = fixtureCommits();
  commits[BASE_SHA] = { ...commits[BASE_SHA], '.gotebanare.yml': 'version: 1\npresets:\n  - getter\n  - nosuchpreset\n' };
  await serveGitHub(context, commits);
  await page.goto(PULL_URL);

  await expect(page.locator('[data-gotebanare-banner] li[data-level="error"]')).toHaveText([
    '.gotebanare.yml has errors, so nothing is hidden.',
    /^line 4, .*unknown preset "nosuchpreset"/,
  ]);
  await expectNothingHidden(page, serviceWorker, { state: 'config-error', configPath: '.gotebanare.yml' });
});

test('a page without a known diff layout hides nothing and shows a banner', async ({ context, page, serviceWorker }) => {
  // Without the diff-table class, the page looks like a diff UI the extension does not know.
  const fixture = readFixture('pull-7-files.html');
  const unknownUi = fixture.replace('<table class="diff-table js-diff-table ', '<table class="');
  expect(unknownUi).not.toBe(fixture);
  await serveGitHub(context, fixtureCommits(), unknownUi);
  await page.goto(PULL_URL);

  await expect(page.locator('[data-gotebanare-banner]')).toContainText('This GitHub diff layout is not supported yet, so nothing is hidden.');
  await expectNothingHidden(page, serviceWorker, { state: 'unsupported-ui', configPath: '.gotebanare.yml' });
});

test('a raw source that differs from the page hides nothing in that file', async ({ context, page, serviceWorker }) => {
  const commits = fixtureCommits();
  const head = commits[HEAD_SHA]?.['store/store.go'] ?? '';
  // Still a getter, so the analysis hides the line, but its text is not the one on the page.
  const changed = head.replace('\treturn s.owner\n', '\treturn (s.owner)\n');
  expect(changed).not.toBe(head);
  commits[HEAD_SHA] = { 'store/store.go': changed };
  await serveGitHub(context, commits);
  await page.goto(PULL_URL);

  await expect(page.locator('[data-gotebanare-banner]')).toContainText(
    'Nothing is hidden in store/store.go because the page does not match the analyzed source.',
  );
  await expectNothingHidden(page, serviceWorker, { state: 'ready', filesWithFolds: 0 });
});
