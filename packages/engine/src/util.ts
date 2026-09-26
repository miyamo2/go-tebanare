const encoder = new TextEncoder();

/** Encodes text as UTF-8. null and undefined give an empty array. */
export function toBytes(v: string | Uint8Array | null | undefined): Uint8Array {
  if (v == null) return new Uint8Array(0);
  return typeof v === 'string' ? encoder.encode(v) : v;
}

/** Returns the lowercase hex SHA-256 digest of data. */
export async function sha256Hex(data: Uint8Array): Promise<string> {
  const digest = await globalThis.crypto.subtle.digest('SHA-256', data as Uint8Array<ArrayBuffer>);
  let hex = '';
  for (const b of new Uint8Array(digest)) hex += b.toString(16).padStart(2, '0');
  return hex;
}

/** Runs tasks one at a time in submission order. */
export class SerialQueue {
  #tail: Promise<unknown> = Promise.resolve();

  run<T>(task: () => T | Promise<T>): Promise<T> {
    const p = this.#tail.then(task);
    this.#tail = p.catch(() => undefined);
    return p;
  }
}

/** A small least-recently-used map. */
export class Lru<K, V> {
  readonly #max: number;
  readonly #map = new Map<K, V>();
  readonly #onEvict: ((key: K, value: V) => void) | undefined;

  constructor(max: number, onEvict?: (key: K, value: V) => void) {
    this.#max = max;
    this.#onEvict = onEvict;
  }

  get(key: K): V | undefined {
    const v = this.#map.get(key);
    if (v !== undefined) {
      this.#map.delete(key);
      this.#map.set(key, v);
    }
    return v;
  }

  set(key: K, value: V): void {
    this.#map.delete(key);
    this.#map.set(key, value);
    while (this.#map.size > this.#max) {
      const oldest = this.#map.keys().next().value as K;
      const v = this.#map.get(oldest) as V;
      this.#map.delete(oldest);
      this.#onEvict?.(oldest, v);
    }
  }

  get size(): number {
    return this.#map.size;
  }
}
