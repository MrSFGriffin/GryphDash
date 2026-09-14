import test from 'node:test';
import assert from 'node:assert/strict';
import {readFile} from 'node:fs/promises';

test('the frontend entrypoint remains an ES module', async () => {
  const source = await readFile(new URL('./main.js', import.meta.url), 'utf8');
  assert.match(source, /import ['"]\.\/dashboard\.js['"]/);
});
