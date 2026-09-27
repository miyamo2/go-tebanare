// Runs the built extension on a real pull request: the "Files changed" tab
// of miyamo2/go-tebanare-sample#1, a pull request that stays open as the
// fixture for these tests. Its base commit enables the getter, noop, and
// iferr presets. Three of its files pair a match with similar code that
// stays visible, and handler/task_handler.go has only code that stays
// visible. The expected rows come from the pull request's two commits, so a
// failure points to a change in the extension, in GitHub's markup, or in the
// sample pull request.

import { createHash } from 'node:crypto';
import type { Locator, Page } from '@playwright/test';
import { expect, pageStatus, tabIdOf, test } from './harness.js';

const REPO = 'miyamo2/go-tebanare-sample';
const PULL_NUMBER = 1;
const PULL_URL = `https://github.com/${REPO}/pull/${PULL_NUMBER}/changes`;


/**
 * fileRegion returns the React view's container of path, the region whose
 * id is "diff-" followed by the hex SHA-256 of path.
 */
function fileRegion(page: Page, path: string): Locator {
  return page.locator(`[role="region"][id="diff-${createHash('sha256').update(path).digest('hex')}"]`);
}

/** newRow returns the row of file with the given new-side line number. */
function newRow(file: Locator, line: number): Locator {
  return file.locator(`tr.diff-line-row:has(> td:nth-child(2)[data-line-number="${line}"])`);
}

interface FileExpectation {
  path: string;
  /** New line numbers of the rows the extension hides. */
  hidden: number[];
  /** New line numbers of rows that must stay visible, including the similar code that does not match. */
  visible: number[];
  /** The number of fold rows in the file. */
  folds: number;
}

// Line numbers are those of the head commit 75f1951. A context row is
// hidden only when the engine's ranges hold both of its sides, so a match
// next to unchanged code of the same kind can fold that code too.
const FILES: FileExpectation[] = [
  {
    // Priority() and its doc comment (76-77) match getter, and so does the
    // unchanged DueAt() above them (73-74). SetPriority (79-81) takes a
    // parameter and IsOverdue (83-85) compares values.
    path: 'domain/task.go',
    hidden: [73, 74, 76, 77],
    visible: [75, 78, 79, 80, 81, 83, 84, 85],
    folds: 2,
  },
  {
    // Warmup() and its doc comment (61-62) match noop, with the unchanged
    // Migrate() (58-59) and the blank line between them.
    path: 'repository/memory_task_repository.go',
    hidden: [58, 59, 60, 61, 62],
    visible: [57, 63, 64, 65, 66, 67, 68, 69, 70],
    folds: 1,
  },
  {
    // The guard in DeleteTask matches iferr; the rest of the function stays.
    path: 'usecase/task_usecase.go',
    hidden: [57, 58, 59],
    visible: [54, 55, 56, 60, 61],
    folds: 1,
  },
  {
    // The guard in Delete calls http.Error before returning.
    path: 'handler/task_handler.go',
    hidden: [],
    visible: [43, 44, 45, 46, 47, 48, 49, 50, 51, 52],
    folds: 0,
  },
];

const HIDDEN_LINES = FILES.reduce((n, f) => n + f.hidden.length, 0);

test.beforeEach(async ({ page }) => {
  await page.goto(PULL_URL);
  // A signed-out page would name no commits, and the extension would hide nothing.
  await expect(page.locator('meta[name="user-login"]')).not.toHaveAttribute('content', '');
});

test('folds the getter, noop, and iferr matches of the sample pull request and keeps similar code visible', async ({ page, serviceWorker }) => {
  // The page status explains a failure better than a missing fold row.
  await expect(async () => {
    const status = await pageStatus(serviceWorker, await tabIdOf(serviceWorker, REPO, PULL_NUMBER));
    expect(status, JSON.stringify(status)).toMatchObject({ state: 'ready', enabled: true, filesWithFolds: 3, linesHidden: HIDDEN_LINES });
  }).toPass({ timeout: 60_000 });
  await expect(page.locator('[data-gotebanare-banner]')).toHaveCount(0);

  for (const f of FILES) {
    const file = fileRegion(page, f.path);
    await expect(file, f.path).toHaveCount(1);
    await expect(file.locator('tr[data-gotebanare-fold]'), f.path).toHaveCount(f.folds);
    await expect(file.locator('[data-gotebanare-hidden]'), f.path).toHaveCount(f.hidden.length);
    for (const n of f.hidden) {
      await expect(newRow(file, n), `${f.path}:${n}`).toHaveAttribute('data-gotebanare-hidden', '');
      await expect(newRow(file, n), `${f.path}:${n}`).toBeHidden();
    }
    for (const n of f.visible) {
      await expect(newRow(file, n), `${f.path}:${n}`).toBeVisible();
    }
  }

  const task = fileRegion(page, 'domain/task.go');
  await expect(task.locator('tr[data-gotebanare-fold]').last()).toHaveText(/^2 lines hidden by gotebanare: getter/);
  await expect(newRow(task, 77)).toHaveText(/func \(t \*Task\) Priority\(\) Priority \{ return t\.priority \}/);
  await expect(newRow(task, 81)).toHaveText(/func \(t \*Task\) SetPriority\(p Priority\)/);
  const repo = fileRegion(page, 'repository/memory_task_repository.go');
  await expect(repo.locator('tr[data-gotebanare-fold]')).toHaveText(/^5 lines hidden by gotebanare: noop/);
  await expect(newRow(repo, 62)).toHaveText(/func \(r \*InMemoryTaskRepository\) Warmup\(\) \{\}/);
  const usecase = fileRegion(page, 'usecase/task_usecase.go');
  await expect(usecase.locator('tr[data-gotebanare-fold]')).toHaveText(/^3 lines hidden by gotebanare: iferr/);
  await expect(newRow(fileRegion(page, 'handler/task_handler.go'), 48)).toHaveText(/http\.Error\(w, err\.Error\(\), http\.StatusNotFound\)/);

  // Clicking the fold row's buttons shows the Priority getter, then hides it again.
  const priority = task.locator('tr[data-gotebanare-fold]').last();
  await priority.getByRole('button', { name: 'Show 2 lines hidden by gotebanare' }).click();
  await expect(newRow(task, 77)).toBeVisible();
  await priority.getByRole('button', { name: 'Hide 2 lines again' }).click();
  await expect(newRow(task, 77)).toBeHidden();
});
