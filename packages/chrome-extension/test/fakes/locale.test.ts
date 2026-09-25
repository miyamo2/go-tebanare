import { describe, expect, it } from 'vitest';
import { formatMessage, localeMessages } from './locale.js';

describe('formatMessage', () => {
  it('formats placeholders, positional substitutions, and $$', () => {
    const entry = { message: '$A$ and $b$ cost $$1 ($2)', placeholders: { a: { content: 'x$1y' }, b: { content: '$2' } } };
    expect(formatMessage(entry, ['1', '2'])).toBe('x1y and 2 cost $1 (2)');
  });
});

describe('localeMessages', () => {
  it('keys entries by lowercase name', () => {
    expect(localeMessages('en')['countlines']?.message).toBe('$count$ lines');
    expect(localeMessages('en')['countLines']).toBeUndefined();
  });
});
