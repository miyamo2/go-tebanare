// @vitest-environment happy-dom
import { describe, expect, it } from 'vitest';
import { loadStaticPage } from './static-page.js';

describe('loadStaticPage', () => {
  it('loads the body and its class without the scripts', () => {
    document.documentElement.lang = 'xx';
    document.body.innerHTML = '<p id="old"></p>';
    loadStaticPage('options.html');
    expect(document.body.className).toBe('options');
    expect(document.getElementById('options-form')).not.toBeNull();
    expect(document.getElementById('old')).toBeNull();
    expect(document.querySelector('script')).toBeNull();
    expect(document.documentElement.lang).toBe('');
  });

  it('throws for a page that does not exist', () => {
    expect(() => loadStaticPage('missing.html')).toThrow(/ENOENT/);
  });
});
