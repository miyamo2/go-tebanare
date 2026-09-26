// @vitest-environment happy-dom
import { describe, expect, it } from 'vitest';
import { FileScanner } from '../../src/content/controller/scan.js';
import { Report } from '../../src/content/controller/status.js';
import { classicVariant } from '../../src/content/dom/classic.js';
import { Semaphore } from '../../src/content/fetcher.js';
import { BASE, CONFIG, FakeBackground, FakeFetcher, HEAD, PAGE, addedMarks, buildPage, fileHtml, hiddenRows, hide } from './controller-fakes.js';

/** scanner creates a FileScanner over the page with fakes; enabled() reads the returned state. */
function scanner() {
  const bg = new FakeBackground();
  bg.ranges.set('store/store.go', { old: [], new: [hide(26, 29)] });
  bg.ranges.set('store/mock_store.go', { old: [], new: [hide(12, 15)] });
  const fetcher = new FakeFetcher();
  const report = new Report();
  const state = { enabled: true, changed: 0 };
  const s = new FileScanner({
    doc: document,
    send: bg.send,
    fetcher,
    limit: new Semaphore(4),
    variant: classicVariant,
    config: { ctx: { ...PAGE, baseSha: BASE, headSha: HEAD }, yaml: CONFIG, configKey: 'k', engineVersion: 'v', rules: [] },
    source: 'base',
    report,
    debug: false,
    enabled: () => state.enabled,
    changed: () => state.changed++,
  });
  return { s, bg, fetcher, report, state };
}

async function settle(): Promise<void> {
  for (let i = 0; i < 6; i++) await new Promise((resolve) => setTimeout(resolve, 0));
}

describe('FileScanner', () => {
  it('analyzes each file once and skips containers that did not change', async () => {
    const [store, mock] = buildPage(fileHtml('classic-modified.html'), fileHtml('classic-added.html'));
    const { s, bg, report, state } = scanner();
    s.scan();
    await settle();
    expect(hiddenRows(store!)).toEqual(['+26', '+27', '+28', '+29']);
    expect(hiddenRows(mock!)).toEqual(['+12', '+13', '+14', '+15']);
    expect(report.counts()).toEqual({ files: 2, filesWithFolds: 2, linesHidden: 8 });
    expect(state.changed).toBe(2);
    const requests = bg.requests.length;
    s.scan();
    await settle();
    expect(bg.requests.length).toBe(requests);
    expect(state.changed).toBe(2);
  });

  it('clears every file when hiding turns off and shows them again when it turns on', async () => {
    const [store] = buildPage(fileHtml('classic-modified.html'));
    const { s, bg, report, state } = scanner();
    s.scan();
    await settle();
    const requests = bg.requests.length;
    state.enabled = false;
    s.refresh();
    expect(addedMarks()).toBe(0);
    expect(report.counts()).toEqual({ files: 1, filesWithFolds: 0, linesHidden: 0 });
    state.enabled = true;
    s.refresh();
    await settle();
    expect(hiddenRows(store!)).toHaveLength(4);
    expect(bg.requests.length).toBe(requests);
  });

  it('clears a folded file whose rows can no longer be read', async () => {
    const [store] = buildPage(fileHtml('classic-modified.html'));
    const { s, report, state } = scanner();
    s.scan();
    await settle();
    expect(hiddenRows(store!)).toHaveLength(4);
    const changed = state.changed;
    const unknown = document.createElement('tr');
    unknown.innerHTML = '<td>?</td>';
    store!.querySelector('tbody')!.append(unknown);
    s.scan();
    await settle();
    expect(hiddenRows(store!)).toEqual([]);
    expect(addedMarks()).toBe(0);
    expect(report.counts()).toEqual({ files: 0, filesWithFolds: 0, linesHidden: 0 });
    expect(state.changed).toBe(changed + 1);
    unknown.remove();
    s.scan();
    await settle();
    expect(hiddenRows(store!)).toHaveLength(4);
    expect(report.counts()).toEqual({ files: 1, filesWithFolds: 1, linesHidden: 4 });
  });

  it('removes what it added and ignores analyses that finish after stop', async () => {
    const [store] = buildPage(fileHtml('classic-modified.html'), fileHtml('classic-added.html'));
    const { s, bg, fetcher } = scanner();
    fetcher.hold((p) => p === 'store/mock_store.go');
    s.scan();
    await settle();
    expect(hiddenRows(store!)).toHaveLength(4);
    s.stop();
    expect(addedMarks()).toBe(0);
    fetcher.release();
    await settle();
    expect(addedMarks()).toBe(0);
    expect(bg.types()).not.toContain('analyze store/mock_store.go');
  });
});
