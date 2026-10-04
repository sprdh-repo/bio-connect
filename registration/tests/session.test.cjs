const { test } = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const vm = require('node:vm');

// Exercise the shipped request handlers without booting the page or a database.
const source = fs.readFileSync(require('node:path').join(__dirname, '../internal/app/web/app.js'), 'utf8').split('(async()=>{try{await loadConfig();')[0];
function client(status = 401, error = 'sign in required') {
  const storage = new Map(), events = {}, intervals = [];
  let reloads = 0;
  const app = { innerHTML: '' }, notice = { focus() {} };
  const document = {
    cookie: 'bc_csrf=test', hidden: false,
    querySelector: selector => selector === '#app' ? app : notice,
    getElementById: () => null,
    addEventListener: (name, callback) => { events[name] = callback; },
  };
  const context = vm.createContext({
    document, location: { pathname: '/admin', reload: () => { reloads++; } },
    sessionStorage: { getItem: key => storage.get(key), setItem: (key, value) => storage.set(key, value), removeItem: key => storage.delete(key) },
    fetch: async () => ({ status, ok: status < 400, json: async () => ({ error }) }),
    crypto: { randomUUID: () => 'test' }, URLSearchParams, AbortController,
    setTimeout, clearTimeout, setInterval: callback => intervals.push(callback),
    addEventListener: (name, callback) => { events[name] = callback; },
  });
  vm.runInContext(source, context);
  return { run: code => vm.runInContext(code, context), document, events, intervals, app, notice, storage, reloads: () => reloads };
}

test('expired staff API responses return to sign-in once and stop the old handlers', async () => {
  const c = client();
  const request = c.run("api('/admin/summary')");
  await new Promise(resolve => setImmediate(resolve));
  assert.equal(c.reloads(), 1);
  c.run("api('/admin/registrations')");
  await new Promise(resolve => setImmediate(resolve));
  assert.equal(c.reloads(), 1);
  assert.equal(await Promise.race([request.then(() => 'returned', () => 'rejected'), Promise.resolve('pending')]), 'pending');
});

test('initial sign-in checks and invalid login credentials do not reload', async () => {
  const c = client();
  await assert.rejects(c.run("api('/admin/me')"), /sign in required/);
  await assert.rejects(c.run("post('/auth/login', {})"), /sign in required/);
  await assert.rejects(c.run("api('/registrations/test')"), /sign in required/);
  assert.equal(c.reloads(), 0);
});

test('session rechecks and staff uploads also return to sign-in on expiry', async () => {
  for (const request of ["api('/admin/me')", "upload('test', 'logo', {size: 1}, {}, '/api/v1/admin/registrations')"]) {
    const c = client();
    c.run("me = {id: 'staff', role: 'reviewer'}");
    c.run(request);
    await new Promise(resolve => setImmediate(resolve));
    assert.equal(c.reloads(), 1);
  }
});

test('permission and server errors stay visible without signing out', async () => {
  for (const status of [403, 503]) {
    const c = client(status, 'try again');
    await assert.rejects(c.run("api('/admin/registrations')"), /try again/);
    assert.equal(c.reloads(), 0);
  }
});

test('idle expiry and returning to an expired tab end the staff session', () => {
  for (const trigger of ['interval', 'focus', 'visibilitychange']) {
    const c = client();
    c.run("me = {id: 'staff', role: 'reviewer'}");
    c.document.cookie = '';
    if (trigger === 'interval') c.intervals[0]();
    else c.events[trigger]();
    assert.equal(c.reloads(), 1, trigger);
  }
});

test('signed-out and active-session tabs do not reload on idle checks', () => {
  const c = client();
  c.document.cookie = '';
  c.intervals[0]();
  c.run("me = {id: 'staff', role: 'reviewer'}");
  c.document.cookie = 'bc_csrf=test';
  c.intervals[0]();
  assert.equal(c.reloads(), 0);
});

test('sign-in clears staff state and explains an ended session', () => {
  const c = client();
  c.run("me = {id: 'staff', role: 'reviewer'}");
  c.storage.set('bc_staff_session_ended', '1');
  c.run('loginPage()');
  assert.equal(c.run('me'), null);
  assert.match(c.notice.textContent, /session.*(expired|ended)/i);
  assert.equal(c.storage.has('bc_staff_session_ended'), false);
});

test('history navigation after sign-out keeps the sign-in screen', async () => {
  const c = client();
  c.run('loginPage()');
  await c.run('adminRoute()');
  assert.match(c.app.innerHTML, /Staff sign in/);
  assert.equal(c.reloads(), 0);
});
