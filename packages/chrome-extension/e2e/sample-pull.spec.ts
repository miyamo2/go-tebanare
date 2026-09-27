// Runs the built extension on a real pull request: the "Files changed" tab
// of miyamo2/go-tebanare-sample#1, a pull request that stays open as the
// fixture for these tests. Its base commit enables the getter, noop, and
// iferr presets. Three of its files pair a match with similar code that
// stays visible, and handler/task_handler.go has only code that stays
// visible. The expected rows come from the pull request's two commits, so a
// failure points to a change in the extension, in GitHub's markup, or in the
// sample pull request. The tests open the page signed in, where GitHub
// shows the React view, and signed out, where it shows the classic table
// and the extension reads the commits from the REST API; both must fold the
// same rows.

import { createHash } from 'node:crypto';
import type { Locator, Page, Worker } from '@playwright/test';
import { expect, pageStatus, tabIdOf, test } from './harness.js';

const REPO = 'miyamo2/go-tebanare-sample';
const PULL_NUMBER = 1;
const PULL_URL = `https://github.com/${REPO}/pull/${PULL_NUMBER}/changes`;

/** How to find a file and its rows in one of GitHub's diff views. */
interface DiffView {
  /** file returns the container of path. */
  file(page: Page, path: string): Locator;
  /** newRow returns the row of file with the given new-side line number. */
  newRow(file: Locator, line: number): Locator;
}

/**
 * reactView reads the React view that signed-in readers get. A file's
 * region has the id "diff-" followed by the hex SHA-256 of its path.
 */
const reactView: DiffView = {
  file: (page, path) => page.locator(`[role="region"][id="diff-${createHash('sha256').update(path).digest('hex')}"]`),
  newRow: (file, line) => file.locator(`tr.diff-line-row:has(> td:nth-child(2)[data-line-number="${line}"])`),
};

/** classicView reads the server-rendered table that signed-out visitors get. */
const classicView: DiffView = {
  file: (page, path) => page.locator(`.file:has(.file-header[data-path="${path}"])`),
  newRow: (file, line) => file.locator(`table.diff-table tr:has(> td.blob-num:nth-child(2)[data-line-number="${line}"])`),
};

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

/**
 * expectFolds checks the page status, the fold rows, and the hidden and
 * visible rows of every file in FILES, then opens and closes the fold of
 * the Priority getter.
 */
async function expectFolds(page: Page, serviceWorker: Worker, view: DiffView): Promise<void> {
  // The page status explains a failure better than a missing fold row.
  await expect(async () => {
    const status = await pageStatus(serviceWorker, await tabIdOf(serviceWorker, REPO, PULL_NUMBER));
    expect(status, JSON.stringify(status)).toMatchObject({ state: 'ready', enabled: true, filesWithFolds: 3, linesHidden: HIDDEN_LINES });
  }).toPass({ timeout: 60_000 });
  await expect(page.locator('[data-gotebanare-banner]')).toHaveCount(0);

  for (const f of FILES) {
    const file = view.file(page, f.path);
    await expect(file, f.path).toHaveCount(1);
    await expect(file.locator('tr[data-gotebanare-fold]'), f.path).toHaveCount(f.folds);
    await expect(file.locator('[data-gotebanare-hidden]'), f.path).toHaveCount(f.hidden.length);
    for (const n of f.hidden) {
      await expect(view.newRow(file, n), `${f.path}:${n}`).toHaveAttribute('data-gotebanare-hidden', '');
      await expect(view.newRow(file, n), `${f.path}:${n}`).toBeHidden();
    }
    for (const n of f.visible) {
      await expect(view.newRow(file, n), `${f.path}:${n}`).toBeVisible();
    }
  }

  const task = view.file(page, 'domain/task.go');
  await expect(task.locator('tr[data-gotebanare-fold]').last()).toHaveText(/^2 lines hidden by gotebanare: getter/);
  await expect(view.newRow(task, 77)).toHaveText(/func \(t \*Task\) Priority\(\) Priority \{ return t\.priority \}/);
  await expect(view.newRow(task, 81)).toHaveText(/func \(t \*Task\) SetPriority\(p Priority\)/);
  const repo = view.file(page, 'repository/memory_task_repository.go');
  await expect(repo.locator('tr[data-gotebanare-fold]')).toHaveText(/^5 lines hidden by gotebanare: noop/);
  await expect(view.newRow(repo, 62)).toHaveText(/func \(r \*InMemoryTaskRepository\) Warmup\(\) \{\}/);
  const usecase = view.file(page, 'usecase/task_usecase.go');
  await expect(usecase.locator('tr[data-gotebanare-fold]')).toHaveText(/^3 lines hidden by gotebanare: iferr/);
  await expect(view.newRow(view.file(page, 'handler/task_handler.go'), 48)).toHaveText(/http\.Error\(w, err\.Error\(\), http\.StatusNotFound\)/);

  // Clicking the fold row's buttons shows the Priority getter, then hides it again.
  const priority = task.locator('tr[data-gotebanare-fold]').last();
  await priority.getByRole('button', { name: 'Show 2 lines hidden by gotebanare' }).click();
  await expect(view.newRow(task, 77)).toBeVisible();
  await priority.getByRole('button', { name: 'Hide 2 lines again' }).click();
  await expect(view.newRow(task, 77)).toBeHidden();
}

test.describe('signed in', () => {
  test('folds the getter, noop, and iferr matches in the React view and keeps similar code visible', async ({ page, serviceWorker }) => {
    await page.goto(PULL_URL);
    await expect(page.locator('meta[name="user-login"]')).not.toHaveAttribute('content', '');
    await expectFolds(page, serviceWorker, reactView);
  });
});

test.describe('signed out', () => {
  test.use({ signedIn: false });

  // GitHub redirects a signed-out visitor from /changes to /files and
  // renders the classic table without the commits, so the extension looks
  // them up through the REST API.
  test('folds the same matches in the classic view', async ({ page, serviceWorker }) => {
    await page.goto(PULL_URL);
    await expect(page.locator('body')).toHaveClass(/\blogged-out\b/);
    await expectFolds(page, serviceWorker, classicView);
  });
});
