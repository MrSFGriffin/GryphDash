/* GridStack handles layout; metric content is always rendered as text/DOM nodes. */
(() => {
  'use strict';
  const STORAGE_KEY = 'gryphdash.layout.v1';
  const $ = id => document.getElementById(id);
  const catalog = new Map();
  const picker = $('widget-picker');
  let ready = false;
  let editing = false;
  let restoring = false;
  let grid;
  let pickerGroup = 'All';

  function element(tag, className, text) {
    const el = document.createElement(tag);
    if (className) el.className = className;
    if (text !== undefined) el.textContent = text;
    return el;
  }
  function readLayout() {
    try {
      const raw = localStorage.getItem(STORAGE_KEY);
      if (raw === null) return null;
      const saved = JSON.parse(raw);
      if (saved.version !== 1 || !Array.isArray(saved.items) || saved.items.length > 500) throw new Error('Invalid layout');
      const ids = new Set();
      return saved.items.map(item => {
        if (!item || typeof item.id !== 'string' || !/^(codex|openrouter)\//.test(item.id) || item.id.length > 1000 || ids.has(item.id)) throw new Error('Invalid widget');
        ids.add(item.id);
        const clean = {id: item.id};
        for (const [key, min, max] of [['x', 0, 11], ['y', 0, 10000], ['w', 1, 12], ['h', 2, 30]]) {
          if (!Number.isInteger(item[key]) || item[key] < min || item[key] > max) throw new Error('Invalid position');
          clean[key] = item[key];
        }
        if (clean.x + clean.w > 12) throw new Error('Invalid width');
        return clean;
      });
    } catch (error) {
      $('layout-status').textContent = 'Saved layout unavailable; using defaults.';
      return null;
    }
  }
  const savedLayout = readLayout();
  function saveLayout() {
    if (!ready || restoring) return;
    try {
      // Ask for the desktop layout even when the grid is currently one column.
      const items = grid.save(false, false, undefined, 12).map(({id, x, y, w, h}) => ({id, x: x ?? 0, y: y ?? 0, w: w ?? 4, h: h ?? 4}));
      localStorage.setItem(STORAGE_KEY, JSON.stringify({version: 1, items}));
      $('layout-status').textContent = 'Layout saved in this browser';
    } catch (error) {
      $('layout-status').textContent = 'Browser storage unavailable; layout changes last for this visit.';
    }
  }
  function activeIDs() { return new Set(grid.getGridItems().map(el => el.gridstackNode.id)); }
  function emptyState() { $('empty-state').hidden = !ready || grid.getGridItems().length !== 0; }
  function missingWidget(id) {
    const group = id.startsWith('openrouter/') ? 'OpenRouter' : 'Codex';
    return {id, title: 'Metric unavailable', group, kind: 'metric', value: 'Unavailable', note: 'This saved metric is not in the latest response. It will return here when available.', status: 'Waiting for metric', width: 4, height: 4};
  }
  function renderBody(container, w) {
    const body = container.querySelector('.widget-body');
    const scrollTop = body.scrollTop;
    body.replaceChildren();
    body.append(element('p', 'value', w.value));
    if (w.percent !== undefined) {
      const progress = element('progress');
      progress.max = 100;
      progress.value = Math.max(0, Math.min(100, w.percent));
      progress.setAttribute('aria-label', `${w.title}: ${w.value} remaining`);
      body.append(progress);
    }
    if (w.note) body.append(element('p', 'note', w.note));
    if (w.resetsAt !== undefined) {
      const date = new Date(w.resetsAt * 1000);
      body.append(element('p', 'note', `Resets ${date.toISOString().replace('T', ' ').slice(0, 16)} UTC`));
      const countdown = element('p', 'countdown');
      countdown.dataset.resetsAt = w.resetsAt;
      body.append(countdown);
    }
    if (w.kind === 'daily' && w.days?.length) {
      const table = element('table');
      const head = table.createTHead().insertRow();
      for (const text of ['Date', 'Tokens', 'Activity']) { const th = element('th', '', text); th.scope = 'col'; head.append(th); }
      const tbody = table.createTBody();
      for (const day of w.days) {
        const row = tbody.insertRow();
        row.insertCell().textContent = day.date;
        row.insertCell().textContent = day.tokens;
        const bar = element('meter');
        bar.min = 0; bar.max = 100; bar.value = day.percent;
        bar.setAttribute('aria-label', `Relative activity on ${day.date}`);
        row.insertCell().append(bar);
      }
      body.append(table);
    }
    if (w.kind === 'resets') {
      for (const reset of w.resets || []) {
        const section = element('section', 'reset-detail');
        section.append(element('h3', '', reset.title), element('p', 'note', reset.description));
        const dl = element('dl');
        for (const [label, text] of [['Status', reset.status], ['Type', reset.type], ['Granted', reset.granted], ['Expires', reset.expires], ['ID', reset.id]]) dl.append(element('dt', '', label), element('dd', '', text));
        section.append(dl); body.append(section);
      }
    }
    body.scrollTop = scrollTop;
    container.querySelector('.widget-title').textContent = w.title;
    container.querySelector('.widget-group').textContent = w.group;
    const status = container.querySelector('.widget-status');
    status.textContent = w.status;
    status.classList.toggle('stale', !w.status.startsWith('Updated '));
    container.querySelector('.remove-widget').setAttribute('aria-label', `Remove ${w.title}`);
    container.setAttribute('aria-label', `${w.group}: ${w.title}`);
  }
  function keyboardLayout(event, container) {
    if (!editing || !['ArrowLeft', 'ArrowRight', 'ArrowUp', 'ArrowDown'].includes(event.key)) return;
    event.preventDefault();
    const el = container.closest('.grid-stack-item');
    const n = el.gridstackNode;
    const dx = event.key === 'ArrowRight' ? 1 : event.key === 'ArrowLeft' ? -1 : 0;
    const dy = event.key === 'ArrowDown' ? 1 : event.key === 'ArrowUp' ? -1 : 0;
    if (event.shiftKey) grid.update(el, {w: Math.max(1, Math.min(grid.getColumn() - n.x, n.w + dx)), h: Math.max(2, Math.min(30, n.h + dy))});
    else grid.update(el, {x: Math.max(0, Math.min(grid.getColumn() - n.w, n.x + dx)), y: Math.max(0, n.y + dy)});
    saveLayout();
  }
  GridStack.renderCB = (container, node) => {
    const w = catalog.get(node.id) || missingWidget(node.id);
    const header = element('div', 'widget-header');
    const heading = element('div', 'widget-heading');
    heading.tabIndex = editing ? 0 : -1;
    heading.setAttribute('aria-describedby', 'edit-help');
    heading.append(element('p', 'widget-group'), element('h2', 'widget-title'));
    heading.addEventListener('keydown', event => keyboardLayout(event, container));
    const remove = element('button', 'remove-widget', '×');
    remove.addEventListener('click', () => {
      grid.removeWidget(container.closest('.grid-stack-item'));
      saveLayout(); emptyState(); $('edit-layout').focus();
    });
    header.append(heading, remove);
    container.append(header, element('div', 'widget-body'), element('p', 'widget-status'));
    renderBody(container, w);
  };
  grid = GridStack.init({
    column: 12, cellHeight: 72, margin: 8, float: true,
    handle: '.widget-heading', minRow: 1,
    disableDrag: true, disableResize: true,
    resizable: {handles: 'se'}
  });
  function responsiveColumns() {
    if (!ready) return;
    const columns = window.innerWidth <= 700 ? 1 : window.innerWidth <= 1000 ? 6 : 12;
    if (grid.getColumn() !== columns) grid.column(columns);
  }
  window.addEventListener('resize', responsiveColumns);
  grid.on('dragstop resizestop', () => { saveLayout(); });
  function addWidget(id, placement = {}) {
    if (activeIDs().has(id)) return;
    const w = catalog.get(id) || missingWidget(id);
    grid.addWidget({id, w: w.width, h: w.height, minH: 2, maxH: 30, ...placement});
    grid.enableMove(editing); grid.enableResize(editing);
  }
  function loadDefaults() {
    grid.column(12);
    grid.batchUpdate();
    grid.removeAll();
    for (const w of catalog.values()) if (w.default) addWidget(w.id);
    grid.batchUpdate(false);
    responsiveColumns();
  }
  function setEditing(on) {
    editing = on;
    document.body.classList.toggle('editing', on);
    grid.enableMove(on); grid.enableResize(on);
    $('edit-layout').textContent = on ? 'Done editing' : 'Edit dashboard';
    $('edit-layout').setAttribute('aria-pressed', String(on));
    $('edit-help').hidden = !on;
    $('restore-defaults').hidden = !on;
    document.querySelectorAll('.widget-heading').forEach(el => { el.tabIndex = on ? 0 : -1; });
  }
  function renderPicker() {
    const query = $('widget-search').value.trim().toLowerCase().split(/\s+/).filter(Boolean);
    const active = activeIDs();
    const list = $('widget-list'); list.replaceChildren();
    const groupNames = [...new Set([...catalog.values()].map(w => w.group))].sort();
    const tabs = $('picker-tabs'); tabs.replaceChildren();
    for (const name of ['All', ...groupNames]) {
      const tab = element('button', name === pickerGroup ? 'picker-tab active' : 'picker-tab', name);
      tab.type = 'button'; tab.setAttribute('role', 'tab'); tab.setAttribute('aria-selected', String(name === pickerGroup));
      tab.addEventListener('click', () => { pickerGroup = name; renderPicker(); });
      tabs.append(tab);
    }
    const groups = new Map();
    for (const w of catalog.values()) {
      if (pickerGroup !== 'All' && w.group !== pickerGroup) continue;
      const searchable = `${w.title} ${w.group} ${w.description || ''}`.toLowerCase();
      if (!query.every(term => searchable.includes(term))) continue;
      if (!groups.has(w.group)) groups.set(w.group, []);
      groups.get(w.group).push(w);
    }
    for (const [name, widgets] of groups) {
      list.append(element('h3', 'picker-group', name));
      for (const w of widgets) {
        const row = element('div', 'picker-row');
        const button = element('button', '', active.has(w.id) ? 'Added' : 'Add');
        button.disabled = active.has(w.id);
        button.setAttribute('aria-label', `Add ${w.title} (${w.group})`);
        button.addEventListener('click', () => {
          addWidget(w.id); saveLayout(); emptyState(); updateCountdowns();
          button.textContent = 'Added'; button.disabled = true;
        });
        row.append(element('p', '', w.title), button); list.append(row);
      }
    }
    if (!groups.size) list.append(element('p', 'note', 'No matching widgets. Try another search.'));
  }
  function openPicker() {
    $('widget-search').value = ''; renderPicker(); picker.showModal(); $('widget-search').focus();
  }
  $('add-widgets').addEventListener('click', openPicker);
  $('empty-add').addEventListener('click', openPicker);
  $('close-picker').addEventListener('click', () => picker.close());
  picker.addEventListener('click', event => { if (event.target === picker) picker.close(); });
  $('widget-search').addEventListener('input', renderPicker);
  $('edit-layout').addEventListener('click', () => setEditing(!editing));
  $('restore-defaults').addEventListener('click', () => { loadDefaults(); saveLayout(); emptyState(); updateCountdowns(); });
  function updateCountdowns() {
    document.querySelectorAll('[data-resets-at]').forEach(el => {
      const seconds = Math.max(0, Math.ceil(Number(el.dataset.resetsAt) - Date.now() / 1000));
      const days = Math.floor(seconds / 86400);
      const hours = Math.floor(seconds % 86400 / 3600);
      const minutes = Math.floor(seconds % 3600 / 60);
      const rest = seconds % 60;
      el.textContent = seconds ? `Resets in ${days ? days + 'd ' : ''}${hours}h ${minutes}m ${rest}s` : 'Awaiting updated window';
    });
  }
  async function refresh() {
    try {
      const response = await fetch('/api/widgets', {cache: 'no-store', signal: AbortSignal.timeout(10000)});
      if (!response.ok) throw new Error('Dashboard request failed');
      const data = await response.json();
      if (!Array.isArray(data.widgets)) throw new Error('Invalid widget response');
      catalog.clear();
      for (const w of data.widgets) catalog.set(w.id, w);
      if (!ready) {
        restoring = true;
        if (savedLayout !== null) {
          grid.load(savedLayout.map(item => ({...item, minH: 2, maxH: 30})));
        } else loadDefaults();
        ready = true; restoring = false;
        // Establish the 12-column layout before adapting to mobile, so reloads
        // on a phone retain the exact desktop coordinates in GridStack's cache.
        responsiveColumns();
        $('add-widgets').disabled = false; $('edit-layout').disabled = false;
        // Save the initial selection too, but preserve recovery messages on bad storage.
        if (savedLayout === null && $('layout-status').textContent === 'Layout saved in this browser') saveLayout();
      }
      for (const el of grid.getGridItems()) renderBody(el.querySelector('.grid-stack-item-content'), catalog.get(el.gridstackNode.id) || missingWidget(el.gridstackNode.id));
      $('connection-status').textContent = 'Connected · widget values refresh automatically';
      $('connection-status').classList.remove('stale');
      emptyState(); updateCountdowns();
    } catch (error) {
      $('connection-status').textContent = 'Dashboard connection lost. Displayed values may be stale; retrying automatically.';
      $('connection-status').classList.add('stale');
    } finally {
      setTimeout(refresh, 15000);
    }
  }
  setInterval(updateCountdowns, 1000);
  refresh();
})();
