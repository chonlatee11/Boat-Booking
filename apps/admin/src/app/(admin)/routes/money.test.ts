import { test } from 'node:test';
import assert from 'node:assert/strict';

// node --test (Node 24, native TS type-stripping) requires an explicit file
// extension on relative specifiers and resolves `.ts` literally; tsc's
// "bundler" moduleResolution instead rejects a literal `.ts` import
// specifier (TS5097) unless allowImportingTsExtensions is set, which this
// project's tsconfig does not enable. A dynamic import keeps both tools
// happy: node resolves it at runtime, tsc only checks the awaited shape.
const moneyPath = './money.ts';
const { bahtToSatang, formatSatang }: typeof import('./money') = await import(
  moneyPath
);

test('bahtToSatang converts plain baht to satang', () => {
  assert.equal(bahtToSatang('150.50'), '15050');
  assert.equal(bahtToSatang('0'), '0');
  assert.equal(bahtToSatang('1'), '100');
});

test('bahtToSatang pads single fraction digit', () => {
  assert.equal(bahtToSatang('1.5'), '150');
});

test('bahtToSatang rejects malformed input', () => {
  assert.equal(bahtToSatang('abc'), null);
  assert.equal(bahtToSatang('-1'), null);
  assert.equal(bahtToSatang('1.555'), null);
  assert.equal(bahtToSatang(''), null);
});

test('bahtToSatang rejects amount exceeding the 10,000,000 satang bound', () => {
  assert.equal(bahtToSatang('100000.01'), null);
  assert.equal(bahtToSatang('100000.00'), '10000000');
});

test('formatSatang groups thousands and formats fraction', () => {
  assert.equal(formatSatang('15050'), '฿150.50');
  assert.equal(formatSatang('100000000'), '฿1,000,000.00');
  assert.equal(formatSatang('5'), '฿0.05');
});
