// @vitest-environment happy-dom
import { describe, expect, it } from 'vitest';
import { inactiveStatus, Report } from '../../src/content/controller/status.js';
import { touches, watchMutations, type Changes, type FrameApi } from '../../src/content/controller/watch.js';

/** manualFrames returns frame calls that queue callbacks until flush(). */
function manualFrames() {
  const queue = new Map<number, () => void>();
  let next = 0;
  const api: FrameApi = {
    requestAnimationFrame: (cb) => {
      queue.set(++next, cb);
      return next;
    },
    cancelAnimationFrame: (id) => void queue.delete(id),
  };
  const flush = () => {
    const due = [...queue.values()];
    queue.clear();
    for (const cb of due) cb();
  };
  return { api, queue, flush };
}

describe('Report', () => {
  it('keeps one page notice per kind, before the file notices', () => {
    const r = new Report();
    r.setFile('a', { kind: 'source-mismatch', path: 'a.go' });
    r.addPage({ kind: 'head-preview', path: '.gotebanare.yml' });
    r.addPage({ kind: 'head-preview', path: 'other' });
    r.addPage({ kind: 'unsupported-ui' });
    r.dropPage('unsupported-ui');
    expect(r.messages().map((m) => m.text)).toEqual(['bannerHeadPreview', 'bannerSourceMismatch']);
    r.setFile('a', undefined);
    expect(r.messages()).toHaveLength(1);
  });

  it('counts analyzed files and hidden lines', () => {
    const r = new Report();
    r.tally('a', true, { folds: 2, lines: 7 });
    r.tally('b', true);
    r.tally('c', false);
    expect(r.counts()).toEqual({ files: 2, filesWithFolds: 1, linesHidden: 7 });
    r.hideNothing();
    expect(r.counts()).toEqual({ files: 2, filesWithFolds: 0, linesHidden: 0 });
  });

  it('describes a page without a controller', () => {
    expect(inactiveStatus()).toMatchObject({ state: 'inactive', messages: [], enabled: true, headPreview: false });
  });
});

describe('watchMutations', () => {
  it('batches changes into one frame and ignores its own nodes', async () => {
    document.body.innerHTML = '<table><tbody><tr><td>x</td></tr></tbody></table>';
    const frames = manualFrames();
    const batches: Changes[] = [];
    const stop = watchMutations(document.body, MutationObserver, frames.api, (c) => batches.push(c));
    const tbody = document.querySelector('tbody')!;
    const fold = document.createElement('tr');
    fold.setAttribute('data-gotebanare-fold', 'k');
    tbody.prepend(fold);
    await Promise.resolve();
    expect(frames.queue.size).toBe(0);

    const rows = [document.createElement('tr'), document.createElement('tr')];
    tbody.append(rows[0]!);
    await Promise.resolve();
    tbody.append(rows[1]!);
    await Promise.resolve();
    expect(frames.queue.size).toBe(1);
    frames.flush();
    expect(batches).toEqual([{ targets: [tbody], added: rows }]);

    tbody.append(document.createElement('tr'));
    await Promise.resolve();
    stop();
    expect(frames.queue.size).toBe(0);
    tbody.append(document.createElement('tr'));
    await Promise.resolve();
    frames.flush();
    expect(batches).toHaveLength(1);
  });

  it('reports text that changes in place', async () => {
    document.body.innerHTML = '<p>old</p>';
    const frames = manualFrames();
    const batches: Changes[] = [];
    const stop = watchMutations(document.body, MutationObserver, frames.api, (c) => batches.push(c));
    const text = document.querySelector('p')!.firstChild as Text;
    text.data = 'new';
    await Promise.resolve();
    frames.flush();
    expect(batches).toEqual([{ targets: [text], added: [] }]);
    expect(touches(batches[0]!, document.querySelector('p')!)).toBe(true);
    stop();
  });

  it('tells which elements a batch touched', () => {
    document.body.innerHTML = '<main><div id="a"><p></p></div><div id="b"></div></main>';
    const [main, a, b, p] = ['main', '#a', '#b', 'p'].map((s) => document.querySelector(s)!);
    expect(touches({ targets: [p!], added: [] }, a!)).toBe(true);
    expect(touches({ targets: [p!], added: [] }, b!)).toBe(false);
    // A new child of main touches only itself and what it contains.
    expect(touches({ targets: [main!], added: [a!] }, a!)).toBe(true);
    expect(touches({ targets: [main!], added: [a!] }, p!)).toBe(true);
    expect(touches({ targets: [main!], added: [a!] }, b!)).toBe(false);
  });
});
