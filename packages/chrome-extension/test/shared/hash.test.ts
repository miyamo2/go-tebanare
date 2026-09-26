import { describe, expect, it } from 'vitest';
import { fnv1a32, lineHash, normalizeLine } from '../../src/shared/hash.js';

describe('normalizeLine', () => {
  it.each([
    ['plain', 'plain'],
    ['crlf\r', 'crlf'],
    ['trailing  \t', 'trailing'],
    ['blanks then cr \t\r', 'blanks then cr'],
    ['\tleading kept', '\tleading kept'],
    ['inner  space kept', 'inner  space kept'],
    [' \t\r', ''],
    ['', ''],
  ])('%j => %j', (input, want) => {
    expect(normalizeLine(input)).toBe(want);
  });

  it('handles a long run of blanks', () => {
    expect(normalizeLine(`x${' '.repeat(100_000)}`)).toBe('x');
  });
});

describe('fnv1a32', () => {
  // Values computed independently over UTF-16 code units.
  it.each([
    ['', '811c9dc5'],
    ['a', 'e40c292c'],
    ['foobar', 'bf9cf968'],
    ['é', '6c0b6c44'],
    ['\u{1F648}', '83315360'],
    ['func (m *Mock) Get() int {', '82e5e926'],
  ])('%j => %s', (input, want) => {
    expect(fnv1a32(input)).toBe(want);
  });
});

describe('lineHash', () => {
  it('ignores trailing blanks and carriage returns', () => {
    expect(lineHash('\treturn m.v  \r')).toBe('778cd1f9');
    expect(lineHash('\treturn m.v')).toBe(lineHash('\treturn m.v\t'));
  });

  it('keeps leading indentation significant', () => {
    expect(lineHash('\treturn m.v')).not.toBe(lineHash('return m.v'));
  });
});
