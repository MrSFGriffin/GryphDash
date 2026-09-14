import test from 'node:test';
import assert from 'node:assert/strict';
import {readFile} from 'node:fs/promises';
import jsonata from 'jsonata';

function plain(value) {
  if (Array.isArray(value)) return value.map(plain);
  if (value && typeof value === 'object') return Object.fromEntries(Object.entries(value).map(([key, item]) => [key, plain(item)]));
  return value;
}

test('JSONata compatibility fixtures evaluate with pinned 2.0.6', async () => {
  const fixtures = JSON.parse(await readFile(new URL('../../testdata/jsonata/expressions.json', import.meta.url)));
  for (const fixture of fixtures) {
    const value = await jsonata(fixture.expression).evaluate(fixture.input);
    assert.deepEqual(plain(value ?? null), fixture.expected, fixture.name);
  }
});
