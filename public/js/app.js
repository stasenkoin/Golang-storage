let currentUser = null;
let currentTab = 'my';
let pendingShareToken = null;

function val(id) {
    return document.getElementById(id).value;
}

function escapeHtml(s) {
    return String(s).replace(/[&<>"]/g, c => ({'&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;'}[c]));
}

function formatBytes(b) {
    if (b < 1024) return b + ' Б';
    if (b < 1048576) return (b / 1024).toFixed(1) + ' КБ';
    return (b / 1048576).toFixed(1) + ' МБ';
}

function formatMB(bytes) {
    return Math.round(bytes / 1048576) + ' МБ';
}

const COMMON_EXTENSIONS = ['pdf', 'txt', 'doc', 'docx', 'png', 'jpg', 'jpeg', 'gif', 'zip', 'csv', 'xlsx'];

function extCheckboxes(selected) {
    selected = selected || [];
    const all = [...COMMON_EXTENSIONS];
    selected.forEach(e => {
        if (!all.includes(e)) all.push(e);
    });
    return all.map(ext =>
        `<label class="ext"><input type="checkbox" value="${escapeHtml(ext)}" ${selected.includes(ext) ? 'checked' : ''}>${escapeHtml(ext)}</label>`
    ).join('');
}

function readCheckedExts(containerId) {
    const boxes = document.querySelectorAll('#' + containerId + ' input[type=checkbox]:checked');
    return [...boxes].map(b => b.value);
}

function toggleExt(prefix) {
    const restricted = document.querySelector('input[name="' + prefix + '-mode"]:checked').value === 'restricted';
    document.getElementById(prefix + '-ext-wrap').style.display = restricted ? 'block' : 'none';
}

function getAccess() {
    return localStorage.getItem('access_token');
}

function getRefresh() {
    return localStorage.getItem('refresh_token');
}

function setTokens(a, r) {
    localStorage.setItem('access_token', a);
    if (r) localStorage.setItem('refresh_token', r);
}

function clearTokens() {
    localStorage.removeItem('access_token');
    localStorage.removeItem('refresh_token');
}

function jsonOpts(method, body) {
    return {method, headers: {'Content-Type': 'application/json'}, body: JSON.stringify(body)};
}

async function apiFetch(path, options = {}, retry = true) {
    options.headers = options.headers || {};
    const access = getAccess();
    if (access) options.headers['Authorization'] = 'Bearer ' + access;

    const res = await fetch(path, options);

    // access протух — пробуем один раз обновить его по refresh-токену и повторить
    if (res.status === 401 && retry && getRefresh()) {
        const ok = await tryRefresh();
        if (ok) return apiFetch(path, options, false);
    }
    return res;
}

async function tryRefresh() {
    const res = await fetch('/api/auth/refresh', jsonOpts('POST', {refresh_token: getRefresh()}));
    if (!res.ok) {
        clearTokens();
        return false;
    }
    const data = await res.json();
    setTokens(data.access_token, data.refresh_token);
    return true;
}

function showAuthScreen() {
    document.getElementById('auth-screen').style.display = 'flex';
    document.getElementById('app-screen').style.display = 'none';
}

function showAuthForm(which) {
    document.getElementById('login-form').style.display = which === 'login' ? 'block' : 'none';
    document.getElementById('register-form').style.display = which === 'register' ? 'block' : 'none';
    document.getElementById('show-login').classList.toggle('active', which === 'login');
    document.getElementById('show-register').classList.toggle('active', which === 'register');
    document.getElementById('auth-error').textContent = '';
}

async function onLogin(e) {
    e.preventDefault();
    try {
        const res = await fetch('/api/auth/login', jsonOpts('POST', {
            email: val('login-email'),
            password: val('login-password')
        }));
        if (!res.ok) throw new Error((await res.json().catch(() => ({}))).error || 'Ошибка входа');
        const data = await res.json();
        setTokens(data.access_token, data.refresh_token);
        currentUser = data.user;
        enterApp();
    } catch (err) {
        document.getElementById('auth-error').textContent = err.message;
    }
}

async function onRegister(e) {
    e.preventDefault();
    try {
        const email = val('register-email'), password = val('register-password');
        const res = await fetch('/api/auth/register', jsonOpts('POST', {email, password}));
        if (!res.ok) throw new Error((await res.json().catch(() => ({}))).error || 'Ошибка регистрации');
        const lr = await fetch('/api/auth/login', jsonOpts('POST', {email, password}));
        const data = await lr.json();
        setTokens(data.access_token, data.refresh_token);
        currentUser = data.user;
        enterApp();
    } catch (err) {
        document.getElementById('auth-error').textContent = err.message;
    }
}

async function onLogout() {
    const r = getRefresh();
    if (r) await fetch('/api/auth/logout', jsonOpts('POST', {refresh_token: r}));
    clearTokens();
    currentUser = null;
    showAuthScreen();
}

async function loadMe() {
    const res = await apiFetch('/api/auth/me');
    if (!res.ok) return false;
    currentUser = await res.json();
    return true;
}

async function enterApp() {
    document.getElementById('auth-screen').style.display = 'none';
    document.getElementById('app-screen').style.display = 'block';
    document.getElementById('user-email').textContent = currentUser.email;

    // если пришли по ссылке ?share=токен — открываем её (добавит файл в "Доступные мне")
    if (pendingShareToken) {
        const t = pendingShareToken;
        pendingShareToken = null;
        const res = await apiFetch('/api/shared/' + t);
        history.replaceState({}, '', '/'); // убираем ?share из адреса
        if (res.ok) {
            alert('Файл добавлен в «Доступные мне»');
            showTab('available');
            return;
        }
        alert('Ссылка недействительна или отозвана');
    }
    showTab('my');
}

function showTab(tab) {
    currentTab = tab;
    document.querySelectorAll('#app-screen .tab').forEach(b => b.classList.toggle('active', b.dataset.tab === tab));
    if (tab === 'my') renderMyStorages();
    else if (tab === 'global') renderGlobalStorages();
    else if (tab === 'available') renderAvailable();
}

async function renderMyStorages() {
    const content = document.getElementById('content');
    content.innerHTML = `
    <div class="bar"><h2>Мои хранилища</h2><button onclick="showCreateForm('personal')">+ Создать личное</button></div>
    <div id="create-area"></div>
    <div id="list">Загрузка…</div>`;
    const res = await apiFetch('/api/storages/my');
    renderStorageList(res.ok ? await res.json() : [], 'list');
}

async function renderGlobalStorages() {
    const content = document.getElementById('content');
    content.innerHTML = `
    <div class="bar"><h2>Глобальные хранилища</h2><button onclick="showCreateForm('global')">+ Создать глобальное</button></div>
    <div id="create-area"></div>
    <div id="list">Загрузка…</div>`;
    const res = await apiFetch('/api/storages/global');
    renderStorageList(res.ok ? await res.json() : [], 'list');
}

function renderStorageList(list, containerId) {
    const c = document.getElementById(containerId);
    if (!list.length) {
        c.innerHTML = '<p class="muted">Пока пусто.</p>';
        return;
    }
    c.innerHTML = list.map(s => {
        const isOwner = s.owner_id === currentUser.id;
        const types = s.allowed_extensions.length ? s.allowed_extensions.join(', ') : 'любые';
        return `<div class="card">
      <div class="card-title">${escapeHtml(s.name)} <span class="badge">${s.type}</span></div>
      <div class="muted">лимит файла: ${formatMB(s.max_file_size_bytes)} · типы: ${escapeHtml(types)}</div>
      <div class="actions">
        <button onclick="openStorage(${s.id})">Открыть</button>
        ${isOwner ? `<button onclick="showSettings(${s.id})">Настройки</button>
                     <button class="danger" onclick="deleteStorage(${s.id})">Удалить</button>` : ''}
      </div>
    </div>`;
    }).join('');
}

function showCreateForm(type) {
    document.getElementById('create-area').innerHTML = `
    <div class="form-card">
      <form onsubmit="onCreateStorage(event, '${type}')">
        <div class="field">
          <label class="title">Название</label>
          <input type="text" id="cs-name" placeholder="Например, Документы" required>
        </div>
        <div class="field">
          <label class="title">Максимальный размер файла</label>
          <span class="with-unit"><input id="cs-size" type="number" min="1" value="10" required> <span class="unit">МБ</span></span>
        </div>
        <div class="field">
          <label class="title">Типы файлов</label>
          <div class="radio-row">
            <label><input type="radio" name="cs-mode" value="any" checked onchange="toggleExt('cs')"> Любой тип</label>
            <label><input type="radio" name="cs-mode" value="restricted" onchange="toggleExt('cs')"> Ограниченные типы</label>
          </div>
          <div id="cs-ext-wrap" style="display:none">
            <div id="cs-ext" class="ext-list">${extCheckboxes([])}</div>
          </div>
        </div>
        <div class="actions">
          <button type="submit">Создать</button>
          <button type="button" onclick="document.getElementById('create-area').innerHTML=''">Отмена</button>
        </div>
      </form>
    </div>`;
}

async function onCreateStorage(e, type) {
    e.preventDefault();
    const mode = document.querySelector('input[name="cs-mode"]:checked').value;
    let ext = [];
    if (mode === 'restricted') {
        ext = readCheckedExts('cs-ext');
        if (ext.length === 0) {
            alert('Выберите хотя бы один тип файла или вариант «Любой тип».');
            return;
        }
    }
    const body = {name: val('cs-name'), type, max_file_size_mb: parseInt(val('cs-size'), 10), allowed_extensions: ext};
    const res = await apiFetch('/api/storages', jsonOpts('POST', body));
    if (!res.ok) {
        alert((await res.json().catch(() => ({}))).error || 'Ошибка');
        return;
    }
    type === 'personal' ? renderMyStorages() : renderGlobalStorages();
}

async function showSettings(id) {
    const res = await apiFetch('/api/storages/' + id);
    if (!res.ok) {
        alert('Ошибка');
        return;
    }
    const s = await res.json();
    document.getElementById('content').innerHTML = `
    <button onclick="showTab(currentTab)">← Назад</button>
    <h2>Настройки: ${escapeHtml(s.name)}</h2>
    <div class="form-card">
      <form onsubmit="onSaveSettings(event, ${id})">
        <div class="field">
          <label class="title">Название</label>
          <input type="text" id="set-name" value="${escapeHtml(s.name)}" required>
        </div>
        <div class="field">
          <label class="title">Максимальный размер файла</label>
          <span class="with-unit"><input id="set-size" type="number" min="1" value="${Math.round(s.max_file_size_bytes / 1048576)}" required> <span class="unit">МБ</span></span>
        </div>
        <div class="field">
          <label class="title">Типы файлов</label>
          <div class="radio-row">
            <label><input type="radio" name="set-mode" value="any" ${s.allowed_extensions.length ? '' : 'checked'} onchange="toggleExt('set')"> Любой тип</label>
            <label><input type="radio" name="set-mode" value="restricted" ${s.allowed_extensions.length ? 'checked' : ''} onchange="toggleExt('set')"> Ограниченные типы</label>
          </div>
          <div id="set-ext-wrap" style="display:${s.allowed_extensions.length ? 'block' : 'none'}">
            <div id="set-ext" class="ext-list">${extCheckboxes(s.allowed_extensions)}</div>
          </div>
        </div>
        <button type="submit">Сохранить</button>
      </form>
    </div>`;
}

async function onSaveSettings(e, id) {
    e.preventDefault();
    const mode = document.querySelector('input[name="set-mode"]:checked').value;
    let ext = [];
    if (mode === 'restricted') {
        ext = readCheckedExts('set-ext');
        if (ext.length === 0) {
            alert('Выберите хотя бы один тип файла или вариант «Любой тип».');
            return;
        }
    }
    const body = {name: val('set-name'), max_file_size_mb: parseInt(val('set-size'), 10), allowed_extensions: ext};
    const res = await apiFetch('/api/storages/' + id, jsonOpts('PATCH', body));
    if (!res.ok) {
        alert((await res.json().catch(() => ({}))).error || 'Ошибка');
        return;
    }
    showTab(currentTab);
}

async function deleteStorage(id) {
    if (!confirm('Удалить хранилище со всеми файлами?')) return;
    const res = await apiFetch('/api/storages/' + id, {method: 'DELETE'});
    if (!res.ok) {
        alert('Не удалось удалить');
        return;
    }
    showTab(currentTab);
}

async function openStorage(id) {
    const content = document.getElementById('content');
    content.innerHTML = 'Загрузка…';

    const sres = await apiFetch('/api/storages/' + id);
    if (!sres.ok) {
        content.innerHTML = '<p class="error">Нет доступа к хранилищу.</p><button onclick="showTab(currentTab)">← Назад</button>';
        return;
    }
    const s = await sres.json();

    const fres = await apiFetch('/api/storages/' + id + '/files');
    const files = fres.ok ? await fres.json() : [];

    const isOwner = s.owner_id === currentUser.id;
    const types = s.allowed_extensions.length ? s.allowed_extensions.join(', ') : 'любые';

    content.innerHTML = `
    <button onclick="showTab(currentTab)">← Назад</button>
    <h2>${escapeHtml(s.name)} <span class="badge">${s.type}</span></h2>
    <div class="muted">лимит файла: ${formatMB(s.max_file_size_bytes)} · типы: ${escapeHtml(types)}</div>

    <div class="upload">
      <input type="file" id="up-${id}">
      <button onclick="uploadFile(${id})">Загрузить</button>
    </div>

    <h3>Файлы</h3>
    <div id="files">${renderFiles(files, s, isOwner)}</div>
    ${isOwner && s.type === 'personal' ? '<div id="perms"></div>' : ''}`;

    if (isOwner && s.type === 'personal') renderPermissions(id);
}

function renderFiles(files, s, isOwner) {
    if (!files.length) return '<p class="muted">Файлов нет.</p>';
    return files.map(f => {
        const mine = isOwner || f.uploaded_by === currentUser.id;
        return `<div class="row">
      <span>${escapeHtml(f.original_name)} <span class="muted">(${formatBytes(f.size_bytes)})</span></span>
      <span class="actions">
        <button onclick="downloadFile(${f.id})">Скачать</button>
        ${mine ? `<button onclick="showLinks(${f.id}, ${s.id})">Ссылки</button>
                  <button class="danger" onclick="deleteFile(${f.id}, ${s.id})">Удалить</button>` : ''}
      </span>
    </div>`;
    }).join('');
}

async function uploadFile(storageId) {
    const input = document.getElementById('up-' + storageId);
    if (!input.files.length) {
        alert('Выберите файл');
        return;
    }
    const fd = new FormData();
    fd.append('file', input.files[0]);
    const res = await apiFetch('/api/storages/' + storageId + '/files', {method: 'POST', body: fd});
    if (!res.ok) {
        alert((await res.json().catch(() => ({}))).error || 'Ошибка загрузки');
        return;
    }
    openStorage(storageId);
}

async function downloadFile(fileId) {
    const res = await apiFetch('/api/files/' + fileId + '/download');
    if (!res.ok) {
        alert('Не удалось скачать');
        return;
    }
    const blob = await res.blob();
    let name = 'file';
    const cd = res.headers.get('Content-Disposition') || '';
    const m = cd.match(/filename="(.*)"/);
    if (m) name = m[1];
    const url = URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url;
    a.download = name;
    a.click();
    URL.revokeObjectURL(url);
}

async function deleteFile(fileId, storageId) {
    if (!confirm('Удалить файл?')) return;
    const res = await apiFetch('/api/files/' + fileId, {method: 'DELETE'});
    if (!res.ok) {
        alert((await res.json().catch(() => ({}))).error || 'Не удалось удалить');
        return;
    }
    openStorage(storageId);
}

async function renderPermissions(storageId) {
    const el = document.getElementById('perms');
    if (!el) return;
    const res = await apiFetch('/api/storages/' + storageId + '/permissions');
    const list = res.ok ? await res.json() : [];
    el.innerHTML = `
    <h3>Доступ к хранилищу</h3>
    <form class="inline-form" onsubmit="onGrant(event, ${storageId})">
      <input id="grant-email" type="email" placeholder="Email пользователя" required>
      <select id="grant-level"><option value="read">read</option><option value="write">write</option></select>
      <button type="submit">Выдать доступ</button>
    </form>
    ${list.length ? list.map(p => `
      <div class="row">
        <span>${escapeHtml(p.user_email)} <span class="badge">${p.permission}</span></span>
        <span class="actions">
          <button onclick="changeLevel(${storageId}, ${p.id}, '${p.permission === 'read' ? 'write' : 'read'}')">→ ${p.permission === 'read' ? 'write' : 'read'}</button>
          <button class="danger" onclick="revokePermission(${storageId}, ${p.id})">Отозвать</button>
        </span>
      </div>`).join('') : '<p class="muted">Доступ ещё никому не выдан.</p>'}`;
}

async function onGrant(e, storageId) {
    e.preventDefault();
    const body = {email: val('grant-email'), permission: val('grant-level')};
    const res = await apiFetch('/api/storages/' + storageId + '/permissions', jsonOpts('POST', body));
    if (!res.ok) {
        alert((await res.json().catch(() => ({}))).error || 'Ошибка');
        return;
    }
    renderPermissions(storageId);
}

async function changeLevel(storageId, pid, level) {
    const res = await apiFetch('/api/storages/' + storageId + '/permissions/' + pid, jsonOpts('PATCH', {permission: level}));
    if (!res.ok) {
        alert('Ошибка');
        return;
    }
    renderPermissions(storageId);
}

async function revokePermission(storageId, pid) {
    if (!confirm('Отозвать доступ?')) return;
    const res = await apiFetch('/api/storages/' + storageId + '/permissions/' + pid, {method: 'DELETE'});
    if (!res.ok) {
        alert('Ошибка');
        return;
    }
    renderPermissions(storageId);
}

async function showLinks(fileId, storageId) {
    const res = await apiFetch('/api/files/' + fileId + '/share-links');
    const links = res.ok ? await res.json() : [];
    document.getElementById('content').innerHTML = `
    <button onclick="openStorage(${storageId})">← Назад</button>
    <h2>Ссылки на файл</h2>
    <button onclick="createLink(${fileId}, ${storageId})">+ Создать ссылку</button>
    <div style="margin-top:12px">${links.length ? links.map(l => `
      <div class="row">
        <span class="muted">${location.origin}/?share=${l.token}</span>
        <button class="danger" onclick="revokeLink(${l.id}, ${fileId}, ${storageId})">Отозвать</button>
      </div>`).join('') : '<p class="muted">Активных ссылок нет.</p>'}</div>`;
}

async function createLink(fileId, storageId) {
    const res = await apiFetch('/api/files/' + fileId + '/share', {method: 'POST'});
    if (!res.ok) {
        alert('Не удалось создать ссылку');
        return;
    }
    showLinks(fileId, storageId);
}

async function revokeLink(linkId, fileId, storageId) {
    if (!confirm('Отозвать ссылку?')) return;
    const res = await apiFetch('/api/share-links/' + linkId, {method: 'DELETE'});
    if (!res.ok) {
        alert('Ошибка');
        return;
    }
    showLinks(fileId, storageId);
}

async function renderAvailable() {
    document.getElementById('content').innerHTML = `
    <h2>Доступные мне</h2>
    <div class="inline-form">
      <input id="open-token" placeholder="Вставь ссылку или токен">
      <button onclick="openByToken()">Открыть ссылку</button>
    </div>
    <h3>Файлы по ссылке</h3><div id="shared-files">Загрузка…</div>
    <h3>Хранилища по доступу</h3><div id="shared-storages">Загрузка…</div>`;

    const fres = await apiFetch('/api/shared-files');
    const files = fres.ok ? await fres.json() : [];
    document.getElementById('shared-files').innerHTML = files.length ? files.map(f => `
    <div class="row">
      <span>${escapeHtml(f.original_name)} <span class="muted">(${formatBytes(f.size_bytes)}) · от ${escapeHtml(f.shared_by_email)}</span></span>
      <button onclick="downloadFile(${f.id})">Скачать</button>
    </div>`).join('') : '<p class="muted">Нет файлов по ссылкам.</p>';

    const sres = await apiFetch('/api/storages/shared');
    const storages = sres.ok ? await sres.json() : [];
    document.getElementById('shared-storages').innerHTML = storages.length ? storages.map(s => `
    <div class="row">
      <span>${escapeHtml(s.name)} <span class="badge">${s.permission}</span> <span class="muted">владелец ${escapeHtml(s.owner_email)}</span></span>
      <button onclick="openStorage(${s.id})">Открыть</button>
    </div>`).join('') : '<p class="muted">Нет доступных хранилищ.</p>';
}

async function openByToken() {
    const raw = val('open-token').trim();
    if (!raw) return;
    const m = raw.match(/share=([a-f0-9]+)/i);
    const token = m ? m[1] : raw;
    const res = await apiFetch('/api/shared/' + token);
    if (!res.ok) {
        alert('Ссылка недействительна или отозвана');
        return;
    }
    alert('Файл добавлен в «Доступные мне»');
    renderAvailable();
}

async function init() {
    const m = location.search.match(/share=([a-f0-9]+)/i);
    if (m) pendingShareToken = m[1];

    if (getAccess() && await loadMe()) enterApp();
    else showAuthScreen();
}

init();
