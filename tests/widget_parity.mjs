import assert from 'node:assert/strict';
import {readFileSync} from 'node:fs';
import test from 'node:test';
import {fileURLToPath} from 'node:url';
import path from 'node:path';

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
const readFixture = name => JSON.parse(readFileSync(path.join(root, 'testdata/widget-parity', name)));
const dashboard = readFixture('current-dashboard.json');
const webDOM = readFixture('current-web-dom.json');

test('web DOM parity fixture represents every current widget', () => {
  assert.equal(webDOM.cards.length, dashboard.widgets.length);
  const cards = new Map(webDOM.cards.map(card => [card.id, card]));
  for (const widget of dashboard.widgets) {
    const card = cards.get(widget.id);
    assert.ok(card, `missing web DOM fixture for ${widget.id}`);
    assert.equal(card.ariaLabel, `${widget.group}: ${widget.title}`);
    assert.equal(card.title, widget.title);
    assert.equal(card.group, widget.group);
    assert.equal(card.status, widget.status);
    assert.equal(card.stale, !widget.status.startsWith('Updated '));
    assert.equal(card.removeAriaLabel, `Remove ${widget.title}`);
    assert.deepEqual(card.body[0], {tag: 'p', class: 'value', text: widget.value});

    const progress = card.body.find(node => node.tag === 'progress');
    assert.equal(Boolean(progress), widget.percent !== undefined);
    if (progress) {
      assert.equal(progress.max, 100);
      assert.equal(progress.value, widget.percent || undefined);
      assert.equal(progress.ariaLabel, `${widget.title}: ${widget.value} remaining`);
    }

    const reset = card.body.find(node => node.class === 'countdown');
    assert.equal(Boolean(reset), widget.resetsAt !== undefined);
    if (reset) assert.equal(reset.resetsAt, String(widget.resetsAt));

    const table = card.body.find(node => node.tag === 'table');
    assert.equal(Boolean(table), widget.kind === 'daily' && widget.days?.length > 0);
    if (table) assert.deepEqual(table.rows.map(row => row.cells.slice(0, 2)), widget.days.map(day => [day.date, day.tokens]));

    const resets = card.body.filter(node => node.class === 'reset-detail');
    assert.equal(resets.length, widget.kind === 'resets' ? widget.resets.length : 0);
  }
});
