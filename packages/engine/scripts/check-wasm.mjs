// Checks a built engine.wasm: it must import nothing, export every function
// the TypeScript side calls, and stay within the size budget (plan 9).
// Usage: node check-wasm.mjs <engine.wasm> [maxGzipBytes]
import { readFileSync } from 'node:fs';
import { gzipSync } from 'node:zlib';

const [path, maxArg] = process.argv.slice(2);
if (!path) {
  console.error('usage: node check-wasm.mjs <engine.wasm> [maxGzipBytes]');
  process.exit(2);
}
const maxGzip = Number(maxArg ?? 512000);
const bytes = readFileSync(path);
const gzip = gzipSync(bytes, { level: 9 }).length;
const module = await WebAssembly.compile(bytes);
const imports = WebAssembly.Module.imports(module);
const exports = new Set(WebAssembly.Module.exports(module).map((e) => e.name));
const required = [
  'memory', '_initialize', 'alloc', 'free', 'result_len', 'info',
  'compile', 'analyze_change', 'release', 'presets', 'yaml_progress_ptr',
];
const problems = [];
if (imports.length > 0) {
  problems.push(`imports must be empty, found: ${imports.map((i) => `${i.module}.${i.name}`).join(', ')}`);
}
for (const name of required) {
  if (!exports.has(name)) problems.push(`missing export: ${name}`);
}
if (gzip > maxGzip) problems.push(`gzip size ${gzip} bytes is over the budget of ${maxGzip} bytes`);
console.log(`${path}: ${bytes.length} bytes, ${gzip} bytes gzip, ${imports.length} imports`);
if (problems.length > 0) {
  for (const p of problems) console.error(`error: ${p}`);
  process.exit(1);
}
