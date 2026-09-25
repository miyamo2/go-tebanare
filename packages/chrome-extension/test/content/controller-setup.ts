// Creates a controller for the fake pull request page of controller-fakes.ts.

import { Controller, type ControllerDeps } from '../../src/content/controller.js';
import { defaultContextProvider } from '../../src/content/context.js';
import { detectVariant } from '../../src/content/dom/variant.js';
import type { PullPage } from '../../src/content/page.js';
import { sanitizeOptions, type Options } from '../../src/shared/settings.js';
import { BASE, CONFIG, FakeBackground, FakeFetcher, HEAD, ManualFrames, PAGE } from './controller-fakes.js';

export * from './controller-fakes.js';

export interface Harness {
  controller: Controller;
  bg: FakeBackground;
  fetcher: FakeFetcher;
  frames: ManualFrames;
  /** settle runs timers, promises, mutation callbacks, and frames until the page is quiet. */
  settle(): Promise<void>;
}

export interface HarnessOptions {
  /** Defaults to PAGE. */
  page?: PullPage;
  options?: Partial<Options>;
  bg?: FakeBackground;
  deps?: Partial<ControllerDeps>;
}

/** harness creates a controller for PAGE with fakes, and a fetcher that serves CONFIG at both commits. */
export function harness(opts: HarnessOptions = {}): Harness {
  const bg = opts.bg ?? new FakeBackground();
  const fetcher = new FakeFetcher();
  fetcher.set(BASE, '.gotebanare.yml', CONFIG);
  fetcher.set(HEAD, '.gotebanare.yml', CONFIG);
  const frames = new ManualFrames();
  const controller = new Controller(opts.page ?? PAGE, {
    doc: document,
    send: bg.send,
    fetcher,
    contexts: defaultContextProvider,
    detectVariant: (doc) => detectVariant(doc),
    loadOptions: async () => sanitizeOptions(opts.options ?? {}),
    frames,
    MutationObserver,
    ...opts.deps,
  });
  const settle = async () => {
    for (let i = 0; i < 12; i++) {
      await new Promise((resolve) => setTimeout(resolve, 0));
      frames.flush();
    }
  };
  return { controller, bg, fetcher, frames, settle };
}
