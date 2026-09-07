// Run against a built binary. Playwright is a development-only dependency.
import assert from 'node:assert/strict';
import {spawn} from 'node:child_process';
import {once} from 'node:events';
import {fileURLToPath} from 'node:url';
import path from 'node:path';
const {chromium} = await import(process.env.GRYPHDASH_PLAYWRIGHT_MODULE || 'playwright');
const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
const port = process.env.GRYPHDASH_TEST_PORT || '18091';
const origin = `http://127.0.0.1:${port}`;
const server = spawn(path.join(root, 'bin/gryphdash'), [], {
  cwd: root,
  env: {...process.env, GRYPHDASH_ADDR: `127.0.0.1:${port}`, GRYPHDASH_CODEX_BIN: '/nonexistent/gryphdash-test-codex'},
  stdio: 'ignore'
});
let browser;
const key = 'gryphdash.layout.v1';
const base = (id, title, visible = true) => ({id, title, group: 'Codex · codex', kind: 'metric', value: '42', note: 'Test metric', status: 'Updated 2026-01-01 12:00:00 MST', default: visible, width: 4, height: 4});
let fixture = {widgets: [
  {...base('codex/bucket/codex/primary', '5-hour limit'), value: '75%', percent: 75, resetsAt: Math.floor(Date.now()/1000)+7200},
  {...base('codex/bucket/codex/secondary', 'Weekly limit'), value: '96%', percent: 96},
  {...base('codex/bucket/codex/credits/balance', 'Credits remaining'), value: '0'},
  {...base('codex/usage/daily', 'Daily token activity'), kind: 'daily', width: 8, height: 6, days: [{date: '2026-01-01', tokens: '100', percent: 100}]},
  {...base('codex/resets/details', 'Earned reset details', false), kind: 'resets', width: 8, height: 6, resets: [{title: 'Full reset', description: 'Test reset', id: 'test-reset', status: 'available', type: 'codexRateLimits', granted: '2026-01-01', expires: 'No expiration'}]},
  {...base('codex/account/plan', 'Account plan'), value: 'plus'},
  {...base('codex/account/auth', 'Authentication', false), value: '<img src=x onerror="window.injected=true">'},
  {...base('codex/usage/lifetimeTokens', 'Lifetime tokens', false), value: '123456'}
]};
async function context(options = {}) {
  const ctx = await browser.newContext({viewport: {width: 1440, height: 1000}, ...options});
  await ctx.route('**/api/widgets', route => route.fulfill({json: fixture}));
  return ctx;
}
async function loaded(page) {
  await page.goto(origin);
  await page.waitForFunction(() => !document.getElementById('add-widgets').disabled);
}
async function layout(page) { return page.evaluate(k => JSON.parse(localStorage.getItem(k)).items, key); }
async function pickerAdd(page, name) {
  await page.getByRole('button', {name: 'Add widgets', exact: true}).first().click();
  await page.getByRole('searchbox').fill(name);
  await page.getByRole('button', {name: `Add ${name} (Codex · codex)`, exact: true}).click();
  await page.getByRole('button', {name: 'Done', exact: true}).click();
}
try {
  for (let i=0;i<60;i++) {
    try { if ((await fetch(origin)).ok) break; } catch {}
    if (server.exitCode !== null) throw new Error('Test server stopped');
    await new Promise(r => setTimeout(r, 100));
  }
  browser = await chromium.launch({headless: true, channel: 'chromium'});
  const ctx = await context();
  const page = await ctx.newPage();
  const errors = [];
  page.on('pageerror', e => errors.push(e.message));
  await loaded(page);
  assert.equal(await page.locator('.grid-stack-item').count(), 5);
  assert.equal(await page.locator('.grid-stack-item img').count(), 0);
  assert.equal(await page.locator('meta[http-equiv="refresh"]').count(), 0);
  assert.equal(await page.getByText('OpenRouter').count(), 0);
  await pickerAdd(page, 'Authentication');
  assert.equal(await page.locator('.grid-stack-item').count(), 6);
  assert.equal(await page.locator('.grid-stack-item img').count(), 0);
  assert.equal(await page.evaluate(() => window.injected), undefined);
  await pickerAdd(page, 'Earned reset details');
  assert.equal(await page.getByText('Full reset', {exact:true}).count(), 1);
  await page.getByRole('button', {name:'Edit dashboard', exact:true}).click();
  await page.getByRole('button', {name:'Remove Credits remaining', exact:true}).click();
  assert.equal((await layout(page)).some(w=>w.id.endsWith('/credits/balance')), false);
  const heading = page.locator('.widget-heading').filter({has: page.getByRole('heading', {name:'5-hour limit', exact:true})});
  await heading.focus();
  const beforeKey = (await layout(page)).find(w=>w.id.endsWith('/primary'));
  await heading.press('Shift+ArrowRight');
  assert.equal((await layout(page)).find(w=>w.id.endsWith('/primary')).w, beforeKey.w + 1);
  // Exercise real pointer drag and resize, not just programmatic grid updates.
  await page.waitForTimeout(350);
  const startBox = await heading.boundingBox();
  const beforeDrag = (await layout(page)).find(w=>w.id.endsWith('/primary'));
  await page.mouse.move(startBox.x+30,startBox.y+20);
  await page.mouse.down();
  await page.mouse.move(startBox.x+630,startBox.y+20,{steps:12});
  await page.mouse.up();
  await page.waitForTimeout(250);
  const afterDrag = (await layout(page)).find(w=>w.id.endsWith('/primary'));
  if(afterDrag.x===beforeDrag.x) {
    console.log('Drag diagnostic', {beforeDrag,afterDrag,dom:await heading.locator('xpath=ancestor::div[contains(@class,"grid-stack-item") and not(contains(@class,"content"))]').evaluate(el=>({x:el.gridstackNode.x,y:el.gridstackNode.y,classes:el.className})),errors});
    await page.screenshot({path:'/tmp/gryphdash-drag-debug.png',fullPage:true});
  }
  assert.notEqual(afterDrag.x,beforeDrag.x,'pointer drag did not persist');
  const handle = heading.locator('xpath=ancestor::div[contains(@class,"grid-stack-item") and not(contains(@class,"content"))]').locator('.ui-resizable-se');
  const handleBox = await handle.boundingBox();
  const beforeResize = (await layout(page)).find(w=>w.id.endsWith('/primary'));
  await page.mouse.move(handleBox.x+handleBox.width/2,handleBox.y+handleBox.height/2);
  await page.mouse.down();
  await page.mouse.move(handleBox.x+handleBox.width/2+100,handleBox.y+handleBox.height/2+75,{steps:12});
  await page.mouse.up();
  await page.waitForTimeout(250);
  const afterResize = (await layout(page)).find(w=>w.id.endsWith('/primary'));
  assert.ok(afterResize.w!==beforeResize.w || afterResize.h!==beforeResize.h,'pointer resize did not persist');
  const saved = await layout(page);
  await loaded(page);
  assert.deepEqual(await layout(page), saved);
  assert.equal(await page.getByRole('heading',{name:'Credits remaining',exact:true}).count(),0);
  // Polling updates body content without dropping selection, layout, or editing state.
  fixture.widgets[0].value = '74%'; fixture.widgets[0].percent = 74;
  await page.getByRole('button',{name:'Edit dashboard',exact:true}).click();
  await page.waitForFunction(() => [...document.querySelectorAll('.value')].some(el=>el.textContent==='74%'), null, {timeout:20000});
  assert.equal(await page.getByRole('button',{name:'Done editing',exact:true}).count(),1);
  assert.deepEqual(await layout(page), saved);
  // Reload directly on mobile too, rather than only resizing an existing page.
  const mobileContext = await context({viewport: {width:390,height:844}});
  await mobileContext.addInitScript(({key, saved}) => localStorage.setItem(key, JSON.stringify({version:1,items:saved})), {key,saved});
  const mobilePage = await mobileContext.newPage(); await loaded(mobilePage);
  await mobilePage.setViewportSize({width:1440,height:1000});
  await mobilePage.waitForTimeout(350);
  const restoredPositions = await mobilePage.locator('.grid-stack-item').evaluateAll(els => els.map(el => {
    const {id,x,y,w,h}=el.gridstackNode; return {id,x,y,w,h};
  }));
  assert.deepEqual(restoredPositions.sort((a,b)=>a.id.localeCompare(b.id)), [...saved].sort((a,b)=>a.id.localeCompare(b.id)));
  await mobileContext.close();
  // Mobile must fit the viewport and must not overwrite the saved desktop arrangement.
  await page.setViewportSize({width:390,height:844});
  await page.waitForTimeout(350);
  assert.equal(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth),true);
  const boxes = await page.locator('.grid-stack-item').evaluateAll(els=>els.map(el=>({x:el.getBoundingClientRect().x,w:el.getBoundingClientRect().width})));
  assert.ok(boxes.every(b=>b.w>300 && b.w<390));
  assert.deepEqual(await layout(page),saved);
  await page.screenshot({path:'/tmp/gryphdash-widgets-mobile.png',fullPage:true});
  await page.setViewportSize({width:1440,height:1000});
  await page.waitForTimeout(350);
  await page.screenshot({path:'/tmp/gryphdash-widgets-desktop.png',fullPage:true});
  // An intentionally empty dashboard stays empty across reloads.
  while(await page.locator('.remove-widget').count()) await page.locator('.remove-widget').first().click();
  assert.deepEqual(await layout(page),[]);
  await loaded(page);
  assert.equal(await page.locator('.grid-stack-item').count(),0);
  assert.equal(await page.locator('#empty-state').isVisible(),true);
  await page.getByRole('button',{name:'Edit dashboard',exact:true}).click();
  await page.getByRole('button',{name:'Restore defaults',exact:true}).click();
  assert.equal(await page.locator('.grid-stack-item').count(),5);
  assert.deepEqual(errors,[]);
  await ctx.close();
  // Corrupt storage and blocked storage both leave a usable dashboard.
  const corrupt = await context();
  await corrupt.addInitScript(k=>localStorage.setItem(k,'{bad json'),key);
  const corruptPage=await corrupt.newPage();await loaded(corruptPage);
  assert.equal(await corruptPage.locator('.grid-stack-item').count(),5);
  assert.match(await corruptPage.locator('#layout-status').textContent(),/defaults/);
  await corrupt.close();
  const blocked = await context();
  await blocked.addInitScript(()=>{Storage.prototype.setItem=()=>{throw new Error('blocked')};});
  const blockedPage=await blocked.newPage();await loaded(blockedPage);
  await pickerAdd(blockedPage,'Authentication');
  assert.match(await blockedPage.locator('#layout-status').textContent(),/unavailable/);
  assert.equal(await blockedPage.locator('.grid-stack-item').count(),6);
  await blocked.close();
  console.log('Browser checks passed: picker, removal, pointer drag/resize, keyboard resize, persistence, live updates, mobile, empty layout, defaults, storage recovery, and safe text rendering.');
} finally {
  await browser?.close();
  if (server.exitCode===null) {const stopped=once(server,'exit');server.kill('SIGTERM');await stopped;}
}
