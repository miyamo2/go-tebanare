// Runs the built extension on the synthetic "Files changed" page of
// acme/widgets#7 (plan 6.2, 6.7) with the config at the base commit:
// presets getter and iferr.

import {
  BASE_SHA,
  HEAD_SHA,
  PULL_URL,
  expect,
  fixtureCommits,
  newRows,
  pageStatus,
  runCommand,
  sendFromPopup,
  serveGitHub,
  tabIdOf,
  test,
} from './harness.js';

// New line numbers in fixtures/testdata/head/store/store.go.
const GETTER = [19, 20, 21, 22]; // Owner and its doc comment
const IFERR = [27, 28, 29]; // if err != nil { return err }
const VISIBLE = [15, 26, 43, 44, 45, 46, 47, 48, 49]; // the owner field, err := validate(k), and func validate

test('folds the getter and the if err block, and Show opens them', async ({ context, page }) => {
  const requested = await serveGitHub(context, fixtureCommits());
  await page.goto(PULL_URL);

  const bar = page.locator('tr[data-gotebanare-fold]:not([data-gotebanare-thin])');
  await expect(bar).toHaveCount(1);
  await expect(bar).toContainText('4 lines hidden');
  await expect(bar.locator('.gotebanare-fold-rule')).toHaveText(['getter']);
  for (const row of newRows(page, GETTER)) {
    await expect(row).toHaveAttribute('data-gotebanare-hidden', '');
    await expect(row).toBeHidden();
  }
  // The bar stands right above the first hidden row.
  expect(await bar.evaluate((tr) => tr.nextElementSibling?.textContent)).toBe('// Owner returns the store owner.');

  // A fold of 3 lines or fewer is a thin separator.
  const thin = page.locator('tr[data-gotebanare-thin]');
  await expect(thin).toHaveCount(1);
  await expect(thin.getByRole('button')).toHaveAttribute('title', /iferr/);
  for (const row of newRows(page, IFERR)) await expect(row).toHaveAttribute('data-gotebanare-hidden', '');

  for (const row of newRows(page, VISIBLE)) {
    await expect(row).toBeVisible();
    await expect(row).not.toHaveAttribute('data-gotebanare-hidden');
  }
  await expect(page.locator('[data-gotebanare-badge]')).toContainText('2 folds / 7 lines hidden');
  await expect(page.locator('[data-gotebanare-banner]')).toHaveCount(0);

  await bar.getByRole('button', { name: 'Show' }).click();
  for (const row of newRows(page, GETTER)) await expect(row).toBeVisible();
  await expect(bar).toHaveCount(0);
  await expect(page.locator('[data-gotebanare-badge]')).toContainText('1 fold / 3 lines hidden');

  await thin.getByRole('button').click();
  for (const row of newRows(page, IFERR)) await expect(row).toBeVisible();
  await expect(page.locator('[data-gotebanare-fold], [data-gotebanare-hidden], [data-gotebanare-badge]')).toHaveCount(0);

  expect(requested).toEqual(
    expect.arrayContaining([
      `/acme/widgets/raw/${BASE_SHA}/.gotebanare.yml`,
      `/acme/widgets/raw/${BASE_SHA}/.gotebanare.yaml`,
      `/acme/widgets/raw/${BASE_SHA}/store/store.go`,
      `/acme/widgets/raw/${HEAD_SHA}/store/store.go`,
    ]),
  );
});

test("the popup's set-tab-state request shows every row, and the toggle-hiding command hides them again", async ({ context, page, serviceWorker }) => {
  await serveGitHub(context, fixtureCommits());
  await page.goto(PULL_URL);
  const hidden = page.locator('[data-gotebanare-hidden]');
  await expect(hidden).toHaveCount(GETTER.length + IFERR.length);
  const tabId = await tabIdOf(serviceWorker);

  // The popup's "Hiding on/off" button sends this request.
  expect(await sendFromPopup(context, serviceWorker, { type: 'set-tab-state', tabId, enabled: false })).toEqual({ ok: true });
  await page.bringToFront();
  await expect(page.locator('[data-gotebanare-fold], [data-gotebanare-hidden], [data-gotebanare-badge]')).toHaveCount(0);
  for (const row of newRows(page, [...GETTER, ...IFERR, ...VISIBLE])) await expect(row).toBeVisible();
  expect(await pageStatus(serviceWorker, tabId)).toMatchObject({ state: 'ready', enabled: false, linesHidden: 0 });

  // Alt+Shift+H runs this command.
  await runCommand(serviceWorker, 'toggle-hiding', tabId);
  await expect(hidden).toHaveCount(GETTER.length + IFERR.length);
  await expect(page.locator('tr[data-gotebanare-fold]')).toHaveCount(2);
  expect(await pageStatus(serviceWorker, tabId)).toMatchObject({ enabled: true, filesWithFolds: 1, linesHidden: 7 });
});
