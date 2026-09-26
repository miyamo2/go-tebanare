// The file badge is an icon button in the file header, next to GitHub's
// own header buttons (plan 6.7). While any fold of the file is closed it
// shows them all (eye icon); once every fold is open it hides them all
// again (eye-closed icon).

import { countLines, t } from '../../shared/i18n.js';
import { createIcon } from './icons.js';

export const BADGE_ATTR = 'data-gotebanare-badge';

/** What the badge acts on: the lines it shows, or with allOpen, the lines it hides again. */
export interface BadgeState {
  lines: number;
  allOpen: boolean;
}

/** createBadge builds a detached badge that calls onToggle when the reader clicks it. */
export function createBadge(doc: Document, state: BadgeState, onToggle: () => void): HTMLElement {
  const button = doc.createElement('button');
  button.type = 'button';
  button.className = 'gotebanare-badge';
  button.setAttribute(BADGE_ATTR, '');
  const label = t(state.allOpen ? 'fileHideTitle' : 'fileShowTitle', countLines(state.lines));
  button.setAttribute('aria-label', label);
  button.title = label;
  button.append(createIcon(doc, state.allOpen ? 'eye-closed' : 'eye'));
  button.addEventListener('click', (e) => {
    // S2: keep the click from reaching header handlers that collapse the file.
    e.stopPropagation();
    onToggle();
  });
  return button;
}
