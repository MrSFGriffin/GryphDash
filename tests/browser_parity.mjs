// Optional browser QA for the committed current-widget DOM baseline.
// Run after building with the same Playwright setup used by browser.mjs.
import assert from 'node:assert/strict';
import {readFileSync} from 'node:fs';
import {spawn} from 'node:child_process';
import {once} from 'node:events';
import {fileURLToPath} from 'node:url';
import path from 'node:path';

const {chromium} = await import(process.env.GRYPHDASH_PLAYWRIGHT_MODULE || 'playwright');
const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
const port = process.env.GRYPHDASH_TEST_PORT || '18092';
const origin = `http://127.0.0.1:${port}`;
const dashboard = JSON.parse(readFileSync(path.join(root, 'testdata/widget-parity/current-dashboard.json')));
const want = JSON.parse(readFileSync(path.join(root, 'testdata/widget-parity/current-web-dom.json')));
const fixture = {widgets: dashboard.widgets.map(widget => ({...widget, default: true}))};
const server = spawn(path.join(root, 'bin/gryphdash'), [], {
  cwd: root,
  env: {...process.env, GRYPHDASH_ADDR: `127.0.0.1:${port}`, GRYPHDASH_CODEX_BIN: '/nonexistent/gryphdash-test-codex'},
  stdio: 'ignore'
});

let browser;
try {
  for (let i = 0; i < 60; i++) {
    try { if ((await fetch(origin)).ok) break; } catch {}
    if (server.exitCode !== null) throw new Error('Test server stopped');
    await new Promise(resolve => setTimeout(resolve, 100));
  }
  browser = await chromium.launch({headless: true, channel: 'chromium'});
  const context = await browser.newContext({viewport: {width: 1440, height: 1000}});
  await context.route('**/api/widgets', route => route.fulfill({json: fixture}));
  const page = await context.newPage();
  await page.goto(origin);
  await page.waitForFunction(() => document.querySelectorAll('.grid-stack-item').length === 49);
  const got = await page.locator('.grid-stack-item-content').evaluateAll(cards => cards.map(card => ({
    id: card.closest('.grid-stack-item').gridstackNode.id,
    ariaLabel: card.getAttribute('aria-label'),
    title: card.querySelector('.widget-title').textContent,
    group: card.querySelector('.widget-group').textContent,
    body: [...card.querySelector('.widget-body').children].map(node => {
      const entry = {
        tag: node.tagName.toLowerCase(),
        class: node.className || undefined,
        text: node.textContent,
        max: node.max || undefined,
        value: node.value || undefined,
        ariaLabel: node.getAttribute('aria-label') || undefined,
        resetsAt: node.dataset.resetsAt || undefined,
        rows: node.tagName === 'TABLE' ? [...node.tBodies[0].rows].map(row => {
          const rowMeter = row.querySelector('meter');
          return {
            cells: [...row.cells].map(cell => cell.textContent),
            meter: rowMeter ? Object.fromEntries(Object.entries({
              value: rowMeter.value || undefined,
              ariaLabel: rowMeter.getAttribute('aria-label')
            }).filter(([, value]) => value !== undefined)) : undefined
          };
        }) : undefined,
        resets: node.classList.contains('reset-detail') ? {
          title: node.querySelector('h3').textContent,
          description: node.querySelector('.note').textContent,
          details: [...node.querySelectorAll('dt')].map((term, index) => [term.textContent, node.querySelectorAll('dd')[index].textContent])
        } : undefined
      };
      return Object.fromEntries(Object.entries(entry).filter(([, value]) => value !== undefined));
    }),
    status: card.querySelector('.widget-status').textContent,
    stale: card.querySelector('.widget-status').classList.contains('stale'),
    removeAriaLabel: card.querySelector('.remove-widget').getAttribute('aria-label')
  })).sort((a, b) => a.id.localeCompare(b.id)));
  assert.deepEqual(got, want.cards);
  await context.close();
  console.log('Browser widget DOM parity passed.');
} finally {
  await browser?.close();
  if (server.exitCode === null) { const stopped = once(server, 'exit'); server.kill('SIGTERM'); await stopped; }
}
