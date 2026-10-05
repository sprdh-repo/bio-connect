'use strict';
// Mobile app editor at /admin?view=mobile. Uses the staff session, the
// CSRF-aware api() helper and esc() from app.js. The whole app document is
// held in memory and saved in one request, guarded by its revision.

const mobileSections = [
  ['event', 'Event'], ['menus', 'Menus'], ['text', 'Headings & text'], ['sessions', 'Sessions'],
  ['activities', 'Activities'], ['faqs', 'FAQs'], ['venue', 'Venue & help'], ['speakers', 'Speakers'],
  ['sponsors', 'Sponsors & partners'], ['leadership', 'Leadership'], ['launch', 'Product launch'], ['themes', 'Themes & highlights']
];
const mobileDestinations = [
  ['sessions', 'Sessions'], ['speakers', 'Speakers'], ['venue', 'Venue & directions'], ['activities', 'Activities'],
  ['faqs', 'FAQs'], ['exhibitors', 'Exhibitors'], ['my_passes', 'My passes'], ['registration', 'Registration & passes'],
  ['brochure', 'Event brochure'], ['product_launch', 'Product launch'], ['sponsors', 'Sponsors'], ['leadership', 'Leadership'],
  ['explore', 'Explore Bio Connect'], ['privacy', 'Privacy policy'], ['link', 'Custom link (web, email or phone)']
];
const mobileMenus = [
  ['tabs', 'Bottom tabs', 'Home and Guide are always shown. Hide Sessions or Speakers, or rename them.'],
  ['home_shortcuts', 'Home - shortcut tiles', 'The grid of tiles on the home screen.'],
  ['home_links', 'Home - links', 'The list below the home announcement.'],
  ['guide', 'Guide tab', 'Every entry on the Guide tab, in order.']
];
// Keys and the app's built-in wording. Leave a field blank to keep the default.
const mobileCopy = [
  ['Home', [['home.eyebrow', 'YOUR CONCLAVE COMPANION'], ['home.title', 'Make the most\nof Bio Connect.'], ['home.button', 'View sessions'], ['home.glance', 'Your event, at a glance.'],
    ['home.notice_title', 'Announcement title (blank shows the session count)'], ['home.notice_message', 'Announcement message']]],
  ['Sessions', [['sessions.eyebrow', 'THE PROGRAMME'], ['sessions.title', 'Find your next\nconversation.'], ['sessions.intro', 'Sessions, speakers and places to be. All times are in IST.']]],
  ['Speakers', [['speakers.eyebrow', 'CONCLAVE SPEAKERS'], ['speakers.title', 'The voices\ntaking the stage.']]],
  ['Guide tab', [['guide.eyebrow', 'YOUR EVENT GUIDE'], ['guide.title', 'Everything in\none place.']]],
  ['Venue & help', [['venue.eyebrow', 'GETTING HERE'], ['venue.directions', 'Open directions'], ['help.title', 'Need a hand?']]],
  ['Activities', [['activities.eyebrow', 'BEYOND THE SESSIONS'], ['activities.title', 'Discover. Meet. Connect.']]],
  ['FAQs', [['faqs.eyebrow', 'A LITTLE HELP'], ['faqs.title', 'Good to know.']]],
  ['Exhibitors', [['exhibitors.eyebrow', 'THE EXPO'], ['exhibitors.title', 'Meet your next\ncollaborator.']]],
  ['Sponsors', [['sponsors.eyebrow', 'OUR SPONSORS'], ['sponsors.title', 'Shared purpose.\nGreater possibilities.'], ['sponsors.cta_eyebrow', 'BECOME A SPONSOR'], ['sponsors.cta_title', 'Help make the next\nconnection possible.'], ['sponsors.cta_button', 'Enquire about sponsorship']]],
  ['Leadership', [['leadership.eyebrow', 'GOVERNMENT OF KERALA'], ['leadership.title', 'The people convening\nthe conclave.'], ['leadership.cta_eyebrow', 'WORKING WITH THE ORGANISERS'], ['leadership.cta_title', 'Partner with\nBio Connect 4.0.'], ['leadership.cta_button', 'Contact the event team']]],
  ['Explore', [['explore.eyebrow', 'BIO CONNECT 4.0'], ['explore.title', 'Explore the ideas\nshaping tomorrow.'], ['explore.intro', 'Five themes drive the conversations, showcases and connections at Bio Connect 4.0.'], ['explore.programme_title', 'Built for connection.'], ['explore.programme_intro', 'The event brings science, enterprise and policy together through:']]],
  ['Registration', [['registration.title', 'Three ways\nto take part.'], ['registration.intro', 'Register for a delegate pass without leaving the app. Exhibition bookings continue on the secure event portal.'], ['registration.sponsorship', 'Sponsorships are arranged with the event team.']]],
  ['Plan your visit panel', [['visit.eyebrow', 'PLAN YOUR VISIT'], ['visit.title', 'See you in Thiruvananthapuram.']]]
];

function mobilePath(root, path) { return path.split('.').reduce((v, k) => v?.[k], root); }
function mobileSet(root, path, value) {
  const keys = path.split('.'), last = keys.pop();
  keys.reduce((v, k) => v[k], root)[last] = value;
}
function mobileIST(value) {
  return value ? new Date(new Date(value).getTime() + 330 * 60000).toISOString().slice(0, 16) : '';
}
function mobileSlug(name, taken) {
  const base = name.toLowerCase().normalize('NFKD').replace(/[^a-z0-9]+/g, '-').replace(/^-+|-+$/g, '').slice(0, 70) || 'speaker';
  let id = base, n = 2;
  while (taken.has(id)) id = `${base}-${n++}`;
  return id;
}

let mobileCleanup = null;
async function mobileContentPage() {
  mobileCleanup?.();
  let state = await api('/admin/mobile-content'), dirty = false;
  let section = sessionStorage.getItem('bc_mobile_section') || 'event';
  if (!mobileSections.some(([k]) => k === section)) section = 'event';

  // One input per field. data-path addresses the value inside state.
  let fieldId = 0;
  function input(path, label, type = 'text', opts = {}) {
    const raw = mobilePath(state, path), value = type === 'datetime-local' ? mobileIST(raw) : type === 'lines' ? (raw || []).join('\n') : raw ?? '';
    const id = `mc-f${++fieldId}`, name = path.split('.').pop();
    const attrs = `id="${id}" data-path="${esc(path)}" data-type="${type}" ${opts.required ? 'required' : ''} ${opts.placeholder ? `placeholder="${esc(opts.placeholder)}"` : ''}`;
    const control = type === 'textarea' || type === 'lines'
      ? `<textarea ${attrs} rows="${opts.rows || 3}" maxlength="6000">${esc(value)}</textarea>`
      : type === 'select'
        ? `<select ${attrs}>${opts.options.map(([v, l]) => `<option value="${esc(v)}" ${v === value ? 'selected' : ''}>${esc(l)}</option>`).join('')}</select>`
        : `<input ${attrs} type="${type}" value="${esc(value)}" maxlength="6000">`;
    // The hide toggle sits beside the field's label, not inside it, so clicking the label focuses the field.
    const hide = opts.hide ? `<label class="mc-hide"><input type="checkbox" data-hide="${esc(opts.hide)}" data-field="${esc(name)}" ${(mobilePath(state, opts.hide) || []).includes(name) ? 'checked' : ''}> Hide in app</label>` : '';
    return `<div class="mc-field ${opts.full || type === 'textarea' || type === 'lines' ? 'full' : ''}"><div class="mc-label"><label for="${id}">${esc(label)}</label>${hide}</div>${control}${opts.help ? `<p class="help">${esc(opts.help)}</p>` : ''}</div>`;
  }
  function shown(path, label = 'Show in app') {
    return `<label class="check"><input type="checkbox" data-path="${esc(path)}" data-type="checkbox" ${mobilePath(state, path) ? 'checked' : ''}> ${esc(label)}</label>`;
  }
  // A reorderable list of records; fields(path) renders one record's inputs.
  function list(path, { title, fields, blank, noun, publish = true }) {
    const items = mobilePath(state, path) || [];
    return `<div class="mc-list">${items.map((item, i) => `<fieldset class="mc-item ${publish && !item.published ? 'mc-off' : ''}"><legend>${esc(title(item) || `New ${noun}`)}</legend>
      <div class="fields">${fields(`${path}.${i}`, item)}</div>
      <div class="mc-row">${publish ? shown(`${path}.${i}.published`) : '<span></span>'}<span class="mc-tools">
        <button type="button" class="quiet" data-move="${esc(path)}" data-index="${i}" data-step="-1" ${i ? '' : 'disabled'} aria-label="Move up">↑</button>
        <button type="button" class="quiet" data-move="${esc(path)}" data-index="${i}" data-step="1" ${i < items.length - 1 ? '' : 'disabled'} aria-label="Move down">↓</button>
        <button type="button" class="quiet" data-remove="${esc(path)}" data-index="${i}">Remove</button></span></div></fieldset>`).join('') || `<p class="muted">No ${noun} entries yet.</p>`}
      <button type="button" class="secondary" data-add="${esc(path)}" data-noun="${esc(noun)}">Add ${esc(noun)}</button></div>`;
  }
  const blanks = {};
  function listOf(path, spec) { blanks[path] = spec.blank; return list(path, spec); }

  const views = {
    event: () => `<section class="card"><h2>Event details</h2><p class="help">Tick “Hide in app” to keep a value here but withhold it from the app. Empty fields are hidden in the app automatically.</p><div class="fields">
      ${input('content.event.title', 'Event name', 'text', { required: true })}${input('content.event.tagline', 'Tagline', 'text', { hide: 'content.event.hidden' })}
      ${input('content.event.start_date', 'First day', 'date', { required: true })}${input('content.event.end_date', 'Last day', 'date', { required: true })}
      ${input('content.event.venue', 'Venue name', 'text', { hide: 'content.event.hidden' })}${input('content.event.city', 'City', 'text', { hide: 'content.event.hidden' })}
      ${input('content.event.hero_title', 'Hero title', 'textarea', { hide: 'content.event.hidden', rows: 2 })}
      ${input('content.event.description', 'Description', 'textarea', { hide: 'content.event.hidden' })}
      ${input('content.event.brochure_url', 'Brochure link (HTTPS)', 'url', { hide: 'content.event.hidden' })}${input('content.event.privacy_url', 'Privacy policy link (HTTPS)', 'url', { hide: 'content.event.hidden' })}
      ${input('content.event.sponsorship_email', 'Sponsorship and organiser email', 'email', { hide: 'content.event.hidden', help: 'Used by the “Enquire” buttons. Hidden buttons disappear.' })}
    </div></section>`,
    menus: () => mobileMenus.map(([name, label, help]) => {
      if (!state.content.menus[name]) state.content.menus[name] = [];
      const keys = name === 'tabs' ? mobileDestinations.filter(([k]) => k === 'sessions' || k === 'speakers') : mobileDestinations;
      return `<section class="card"><h2>${esc(label)}</h2><p class="help">${esc(help)} A blank title uses the app’s default label. Entries for empty sections (no sessions, no sponsors…) are hidden automatically.</p>${listOf(`content.menus.${name}`, {
        noun: 'entry', blank: () => ({ key: name === 'tabs' ? 'sessions' : 'link', title: '', subtitle: '', url: '', published: false }),
        title: item => item.title || keys.find(([k]) => k === item.key)?.[1],
        fields: (p, item) => `${input(`${p}.key`, 'Opens', 'select', { options: keys })}${input(`${p}.title`, name === 'tabs' ? 'Tab label' : 'Title')}${name === 'tabs' ? '' : input(`${p}.subtitle`, 'Subtitle')}${item.key === 'link' ? input(`${p}.url`, 'Address (https://, mailto: or tel:)', 'text', { required: true }) : ''}`
      })}</section>`;
    }).join(''),
    text: () => `<section class="card"><h2>Headings & text</h2><p class="help">Override the app’s built-in headings. Leave a field blank to keep the default shown in grey. Use a new line to break a title.</p></section>` +
      mobileCopy.map(([group, keys]) => `<section class="card"><h3>${esc(group)}</h3><div class="fields">${keys.map(([key, fallback]) =>
        `<label class="full"><span class="mc-label">${esc(key)}</span><textarea data-copy="${esc(key)}" rows="${fallback.length > 60 ? 2 : 1}" maxlength="600" placeholder="${esc(fallback)}">${esc(state.content.copy[key] || '')}</textarea></label>`).join('')}</div></section>`).join(''),
    sessions: () => `<section class="card"><h2>Sessions</h2><p class="help">Times are India time (IST). Leave both blank if unconfirmed; blank details are left out in the app.</p>${listOf('guide.sessions', {
      noun: 'session', blank: () => ({ id: uuid(), title: '', description: '', starts_at: '', ends_at: '', location: '', speakers: '', published: false }), title: i => i.title,
      fields: p => `${input(`${p}.title`, 'Session title', 'text', { required: true, full: true })}${input(`${p}.starts_at`, 'Start (IST)', 'datetime-local')}${input(`${p}.ends_at`, 'End (IST)', 'datetime-local')}${input(`${p}.location`, 'Hall / location')}${input(`${p}.speakers`, 'Speakers')}${input(`${p}.description`, 'Description', 'textarea')}`
    })}</section>`,
    activities: () => `<section class="card"><h2>Activities</h2>${listOf('guide.activities', {
      noun: 'activity', blank: () => ({ id: uuid(), title: '', description: '', schedule: '', location: '', published: false }), title: i => i.title,
      fields: p => `${input(`${p}.title`, 'Activity title', 'text', { required: true, full: true })}${input(`${p}.schedule`, 'Schedule')}${input(`${p}.location`, 'Location')}${input(`${p}.description`, 'Description', 'textarea')}`
    })}</section>`,
    faqs: () => `<section class="card"><h2>FAQs</h2>${listOf('guide.faqs', {
      noun: 'FAQ', blank: () => ({ id: uuid(), question: '', answer: '', published: false }), title: i => i.question,
      fields: p => `${input(`${p}.question`, 'Question', 'text', { required: true, full: true })}${input(`${p}.answer`, 'Answer', 'textarea')}`
    })}</section>`,
    venue: () => `<section class="card"><h2>Venue, arrival and help desk</h2><p class="help">Each line appears in the app only when filled in and not hidden.</p>${shown('guide.venue.published', 'Show venue and help details in the app')}<div class="fields">
      ${input('guide.venue.address', 'Full address', 'textarea', { hide: 'guide.venue.hidden', rows: 2 })}
      ${input('guide.venue.arrival', 'Arrival, parking and check-in', 'textarea', { hide: 'guide.venue.hidden' })}
      ${input('guide.venue.accessibility', 'Accessibility', 'textarea', { hide: 'guide.venue.hidden' })}
      ${input('guide.venue.floor_plan_url', 'Floor plan link (HTTPS)', 'url', { hide: 'guide.venue.hidden', full: true })}
      ${input('guide.venue.help_phone', 'Help desk phone', 'tel', { hide: 'guide.venue.hidden' })}${input('guide.venue.help_whatsapp', 'Help desk WhatsApp', 'tel', { hide: 'guide.venue.hidden', help: 'With country code, e.g. +91 98470 00000' })}
      ${input('guide.venue.help_email', 'Help desk email', 'email', { hide: 'guide.venue.hidden' })}
    </div></section>`,
    speakers: () => `<section class="card"><h2>Speakers</h2><p class="help">Shown in this order in the app.</p>${listOf('speakers', {
      noun: 'speaker', blank: () => ({ id: '', name: '', role: '', organization: '', image_url: '', linkedin: '', published: false }), title: i => i.name,
      fields: p => `${input(`${p}.name`, 'Name', 'text', { required: true })}${input(`${p}.role`, 'Role')}${input(`${p}.organization`, 'Organisation', 'text', { full: true })}${input(`${p}.image_url`, 'Portrait URL (HTTPS)', 'url')}${input(`${p}.linkedin`, 'LinkedIn URL', 'url')}`
    })}</section>`,
    sponsors: () => `<section class="card"><h2>Sponsors</h2><div class="fields">${input('content.sponsors_intro', 'Introduction', 'textarea')}</div></section><section class="card"><h3>Sponsor list</h3>${listOf('content.sponsors', {
      noun: 'sponsor', blank: () => ({ name: '', category: '', description: '', logo_url: '', website_url: '', published: false }), title: i => i.name,
      fields: p => `${input(`${p}.name`, 'Name', 'text', { required: true })}${input(`${p}.category`, 'Category · city')}${input(`${p}.description`, 'Description', 'textarea')}${input(`${p}.logo_url`, 'Logo URL (HTTPS)', 'url')}${input(`${p}.website_url`, 'Website (HTTPS)', 'url')}`
    })}</section><section class="card"><h3>Ecosystem partners</h3>${listOf('content.ecosystem_partners', {
      noun: 'partner', blank: () => ({ name: '', logo_url: '', website_url: '', published: false }), title: i => i.name,
      fields: p => `${input(`${p}.name`, 'Name', 'text', { required: true, full: true })}${input(`${p}.logo_url`, 'Logo URL (HTTPS)', 'url')}${input(`${p}.website_url`, 'Website (HTTPS)', 'url')}`
    })}</section>`,
    leadership: () => {
      const l = state.content.leadership;
      if (!l.convened_by) l.convened_by = { name: '', note: '' };
      return `<section class="card"><h2>Leadership</h2><div class="fields">${input('content.leadership.intro', 'Introduction', 'textarea', { hide: 'content.leadership.hidden' })}
        ${input('content.leadership.convened_by.name', 'Convened by', 'text', { full: true })}${input('content.leadership.convened_by.note', 'Convenor note', 'textarea', { rows: 2 })}
        <label class="check full"><input type="checkbox" data-hide="content.leadership.hidden" data-field="convened_by" ${(l.hidden || []).includes('convened_by') ? 'checked' : ''}> Hide “Convened by” in the app</label></div></section>
      <section class="card"><h3>State leadership</h3>${listOf('content.leadership.people', {
        noun: 'leader', blank: () => ({ name: '', role: '', badge: '', image_url: '', image_credit: '', image_credit_url: '', published: false }), title: i => i.name,
        fields: p => `${input(`${p}.name`, 'Name', 'text', { required: true })}${input(`${p}.badge`, 'Badge')}${input(`${p}.role`, 'Role', 'text', { full: true })}${input(`${p}.image_url`, 'Portrait URL (HTTPS)', 'url')}${input(`${p}.image_credit_url`, 'Portrait credit link', 'url')}${input(`${p}.image_credit`, 'Portrait credit', 'text', { full: true })}`
      })}</section>
      <section class="card"><h3>Advisory committee</h3><div class="fields">${input('content.leadership.committee.title', 'Title')}${input('content.leadership.committee.order_note', 'Government order note')}${input('content.leadership.advisory_note', 'Committee note', 'textarea', { hide: 'content.leadership.hidden' })}</div>
      ${listOf('content.leadership.committee.members', {
        noun: 'member', blank: () => ({ role: 'Member', name: '', organization: '', published: false }), title: i => i.name && `${i.role} - ${i.name}`,
        fields: p => `${input(`${p}.role`, 'Role (Chairman, Convenor, Member…)')}${input(`${p}.name`, 'Name', 'text', { required: true })}${input(`${p}.organization`, 'Organisation', 'text', { full: true })}`
      })}</section>`;
    },
    launch: () => `<section class="card"><h2>Product launch</h2><p class="help">Hide the whole page from the Menus tab. Empty or hidden lines are left out.</p><div class="fields">
      ${input('content.product_launch.eyebrow', 'Eyebrow', 'text', { hide: 'content.product_launch.hidden' })}${input('content.product_launch.deadline', 'Deadline', 'text', { hide: 'content.product_launch.hidden' })}
      ${input('content.product_launch.title', 'Title', 'text', { hide: 'content.product_launch.hidden', full: true })}
      ${input('content.product_launch.description', 'Description', 'textarea', { hide: 'content.product_launch.hidden' })}
      ${input('content.product_launch.focus_areas', 'Focus areas (one per line)', 'lines', { hide: 'content.product_launch.hidden' })}
      ${input('content.product_launch.apply_url', 'Apply link (HTTPS)', 'url', { hide: 'content.product_launch.hidden' })}${input('content.product_launch.apply_label', 'Apply button label')}
    </div></section><section class="card"><h3>Who can apply</h3>${listOf('content.product_launch.eligibility', {
      noun: 'eligibility', publish: false, blank: () => ({ title: '', description: '' }), title: i => i.title,
      fields: p => `${input(`${p}.title`, 'Title', 'text', { required: true, full: true })}${input(`${p}.description`, 'Description', 'textarea', { rows: 2 })}`
    })}</section>`,
    themes: () => `<section class="card"><h2>Programme highlights</h2><div class="fields">${input('content.programme_highlights', 'Highlights (one per line)', 'lines', { rows: 8 })}</div></section><section class="card"><h3>Themes</h3>${listOf('content.themes', {
      noun: 'theme', blank: () => ({ title: '', description: '', image: '', published: false }), title: i => i.title,
      fields: p => `${input(`${p}.title`, 'Title', 'text', { required: true })}${input(`${p}.image`, 'Image (bundled path or HTTPS URL)', 'text', { required: true })}${input(`${p}.description`, 'Description', 'textarea', { rows: 2 })}`
    })}</section>`
  };

  function render() {
    const y = scrollY;
    fieldId = 0;
    app.innerHTML = `<div class="admin-page mc-page"><header class="admin-heading"><div><p class="admin-kicker">Staff console</p><h1>Mobile app</h1><p class="admin-intro">Everything the attendee app shows. Changes reach phones on their next refresh after you save.</p></div>
      <div class="admin-header-actions"><a class="button secondary" href="/api/v1/public/app-content" target="_blank" rel="noopener">View live data</a><button type="button" id="mc-back" class="quiet">Back to administration</button></div></header>
      <nav class="mc-tabs" aria-label="Mobile app sections">${mobileSections.map(([k, l]) => `<button type="button" data-section="${k}" class="${k === section ? '' : 'secondary'}" ${k === section ? 'aria-current="page"' : ''}>${esc(l)}</button>`).join('')}</nav>
      <form id="mc-form" novalidate>${views[section]()}<div class="mc-savebar"><span id="mc-status" class="muted">${dirty ? 'Unsaved changes' : `Saved · revision ${state.revision}`}</span><button type="button" id="mc-reload" class="secondary">Discard & reload</button><button type="submit">Save all changes</button></div></form></div>`;
    scrollTo({ top: y });
  }
  function changed() {
    dirty = true;
    const status = document.getElementById('mc-status');
    if (status) status.textContent = 'Unsaved changes';
  }
  const leaving = e => { if (dirty) { e.preventDefault(); e.returnValue = ''; } };
  addEventListener('beforeunload', leaving);

  const leave = () => {
    removeEventListener('beforeunload', leaving);
    app.oninput = app.onchange = app.onclick = app.onsubmit = null;
    mobileCleanup = null;
  };
  mobileCleanup = leave;
  // Back/forward can replace this screen without our buttons; detach then.
  const active = () => document.getElementById('mc-form') || (leave(), false);
  app.oninput = app.onchange = e => {
    if (!active()) return;
    const el = e.target;
    if (el.dataset.path) {
      const t = el.dataset.type;
      let v = t === 'checkbox' ? el.checked : el.value;
      if (t === 'datetime-local') v = v ? `${v}:00+05:30` : '';
      if (t === 'lines') v = v.split('\n').map(s => s.trim()).filter(Boolean);
      mobileSet(state, el.dataset.path, v);
      // Picking a custom link reveals its address field; ticks restyle the card.
      if (e.type === 'change' && (el.dataset.path.endsWith('.key') || t === 'checkbox')) render();
      changed();
    } else if (el.dataset.hide) {
      const list = mobilePath(state, el.dataset.hide) || [], name = el.dataset.field;
      mobileSet(state, el.dataset.hide, el.checked ? [...new Set([...list, name])] : list.filter(n => n !== name));
      changed();
    } else if (el.dataset.copy) {
      if (el.value.trim()) state.content.copy[el.dataset.copy] = el.value; else delete state.content.copy[el.dataset.copy];
      changed();
    }
  };
  // The form is re-rendered on every structural change, so all controls are
  // handled by delegation from #app.
  app.onclick = async e => {
    const b = e.target.closest('button');
    if (!b || !active()) return;
    try {
      if (b.id === 'mc-back') {
        if (dirty && !confirm('Leave without saving your changes?')) return;
        leave();
        history.pushState(null, '', '/admin');
        await adminRoute();
      } else if (b.id === 'mc-reload') {
        if (dirty && !confirm('Discard unsaved changes and reload the saved content?')) return;
        message('');
        state = await api('/admin/mobile-content'); dirty = false; render();
      } else if (b.dataset.section) {
        section = b.dataset.section;
        sessionStorage.setItem('bc_mobile_section', section);
        render();
        scrollTo({ top: 0 });
      } else if (b.dataset.add) {
        mobilePath(state, b.dataset.add).push(blanks[b.dataset.add]());
        changed(); render();
      } else if (b.dataset.move) {
        const items = mobilePath(state, b.dataset.move), i = +b.dataset.index, j = i + +b.dataset.step;
        [items[i], items[j]] = [items[j], items[i]];
        changed(); render();
      } else if (b.dataset.remove) {
        if (!confirm('Remove this entry? It is deleted when you save.')) return;
        mobilePath(state, b.dataset.remove).splice(+b.dataset.index, 1);
        changed(); render();
      }
    } catch (err) { message(err.message, true); }
  };
  app.onsubmit = async e => {
    if (e.target.id !== 'mc-form' || !active()) return;
    e.preventDefault();
    message('');
    const taken = new Set(state.speakers.map(s => s.id).filter(Boolean));
    for (const s of state.speakers) if (!s.id) { s.id = mobileSlug(s.name, taken); taken.add(s.id); }
    try {
      await busy(e.submitter || app.querySelector('[type=submit]'), async () => {
        state = await api('/admin/mobile-content', { method: 'PUT', body: JSON.stringify(state) });
      });
      dirty = false; render();
      message(`Saved. The app shows these changes on its next refresh (revision ${state.revision}).`);
    } catch (err) { message(err.message, true); }
  };
  render();
}
