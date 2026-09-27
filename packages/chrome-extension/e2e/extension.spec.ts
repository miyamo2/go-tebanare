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

test('folds the getter and the if err block, and the fold rows and the file badge toggle them', async ({ context, page }) => {
  const requested = await serveGitHub(context, fixtureCommits());
  await page.goto(PULL_URL);

  const folds = page.locator('tr[data-gotebanare-fold]');
  await expect(folds).toHaveCount(2);
  const [getter, iferr] = [folds.nth(0), folds.nth(1)];
  // Fold rows summarize the rule on one line; the button's tooltip names it too.
  await expect(getter).toHaveText(/^4 lines hidden by gotebanare: getter/);
  await expect(iferr).toHaveText(/^3 lines hidden by gotebanare: iferr/);
  const showGetter = getter.getByRole('button', { name: 'Show 4 lines hidden by gotebanare' });
  await expect(showGetter).toHaveAttribute('title', /\ngetter/);
  await expect(iferr.getByRole('button', { name: 'Show 3 lines hidden by gotebanare' })).toHaveAttribute('title', /\niferr/);
  for (const row of newRows(page, [...GETTER, ...IFERR])) {
    await expect(row).toHaveAttribute('data-gotebanare-hidden', '');
    await expect(row).toBeHidden();
  }
  // The fold row stands right above the first hidden row.
  expect(await getter.evaluate((tr) => tr.nextElementSibling?.textContent)).toBe('// Owner returns the store owner.');

  for (const row of newRows(page, VISIBLE)) {
    await expect(row).toBeVisible();
    await expect(row).not.toHaveAttribute('data-gotebanare-hidden');
  }
  const badge = page.locator('[data-gotebanare-badge]');
  await expect(badge).toHaveAttribute('aria-label', 'Show the 7 lines that gotebanare hid in this file');
  await expect(page.locator('[data-gotebanare-banner]')).toHaveCount(0);

  // The fold row shows the getter, and the same row hides it again.
  await showGetter.click();
  for (const row of newRows(page, GETTER)) await expect(row).toBeVisible();
  await expect(badge).toHaveAttribute('aria-label', 'Show the 3 lines that gotebanare hid in this file');
  await getter.getByRole('button', { name: 'Hide 4 lines again' }).click();
  for (const row of newRows(page, GETTER)) await expect(row).toBeHidden();

  // The file badge shows every fold, then hides them all again.
  await badge.click();
  for (const row of newRows(page, [...GETTER, ...IFERR])) await expect(row).toBeVisible();
  await expect(page.locator('[data-gotebanare-hidden]')).toHaveCount(0);
  await expect(page.locator('tr[data-gotebanare-fold][data-gotebanare-open]')).toHaveCount(2);
  await expect(badge).toHaveAttribute('aria-label', 'Hide the 7 lines in this file again');
  await badge.click();
  await expect(page.locator('[data-gotebanare-hidden]')).toHaveCount(GETTER.length + IFERR.length);

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
