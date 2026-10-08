// Tests session isolation at the DOM/fetch boundary; not a browser-layout test.
const { test } = require('node:test');
const assert = require('node:assert/strict');
const { readFileSync } = require('node:fs');
const vm = require('node:vm');
function page(fetch) {
  const elements = new Map();
  function element() {
    return { textContent: '', value: '', disabled: false, children: [],
      replaceChildren() { this.children = []; this.textContent = ''; },
      append(...items) { this.children.push(...items); } };
  }
  const document = {
    getElementById(id) { if (!elements.has(id)) elements.set(id, element()); return elements.get(id); },
    createElement: element
  };
  const context = vm.createContext({ document, fetch, AbortController, confirm: () => true });
  vm.runInContext(readFileSync('web/app.js', 'utf8'), context);
  return { elements, get: document.getElementById,
    state: vm.runInContext('({api, clearSession, renderChat})', context) };
}
test('switching identity clears rendered data and ignores a late old response', async () => {
  let complete;
  const ui = page(() => new Promise(resolve => { complete = resolve; }));
  ui.state.renderChat({ id: 'old', status: 'awaiting_approval', answer: 'private old answer', messages: ['private'] });
  const pending = ui.state.api('overview');
  ui.state.clearSession();
  complete({ ok: true, json: async () => ({ secret: 'old tenant' }) });
  await assert.rejects(pending, /会话已切换/);
  assert.equal(ui.get('answer').textContent, '');
  assert.equal(ui.get('transcript').textContent, '');
  assert.equal(ui.get('continue').disabled, true);
});
test('late network failure cannot overwrite a new identity screen', async () => {
  let reject;
  const ui = page(() => new Promise((_, fail) => { reject = fail; }));
  ui.get('mode').value = 'local'; ui.get('task').value = 'read';
  const pending = ui.get('execute').onclick();
  ui.state.clearSession();
  ui.get('answer').textContent = 'new identity';
  reject(new TypeError('network lost'));
  await pending;
  assert.equal(ui.get('answer').textContent, 'new identity');
  assert.equal(ui.get('notice').textContent, '');
});

test('follow-up sends the selected parent and disables itself while approval is pending', async () => {
  const requests = [];
  const overview = { identity: { user: 'alice', tenant: 'acme', role: 'operator' },
    policy: {}, audit: [], chats: [], approvals: [], runs: [], tools: [] };
  const ui = page(async (url, options) => {
    requests.push([url, options.body && JSON.parse(options.body)]);
    return { ok: true, json: async () => url.startsWith('/api/overview') ? overview :
      { id: 'next', status: 'awaiting_approval', answer: 'approval needed', messages: [] } };
  });
  ui.get('mode').value = 'local';
  ui.state.renderChat({ id: 'parent', status: 'succeeded', messages: [], answer: 'read' });
  assert.equal(ui.get('followup').disabled, false);
  ui.get('task').value = 'update that ticket';
  await ui.get('followup').onclick();
  assert.deepEqual(requests[0], ['/api/chat/continue', { chat_id: 'parent', task: 'update that ticket' }]);
  assert.equal(ui.get('followup').disabled, true);
  assert.equal(ui.get('continue').disabled, false);
  ui.state.clearSession();
  assert.equal(ui.get('followup').disabled, true);
});
test('a pending network turn cannot dispatch a duplicate conversation', async () => {
  let complete;
  let calls = 0;
  const ui = page(() => { calls++; return new Promise(resolve => { complete = resolve; }); });
  ui.get('mode').value = 'local';
  ui.state.renderChat({ id: 'parent', status: 'succeeded', messages: [] });
  ui.get('task').value = 'follow up';
  const pending = ui.get('followup').onclick();
  await ui.get('execute').onclick();
  assert.equal(calls, 1);
  assert.equal(ui.get('execute').disabled, true);
  ui.state.clearSession();
  complete({ ok: true, json: async () => ({ id: 'old', status: 'succeeded', messages: [] }) });
  await pending;
  assert.equal(ui.get('answer').textContent, '');
});
