// The file badge sits in the file header and counts the folds that are
// closed: "3 folds / 57 lines hidden [Show all]" (plan 6.7).

import { countFolds, countLines, t } from '../../shared/i18n.js';

export const BADGE_ATTR = 'data-gotebanare-badge';

/** createBadge builds a detached badge that calls onShowAll when the reader opens every fold. */
export function createBadge(doc: Document, folds: number, lines: number, onShowAll: () => void): HTMLElement {
  const badge = doc.createElement('span');
  badge.className = 'gotebanare-badge';
  badge.setAttribute(BADGE_ATTR, '');
  const text = doc.createElement('span');
  text.className = 'gotebanare-badge-text';
  text.textContent = t('badgeText', countFolds(folds), countLines(lines));
  const show = doc.createElement('button');
  show.type = 'button';
  show.className = 'gotebanare-badge-show-all';
  show.textContent = t('badgeShowAll');
  show.addEventListener('click', (e) => {
    // S2: keep the click from reaching header handlers that collapse the file.
    e.stopPropagation();
    onShowAll();
  });
  badge.append(text, show);
  return badge;
}
