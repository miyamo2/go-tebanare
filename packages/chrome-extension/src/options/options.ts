// The options page (plan 6.1): repositories where the extension does
// nothing, and debug mode. Both live in chrome.storage.sync.

import { localizePage, t } from '../shared/i18n.js';
import { errorText } from '../shared/messages.js';
import { loadOptions, saveOptions, type OptionsStorage } from '../shared/settings.js';
import { fillForm, optionsElements, renderStatus } from './form.js';
import { checkPatterns } from './patterns.js';

export interface OptionsPage {
  /**
   * save stores the patterns and the debug flag. When a line is not a
   * pattern, it saves nothing and lists the lines to fix, which stay in the
   * textarea. It does nothing when loading the stored options failed.
   */
  save(): Promise<void>;
}

/** startOptions localizes doc (options.html), fills the form from storage, and saves on submit. */
export async function startOptions(doc: Document, storage: OptionsStorage): Promise<OptionsPage> {
  doc.documentElement.lang = t('pageLang');
  localizePage(doc);
  const el = optionsElements(doc);
  let loaded = false;
  try {
    fillForm(el, await loadOptions(storage));
    loaded = true;
    renderStatus(el, { kind: 'idle' });
  } catch (e) {
    renderStatus(el, { kind: 'load-failed', error: errorText(e) });
  }

  async function save(): Promise<void> {
    if (!loaded) return;
    const { patterns, invalid } = checkPatterns(el.repos.value);
    // Saving only the valid lines would drop an exclusion the reader
    // mistyped, and the extension would start hiding code in that repository.
    if (invalid.length > 0) {
      renderStatus(el, { kind: 'invalid', lines: invalid });
      return;
    }
    renderStatus(el, { kind: 'saving' });
    try {
      await saveOptions({ excludedRepos: patterns, debug: el.debug.checked }, storage);
      renderStatus(el, { kind: 'saved' });
    } catch (e) {
      renderStatus(el, { kind: 'save-failed', error: errorText(e) });
    }
  }

  el.form.addEventListener('submit', (e) => {
    e.preventDefault();
    void save();
  });
  // A status from an earlier save no longer describes an edited form.
  const clear = () => {
    if (loaded && !el.save.disabled) renderStatus(el, { kind: 'idle' });
  };
  el.repos.addEventListener('input', clear);
  el.debug.addEventListener('change', clear);
  return { save };
}

// Chrome runs this file as the script of options.html. Tests import it without chrome.
if (typeof chrome !== 'undefined' && chrome.storage?.sync && typeof document !== 'undefined') {
  startOptions(document, chrome.storage.sync).catch((e: unknown) => console.error(`go-tebanare: ${errorText(e)}`));
}
