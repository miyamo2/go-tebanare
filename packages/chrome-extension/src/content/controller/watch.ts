// Follows DOM changes on the page (plan 6.6): lazy loading, "Load diff",
// context expansion, and re-rendering all show up as child list changes,
// and code that changes in place can also show up as text changes.
// Changes are batched per animation frame.

import { isOwnNode } from '../apply.js';

/** The animation frame calls watchMutations makes. window provides them. */
export interface FrameApi {
  requestAnimationFrame(cb: () => void): number;
  cancelAnimationFrame(id: number): void;
}

/** The MutationObserver constructor, or a fake with the same members. */
export type ObserverCtor = new (cb: MutationCallback) => Pick<MutationObserver, 'observe' | 'disconnect'>;

/** The nodes one batch of mutations touched, leaving out the extension's own nodes. */
export interface Changes {
  /** Nodes whose children or text changed. */
  targets: readonly Node[];
  /** Nodes that were added. */
  added: readonly Node[];
}

/**
 * touches reports whether changes can have changed what el shows: a child
 * list or text changed inside el (or el's own children changed), or el
 * arrived inside an added node.
 */
export function touches(changes: Changes, el: Node): boolean {
  return changes.targets.some((t) => el.contains(t)) || changes.added.some((a) => a.contains(el));
}

/** isOwnRecord reports a record that only adds or removes nodes the extension inserted. */
function isOwnRecord(r: MutationRecord): boolean {
  const nodes = [...r.addedNodes, ...r.removedNodes];
  return nodes.length > 0 && nodes.every(isOwnNode);
}

/**
 * watchMutations calls onBatch in the next animation frame after the child
 * lists or text nodes under root change, once per frame, with the nodes
 * that changed.
 * Records that only add or remove the extension's fold rows, badges, and
 * banner are ignored, so applying a plan does not wake the watcher again.
 * It returns a function that stops watching and cancels a pending frame.
 */
export function watchMutations(root: Node, Observer: ObserverCtor, frames: FrameApi, onBatch: (changes: Changes) => void): () => void {
  let frame: number | null = null;
  let stopped = false;
  const targets = new Set<Node>();
  const added = new Set<Node>();
  const observer = new Observer((records) => {
    if (stopped) return;
    for (const r of records) {
      if (isOwnRecord(r)) continue;
      targets.add(r.target);
      for (const n of r.addedNodes) if (!isOwnNode(n)) added.add(n);
    }
    if (frame !== null || targets.size === 0) return;
    frame = frames.requestAnimationFrame(() => {
      frame = null;
      const changes: Changes = { targets: [...targets], added: [...added] };
      targets.clear();
      added.clear();
      if (!stopped) onBatch(changes);
    });
  });
  observer.observe(root, { childList: true, characterData: true, subtree: true });
  return () => {
    stopped = true;
    observer.disconnect();
    if (frame !== null) frames.cancelAnimationFrame(frame);
    frame = null;
  };
}
