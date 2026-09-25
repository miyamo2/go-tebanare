import { describe, expect, it } from 'vitest';
import { Lru, SerialQueue, sha256Hex, toBytes } from '../src/util.js';

describe('sha256Hex', () => {
  it('matches known digests', async () => {
    expect(await sha256Hex(new Uint8Array(0))).toBe(
      'e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855',
    );
    expect(await sha256Hex(toBytes('abc'))).toBe(
      'ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad',
    );
  });
});

describe('toBytes', () => {
  it('encodes strings as UTF-8 and treats null as empty', () => {
    expect(Array.from(toBytes('é'))).toEqual([0xc3, 0xa9]);
    expect(toBytes(null).length).toBe(0);
    expect(toBytes(undefined).length).toBe(0);
    const b = new Uint8Array([1, 2]);
    expect(toBytes(b)).toBe(b);
  });
});

describe('SerialQueue', () => {
  it('runs tasks one at a time in submission order', async () => {
    const q = new SerialQueue();
    const log: string[] = [];
    const slow = q.run(async () => {
      log.push('a start');
      await new Promise((r) => setTimeout(r, 20));
      log.push('a end');
      return 'a';
    });
    const fast = q.run(() => {
      log.push('b');
      return 'b';
    });
    expect(await Promise.all([slow, fast])).toEqual(['a', 'b']);
    expect(log).toEqual(['a start', 'a end', 'b']);
  });

  it('keeps running after a task fails', async () => {
    const q = new SerialQueue();
    const failed = q.run(() => {
      throw new Error('boom');
    });
    await expect(failed).rejects.toThrow('boom');
    await expect(q.run(() => 1)).resolves.toBe(1);
  });
});

describe('Lru', () => {
  it('evicts the least recently used entry', () => {
    const evicted: string[] = [];
    const lru = new Lru<string, number>(2, (k) => evicted.push(k));
    lru.set('a', 1);
    lru.set('b', 2);
    expect(lru.get('a')).toBe(1);
    lru.set('c', 3);
    expect(evicted).toEqual(['b']);
    expect(lru.get('b')).toBeUndefined();
    expect(lru.size).toBe(2);
  });
});
