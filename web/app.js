'use strict';
let token = '', offset = 0, currentChat = null, session = 0;
const requests = new Set();
const $ = id => document.getElementById(id);
const show = (id, value) => {
  $(id).textContent = typeof value === 'string' ? value : JSON.stringify(value, null, 2);
};

function clearSession() {
  session++;
  for (const controller of requests) controller.abort();
  requests.clear();
  token = ''; offset = 0; currentChat = null;
  for (const id of ['identity', 'answer', 'transcript', 'chat-list', 'approval-list',
    'run-list', 'tool-list', 'policy-info', 'audit-list', 'notice']) $(id).replaceChildren();
  $('continue').disabled = true;
  $('reload-chat').disabled = true;
  $('execute').disabled = false;
  show('connection', '未连接');
  show('page', '第 1 页');
}

async function api(path, body) {
  const epoch = session;
  const controller = new AbortController();
  requests.add(controller);
  try {
    const response = await fetch('/api/' + path, {
      method: body === undefined ? 'GET' : 'POST',
      headers: { Authorization: 'Bearer ' + token, 'Content-Type': 'application/json' },
      body: body === undefined ? undefined : JSON.stringify(body), signal: controller.signal
    });
    const data = await response.json();
    // Ignore replies belonging to a previous identity, even if abort lost a race.
    if (epoch !== session) throw new Error('会话已切换');
    if (!response.ok) throw new Error(`${response.status} ${data.error} · ${data.request_id || ''}`);
    return data;
  } finally { requests.delete(controller); }
}

async function act(fn, button) {
  if (button) button.disabled = true;
  const epoch = session;
  try {
    await fn();
    if (epoch === session) show('notice', '操作完成，数据已由后端返回。');
  } catch (error) {
    if (epoch === session) show('notice', error.message);
  } finally {
    if (button && epoch === session) button.disabled = button.id === 'continue' && currentChat?.status !== 'awaiting_approval';
  }
}
function button(text, fn, className = '') {
  const element = document.createElement('button');
  element.textContent = text; element.className = className;
  element.onclick = () => act(fn, element);
  return element;
}
function renderChat(chat) {
  currentChat = chat;
  show('answer', chat.answer || chat.status);
  show('transcript', chat.messages);
  $('continue').disabled = chat.status !== 'awaiting_approval';
  $('reload-chat').disabled = false;
}

async function refresh() {
  const data = await api('overview?offset=' + offset);
  show('identity', `${data.identity.user} / 租户 ${data.identity.tenant} / ${data.identity.role}`);
  show('connection', '已连接 · ' + data.identity.tenant);
  show('policy-info', data.policy); show('audit-list', data.audit);
  show('page', '第 ' + (offset / 50 + 1) + ' 页');
  $('chat-list').replaceChildren();
  for (const chat of data.chats || []) {
    $('chat-list').append(button(chat.id.slice(0, 8) + ' · ' + chat.status,
      async () => renderChat(await api('chat?id=' + encodeURIComponent(chat.id))), 'secondary'));
  }
  $('approval-list').replaceChildren();
  for (const action of data.approvals) {
    const card = document.createElement('div'); card.className = 'card';
    const title = document.createElement('strong'); title.textContent = action.tool + ' · ' + action.status;
    const preview = document.createElement('pre');
    preview.textContent = JSON.stringify({ params: JSON.parse(action.params), digest: action.digest,
      expires: new Date(action.expires * 1000).toISOString() }, null, 2);
    card.append(title, preview);
    if (action.status === 'pending') card.append(button('确认以上参数并执行', async () => {
      show('answer', await api('approve', { action_id: action.id, digest: action.digest }));
      await refresh();
    }));
    $('approval-list').append(card);
  }
  if (!data.approvals.length) show('approval-list', '当前没有审批记录。');
  $('run-list').replaceChildren();
  for (const run of data.runs) {
    const row = document.createElement('tr');
    for (const key of ['id', 'agent', 'status', 'steps']) {
      const cell = document.createElement('td'); cell.textContent = run[key]; row.append(cell);
    }
    const cell = document.createElement('td');
    if (run.status === 'running') cell.append(button('取消', async () => {
      await api('cancel', { run_id: run.id });
      if (currentChat?.run_id === run.id) renderChat(await api('chat?id=' + encodeURIComponent(currentChat.id)));
      await refresh();
    }, 'secondary'));
    row.append(cell); $('run-list').append(row);
  }
  $('tool-list').replaceChildren();
  for (const tool of data.tools) {
    const item = document.createElement('pre');
    item.textContent = `${tool.name}\n${tool.action} · ${tool.risk}\n超时 ${tool.timeout_ms}ms · 副作用 ${tool.side_effect}`;
    $('tool-list').append(item);
  }
}

$('connect').onclick = () => {
  const credential = $('token').value.trim(); $('token').value = '';
  clearSession(); token = credential;
  return act(async () => {
    const capabilities = await api('capabilities');
    $('mode').value = capabilities.local_model ? 'local' : 'mock';
    show('mode-help', capabilities.local_model
      ? '本地模型已配置：输入自然语言，读取数据、提出修改，审批后继续回答。'
      : '未配置本地模型，当前使用固定命令。按本地模型文档启动后重新连接。');
    $('task').value = capabilities.local_model ? '请读取 doc-1 文档并用中文概括内容。' : 'read-doc doc-1';
    await refresh();
  }, $('connect'));
};
$('refresh').onclick = () => act(refresh, $('refresh'));
$('execute').onclick = () => act(async () => {
  const epoch = session;
  currentChat = null; show('transcript', ''); $('continue').disabled = true; $('reload-chat').disabled = true;
  show('answer', '正在执行，请稍候。本地模型首次加载可能较慢。');
  try {
    if ($('mode').value === 'local') renderChat(await api('chat', { task: $('task').value }));
    else show('answer', await api('agent', { task: $('task').value }));
    await refresh();
  } catch (error) {
    if (epoch === session && error.name !== 'AbortError' && error.message !== '会话已切换') {
      show('answer', '本次执行未完成：' + error.message);
    }
    throw error;
  }
}, $('execute'));
$('continue').onclick = () => act(async () => {
  if (!currentChat) return;
  renderChat(await api('chat/resume', { chat_id: currentChat.id })); await refresh();
}, $('continue'));
$('reload-chat').onclick = () => act(async () => {
  if (currentChat) renderChat(await api('chat?id=' + encodeURIComponent(currentChat.id)));
  await refresh();
}, $('reload-chat'));
$('revoke').onclick = () => act(async () => {
  if (!confirm('撤销当前身份，后续访问将返回 401。确认继续？')) return;
  await api('revoke', {}); clearSession(); show('connection', '身份已撤销');
}, $('revoke'));
$('prev').onclick = () => act(async () => { offset = Math.max(0, offset - 50); await refresh(); });
$('next').onclick = () => act(async () => { offset = Math.min(100000, offset + 50); await refresh(); });
$('mode').onchange = () => {
  $('task').value = $('mode').value === 'local' ? '请读取 doc-1 文档并用中文概括内容。' : 'read-doc doc-1';
};
