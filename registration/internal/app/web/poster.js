'use strict';
// Social poster studio for the staff console.
//
// One renderer, renderPoster(), draws to a canvas at true output resolution. The
// builder preview, the poster editor and the exported PNG all call it, so what a
// staff member drags a portrait around in is byte-for-byte what they download.
//
// Written plainly rather than in app.js's one-line-per-function style: this is
// canvas geometry, and the arithmetic has to stay legible.
//
// It shares app.js's globals (app, esc, api, post, bind, busy, message, csrf).
// page.html loads app.js first.

const PS = {
  cream: '#f3f1e9', forest: '#0b3329', lime: '#b9dc72', gold: '#e4ad54', ink: '#10201b',
  // The portrait duotone from scripts/portrait.py, so a poster portrait and the
  // one on speakers.html read as the same set.
  duotoneStops: [[0.0, [5, 28, 22]], [0.55, [118, 150, 122]], [1.0, [232, 238, 216]]],
  // The spec stores a role, not a face, so a template survives a face being
  // swapped. Every entry needs an @font-face in web/fonts/fonts.css and a
  // matching case in validateSpec.
  fonts: {
    display: { name: 'Manrope', label: 'Manrope (display)' },
    body: { name: 'DM Sans', label: 'DM Sans (body)' },
    serif: { name: 'Fraunces', label: 'Fraunces (serif)' },
    condensed: { name: 'Archivo Narrow', label: 'Archivo Narrow (condensed)' },
  },
  sizes: { '4x5': [1080, 1350], '1x1': [1080, 1080], '9x16': [1080, 1920] },
  sizeLabels: { '4x5': 'Feed 4:5', '1x1': 'Square 1:1', '9x16': 'Story 9:16' },
  minFont: 6,
};

/* ------------------------------------------------------------------ images */

// An unknown role falls back to the body face rather than a system font, so a
// template saved against a face that was later removed still renders.
const fontName = role => (PS.fonts[role] || PS.fonts.body).name;

const imageCache = new Map();

function loadImage(src) {
  if (imageCache.has(src)) return imageCache.get(src);
  const p = new Promise((resolve, reject) => {
    const img = new Image();
    img.onload = () => resolve(img);
    img.onerror = () => reject(Error('Could not load an image for this poster.'));
    // Every source is same-origin or a data: URL, which is what keeps the canvas
    // untainted and toDataURL working. Never point this at S3 directly.
    img.src = src;
  });
  imageCache.set(src, p);
  return p;
}

function layerSrc(layer, values) {
  if (layer.type === 'art') {
    return layer.asset_id ? '/api/v1/admin/posters/assets/' + layer.asset_id
      : '/static/logos/' + layer.builtin + '.png';
  }
  if (layer.type === 'photo') {
    const v = values[layer.key];
    if (!v) return '';
    return v.data_url || (v.asset_id ? '/api/v1/admin/posters/assets/' + v.asset_id : '');
  }
  if (layer.type === 'qr') {
    const url = values[layer.key];
    return url ? '/api/v1/admin/posters/qr?data=' + encodeURIComponent(url) : '';
  }
  return '';
}

async function preload(template, values) {
  const srcs = template.spec.layers.map(l => layerSrc(l, values)).filter(Boolean);
  await Promise.all(srcs.map(src => loadImage(src).catch(() => null)));
}

// Canvas does not trigger webfont loading, and an unloaded face falls back to a
// system font silently. Ask for every size a template uses before the first draw.
async function loadFonts(templates) {
  const wanted = new Set();
  for (const t of templates) {
    for (const l of t.spec.layers) {
      if (l.type !== 'text') continue;
      const family = fontName(l.font);
      for (let size = PS.minFont; size <= Math.ceil(l.size); size += 6) {
        wanted.add(`${l.weight} ${size}px "${family}"`);
      }
      wanted.add(`${l.weight} ${Math.ceil(l.size)}px "${family}"`);
    }
  }
  await Promise.all([...wanted].map(f => document.fonts.load(f).catch(() => null)));
  await document.fonts.ready;
}

/* -------------------------------------------------------------------- text */

function fontString(layer, size) {
  const generic = layer.font === 'serif' ? 'serif' : 'sans-serif';
  return `${layer.weight} ${size}px "${fontName(layer.font)}", ${generic}`;
}

// Tracking is applied by hand rather than through ctx.letterSpacing so the
// measurement and the draw can never disagree. Untracked text goes through
// fillText so it keeps its kerning.
function measureText(ctx, text, tracking) {
  if (!tracking) return ctx.measureText(text).width;
  let w = 0;
  for (const ch of text) w += ctx.measureText(ch).width + tracking;
  return w - tracking;
}

function fillTracked(ctx, text, x, y, tracking) {
  if (!tracking) { ctx.fillText(text, x, y); return; }
  let cursor = x;
  for (const ch of text) {
    ctx.fillText(ch, cursor, y);
    cursor += ctx.measureText(ch).width + tracking;
  }
}

function wrapText(ctx, text, maxWidth, tracking) {
  const lines = [];
  for (const para of String(text).split('\n')) {
    let line = '';
    for (const word of para.split(/\s+/).filter(Boolean)) {
      const next = line ? line + ' ' + word : word;
      if (line && measureText(ctx, next, tracking) > maxWidth) { lines.push(line); line = word; }
      else line = next;
    }
    lines.push(line);
  }
  return lines;
}

// The same rule the PDF passes use in drawPassText: wrap onto more lines first,
// then step the size down, and never ellipsise. A long speaker name gets smaller
// rather than cut.
function fitText(ctx, layer, text) {
  let size = layer.size;
  for (;;) {
    ctx.font = fontString(layer, size);
    const tracking = (layer.tracking || 0) * size;
    const lines = wrapText(ctx, text, layer.w, tracking);
    const lineHeight = size * (layer.line_height || 1.2);
    const height = (lines.length - 1) * lineHeight + size;
    const fits = height <= layer.h && lines.every(l => measureText(ctx, l, tracking) <= layer.w);
    if (fits || size <= PS.minFont || layer.autofit === false) {
      return { size, lines, lineHeight, tracking };
    }
    size -= 1;
  }
}

function drawText(ctx, layer, values, defaults) {
  let text = values[layer.key];
  if (text === undefined || text === null || text === '') text = defaults[layer.key] || '';
  text = String(text);
  if (layer.transform === 'upper') text = text.toUpperCase();
  if (!text.trim()) return;

  const fit = fitText(ctx, layer, text);
  ctx.save();
  ctx.font = fontString(layer, fit.size);
  ctx.fillStyle = layer.color;
  ctx.textBaseline = 'top';
  fit.lines.forEach((line, i) => {
    const width = measureText(ctx, line, fit.tracking);
    let x = layer.x;
    if (layer.align === 'center') x = layer.x + (layer.w - width) / 2;
    else if (layer.align === 'right') x = layer.x + layer.w - width;
    fillTracked(ctx, line, x, layer.y + i * fit.lineHeight, fit.tracking);
  });
  ctx.restore();
}

/* ------------------------------------------------------------------ photos */

function duotoneLUT() {
  const lut = new Uint8ClampedArray(256 * 3);
  for (let i = 0; i < 256; i++) {
    const t = i / 255;
    let [a, b] = [PS.duotoneStops[0], PS.duotoneStops[PS.duotoneStops.length - 1]];
    for (let s = 0; s < PS.duotoneStops.length - 1; s++) {
      if (t >= PS.duotoneStops[s][0] && t <= PS.duotoneStops[s + 1][0]) {
        a = PS.duotoneStops[s]; b = PS.duotoneStops[s + 1]; break;
      }
    }
    const span = b[0] - a[0] || 1;
    const k = (t - a[0]) / span;
    for (let c = 0; c < 3; c++) lut[i * 3 + c] = a[1][c] + (b[1][c] - a[1][c]) * k;
  }
  return lut;
}
const DUOTONE = duotoneLUT();

// A 1st/99th-percentile contrast stretch then the three-stop gradient, matching
// scripts/portrait.py so portraits made either way sit together.
function applyDuotone(ctx, w, h) {
  const frame = ctx.getImageData(0, 0, w, h);
  const px = frame.data;
  const histogram = new Uint32Array(256);
  let opaque = 0;
  for (let i = 0; i < px.length; i += 4) {
    if (px[i + 3] < 8) continue;
    const luma = (px[i] * 299 + px[i + 1] * 587 + px[i + 2] * 114) / 1000 | 0;
    histogram[luma]++; opaque++;
  }
  if (!opaque) return;
  const lowTarget = opaque * 0.01, highTarget = opaque * 0.99;
  let low = 0, high = 255, running = 0;
  for (let i = 0; i < 256; i++) { running += histogram[i]; if (running >= lowTarget) { low = i; break; } }
  running = 0;
  for (let i = 0; i < 256; i++) { running += histogram[i]; if (running >= highTarget) { high = i; break; } }
  const span = Math.max(1, high - low);
  for (let i = 0; i < px.length; i += 4) {
    if (px[i + 3] < 8) continue;
    const luma = (px[i] * 299 + px[i + 1] * 587 + px[i + 2] * 114) / 1000;
    const stretched = Math.min(255, Math.max(0, Math.round((luma - low) / span * 255)));
    px[i] = DUOTONE[stretched * 3];
    px[i + 1] = DUOTONE[stretched * 3 + 1];
    px[i + 2] = DUOTONE[stretched * 3 + 2];
  }
  ctx.putImageData(frame, 0, 0);
}

function roundedPath(ctx, x, y, w, h, r) {
  const radius = Math.min(r, w / 2, h / 2);
  ctx.beginPath();
  ctx.moveTo(x + radius, y);
  ctx.arcTo(x + w, y, x + w, y + h, radius);
  ctx.arcTo(x + w, y + h, x, y + h, radius);
  ctx.arcTo(x, y + h, x, y, radius);
  ctx.arcTo(x, y, x + w, y, radius);
  ctx.closePath();
}

// The clip a photo slot is cut to. Returns false when the slot is a plain
// rectangle and no clipping is needed.
//
// An absent `shape` is the pre-shape form: the seeded session-announce slots
// are rects carrying radius = w/2, which is how a circle was expressed before
// there was a picker. They have to keep drawing as circles.
function photoPath(ctx, layer) {
  const { x, y, w, h } = layer;
  const shape = layer.shape || (layer.radius ? 'rounded' : 'rect');
  if (shape === 'rect') return false;
  if (shape === 'circle') {
    ctx.beginPath();
    ctx.ellipse(x + w / 2, y + h / 2, w / 2, h / 2, 0, 0, Math.PI * 2);
    return true;
  }
  if (shape === 'arch') {
    // A semicircular top on straight sides, the shape the designer's speaker
    // poster uses. The arc flattens into an ellipse when the box is too short
    // to fit a true half-circle.
    const r = Math.min(w / 2, h);
    ctx.beginPath();
    ctx.moveTo(x, y + h);
    ctx.lineTo(x, y + r);
    // Canvas angles run clockwise with y down, so PI -> 2PI is the top half.
    ctx.ellipse(x + w / 2, y + r, w / 2, r, 0, Math.PI, Math.PI * 2);
    ctx.lineTo(x + w, y + h);
    ctx.closePath();
    return true;
  }
  roundedPath(ctx, x, y, w, h, layer.radius || 0);
  return true;
}

function drawPhoto(ctx, layer, values, transform) {
  const src = layerSrc(layer, values);
  const img = src && imageCache.get(src) && imageCache.get(src).settled;
  if (!img) return;

  // Compose into an offscreen canvas at slot size so the duotone only ever sees
  // the portrait, not whatever artwork sits behind the slot.
  const slot = document.createElement('canvas');
  slot.width = Math.max(1, Math.round(layer.w));
  slot.height = Math.max(1, Math.round(layer.h));
  const sctx = slot.getContext('2d');

  const t = transform || { x: 0, y: 0, scale: 1 };
  const cover = layer.fit !== 'contain';
  const ratio = cover
    ? Math.max(slot.width / img.naturalWidth, slot.height / img.naturalHeight)
    : Math.min(slot.width / img.naturalWidth, slot.height / img.naturalHeight);
  const scale = ratio * (t.scale || 1);
  const dw = img.naturalWidth * scale, dh = img.naturalHeight * scale;
  sctx.drawImage(img, (slot.width - dw) / 2 + (t.x || 0), (slot.height - dh) / 2 + (t.y || 0), dw, dh);
  if (layer.duotone) applyDuotone(sctx, slot.width, slot.height);

  ctx.save();
  if (photoPath(ctx, layer)) ctx.clip();
  ctx.drawImage(slot, layer.x, layer.y, layer.w, layer.h);
  ctx.restore();
}

/* ---------------------------------------------------------------- renderer */

function drawArt(ctx, layer) {
  const src = layerSrc(layer, {});
  const img = imageCache.get(src) && imageCache.get(src).settled;
  if (img) ctx.drawImage(img, layer.x, layer.y, layer.w, layer.h);
}

function drawQR(ctx, layer, values) {
  const src = layerSrc(layer, values);
  const img = src && imageCache.get(src) && imageCache.get(src).settled;
  if (!img) return;
  // A white card behind the code, for the same reason the passes carry one: the
  // artwork behind the slot must not get a vote in whether it scans.
  const pad = Math.round(layer.w * 0.05);
  ctx.save();
  ctx.fillStyle = '#ffffff';
  roundedPath(ctx, layer.x - pad, layer.y - pad, layer.w + pad * 2, layer.h + pad * 2, pad);
  ctx.fill();
  ctx.drawImage(img, layer.x, layer.y, layer.w, layer.h);
  ctx.restore();
}

// The one draw function. Layer order in the spec is draw order, which is what
// puts the artwork's decoration above the portrait and the caption above that.
function renderPoster(canvas, template, values, transforms) {
  canvas.width = template.width;
  canvas.height = template.height;
  const ctx = canvas.getContext('2d');
  ctx.imageSmoothingQuality = 'high';
  ctx.fillStyle = template.spec.background || PS.cream;
  ctx.fillRect(0, 0, canvas.width, canvas.height);
  const defaults = template.spec.defaults || {};
  for (const layer of template.spec.layers) {
    if (layer.type === 'art') drawArt(ctx, layer);
    else if (layer.type === 'photo') drawPhoto(ctx, layer, values, (transforms || {})[layer.key]);
    else if (layer.type === 'qr') drawQR(ctx, layer, values);
    else if (layer.type === 'text') drawText(ctx, layer, values, defaults);
  }
  return ctx;
}

// loadImage caches promises; the draw path needs the resolved element, so settle
// them onto the promise before any render.
// Returns the layers whose image could not be loaded. drawArt and drawPhoto
// skip a missing image rather than throwing, which keeps the editor usable but
// would otherwise let someone download a poster quietly missing its artwork.
// Callers surface this; nothing else can tell the difference.
async function settleImages(template, values) {
  await preload(template, values);
  for (const [, p] of imageCache) {
    if (p.settled === undefined) p.settled = await p.then(v => v, () => null);
  }
  return template.spec.layers.filter(l => {
    const src = layerSrc(l, values);
    return src && !(imageCache.get(src) || {}).settled;
  });
}

async function exportPNG(template, values, transforms) {
  const broken = await settleImages(template, values);
  if (broken.length) throw Error(`Cannot export: ${broken.length} of this template's images `
    + `could not be loaded (${broken.map(l => l.label || l.id).join(', ')}).`);
  const canvas = document.createElement('canvas');
  renderPoster(canvas, template, values, transforms);
  return canvas.toDataURL('image/png');
}

function downloadDataURL(dataURL, filename) {
  const a = document.createElement('a');
  a.href = dataURL;
  a.download = filename;
  document.body.appendChild(a);
  a.click();
  a.remove();
}

const slugify = v => String(v || 'poster').toLowerCase().replace(/[^a-z0-9]+/g, '-').replace(/^-|-$/g, '') || 'poster';

// Screens rendered from a list need the same rule app.js's bind() applies: an
// action clears the previous outcome before it runs, so a stale error cannot
// outlive the screen it belonged to.
const navigate = (selector, fn) => document.querySelectorAll(selector).forEach(el =>
  el.addEventListener('click', () => {
    message('');
    Promise.resolve(fn(el)).catch(e => message(e.message, true));
  }));

/* ------------------------------------------------------------------ upload */

async function uploadAsset(kind, file, label) {
  if (!file || !file.size) throw Error('Choose an image.');
  if (file.size > 8 * 1024 * 1024) throw Error('Images must be 8 MB or smaller.');
  const url = `/api/v1/admin/posters/assets?kind=${kind}&label=${encodeURIComponent(label || file.name)}`;
  const res = await fetch(url, {
    method: 'POST',
    headers: { 'X-CSRF-Token': csrf(), 'Content-Type': 'application/octet-stream' },
    body: file,
  });
  const out = await res.json();
  if (!res.ok) throw Error(out.error || 'Upload failed. Please retry.');
  return out;
}

/* -------------------------------------------------------------------- data */

let studio = { templates: [], families: new Map(), logos: [] };

async function loadTemplates() {
  const d = await api('/admin/posters/templates');
  studio.logos = d.builtin_logos || [];
  studio.templates = (d.items || []).map(t => ({ ...t, spec: typeof t.spec === 'string' ? JSON.parse(t.spec) : t.spec }));
  studio.families = new Map();
  for (const t of studio.templates) {
    if (!studio.families.has(t.family)) studio.families.set(t.family, []);
    studio.families.get(t.family).push(t);
  }
}

// Fields are the union of every keyed layer across the family's sizes, so a
// poster is filled once and exported at all three.
function familyFields(templates) {
  const fields = new Map();
  for (const t of templates) {
    for (const l of t.spec.layers) {
      if (!l.key || fields.has(l.key)) continue;
      fields.set(l.key, {
        key: l.key, type: l.type,
        label: l.label || l.key.replace(/_/g, ' ').replace(/^./, c => c.toUpperCase()),
        value: (t.spec.defaults || {})[l.key] || '',
      });
    }
  }
  return [...fields.values()];
}

/* ------------------------------------------------------------------- home */

// A post type is one or more families that differ only by look, so the gallery
// can group "speaker-reveal-light" and "-dark" under one card with a toggle.
// Families that do not end in a known look stand on their own.
const LOOKS = ['light', 'dark'];

function postTypes() {
  const groups = new Map();
  for (const [family, list] of studio.families) {
    const look = LOOKS.find(l => family.endsWith('-' + l));
    const key = look ? family.slice(0, -look.length - 1) : family;
    if (!groups.has(key)) groups.set(key, { key, looks: [] });
    groups.get(key).looks.push({ family, look: look || '', list });
  }
  for (const g of groups.values()) {
    g.looks.sort((a, b) => LOOKS.indexOf(a.look) - LOOKS.indexOf(b.look));
    // The card's title drops the look suffix the template name carries.
    g.title = g.looks[0].list[0].name.replace(/\s*[-–]\s*(light|dark)\s*$/i, '');
    g.shipped = g.looks.every(l => l.list.every(t => t.origin === 'seed'));
  }
  return [...groups.values()].sort((a, b) => a.title.localeCompare(b.title));
}

// The thumbnail is the template's own artwork layers stacked, not a canvas
// render: it is the picture staff recognise, costs two cached <img> per card,
// and needs none of the font loading a real render would.
//
// The first image sits in normal flow and gives the box its height; the rest are
// absolutely positioned over it. Sizing off the image rather than an
// aspect-ratio on the wrapper keeps it correct inside a <button>, whose flex
// formatting context swallowed the ratio.
function thumbnail(list, eager) {
  const t = list.find(x => x.size === '4x5') || list[0];
  const art = t.spec.layers.filter(l => l.type === 'art');
  const src = l => l.asset_id ? '/api/v1/admin/posters/assets/' + l.asset_id
    : '/static/logos/' + l.builtin + '.png';
  if (!art.length) {
    // A template can legitimately have no artwork - text, a logo and a QR on a
    // plain background. Show its background colour rather than an empty box.
    return `<span class="poster-thumb poster-thumb-plain"
      data-bg="${esc(t.spec.background || PS.cream)}"></span>`;
  }
  return `<span class="poster-thumb" data-bg="${esc(t.spec.background || PS.cream)}">${
    // The shown look loads eagerly; the other waits until someone toggles to
    // it, which keeps the gallery to two images per card on first paint.
    art.map(l => `<img src="${esc(src(l))}" alt="" decoding="async"
      loading="${eager ? 'eager' : 'lazy'}">`).join('')
  }</span>`;
}

async function posterHome() {
  builderKeys?.abort();
  await loadTemplates();
  const posters = await api('/admin/posters');
  const types = postTypes();
  app.innerHTML = `<div class="admin-page poster-page">
    <header class="admin-heading">
      <div><p class="admin-kicker">Staff console</p><h1>Social posters</h1>
      <p class="admin-intro">Pick a template, drop in a portrait and the wording, and download a ready-to-post image at every size.</p></div>
      <div class="admin-header-actions">
        <button id="new-template" class="secondary">New template</button>
        <button id="poster-back" class="quiet">Back to console</button>
      </div>
    </header>
    ${types.length ? `<ul class="poster-gallery">${types.map(g => {
      const active = g.looks[0];
      return `<li class="poster-card" data-type="${esc(g.key)}">
        ${g.looks.map((l, i) => `<button class="poster-pick${i ? '' : ' active'}" data-family="${esc(l.family)}"
            aria-label="Make a ${esc(g.title)} poster">${thumbnail(l.list, !i)}</button>`).join('')}
        <div class="poster-card-body">
          <strong>${esc(g.title)}</strong>
          ${g.looks.length > 1 ? `<span class="poster-looks">${g.looks.map((l, i) =>
            `<button class="poster-look${i ? '' : ' active'}" data-family="${esc(l.family)}">${esc(l.look)}</button>`).join('')}</span>` : ''}
          <span class="poster-sizes">${active.list.map(t => esc(PS.sizeLabels[t.size] || t.size)).join(' · ')}</span>
        </div>
        <div class="poster-card-actions">
          <button class="make-poster" data-family="${esc(active.family)}">Make a poster</button>
          <details class="poster-menu">
            <summary aria-label="More actions for ${esc(g.title)}">&hellip;</summary>
            <div class="poster-menu-items">
              ${active.list.map(t => `<button class="edit-template" data-id="${esc(t.id)}">Edit ${esc(PS.sizeLabels[t.size] || t.size)}</button>`).join('')}
              ${Object.keys(PS.sizes).filter(sz => !active.list.some(t => t.size === sz))
                .map(sz => `<button class="add-size" data-family="${esc(active.family)}" data-size="${esc(sz)}">Add ${esc(PS.sizeLabels[sz])}</button>`).join('')}
              <button class="dup-template" data-family="${esc(active.family)}" data-name="${esc(g.title)}">Duplicate&hellip;</button>
              <button class="retire-template danger" data-family="${esc(active.family)}" data-title="${esc(g.title)}">Retire</button>
            </div>
          </details>
        </div>
        <form class="poster-dup-form" hidden>
          <label>Name for the copy
            <input name="name" maxlength="120" placeholder="${esc(g.title)} (our version)" required></label>
          <p class="help">A copy is yours: rolling out new artwork never changes it.</p>
          <div class="actions"><button>Create copy</button>
            <button type="button" class="dup-cancel secondary">Cancel</button></div>
        </form>
        ${g.shipped ? '' : '<span class="poster-badge">edited</span>'}
      </li>`;
    }).join('')}</ul>`
    : '<p class="muted card">No templates yet. Create one to get started - a template works with nothing but text and a logo, so you do not have to wait for artwork.</p>'}
    <section class="card">
      <h2>Recent posters</h2>
      ${(posters.items || []).length
        ? table(posters.items.map(p => ({ ...p, updated_at: date(p.updated_at) })),
            [['title', 'Title'], ['family', 'Template'], ['created_by', 'Made by'], ['updated_at', 'Updated']])
          + `<div class="poster-reopen">${posters.items.map(p => `<button class="open-poster secondary" data-id="${esc(p.id)}" data-family="${esc(p.family)}">Reopen ${esc(p.title || p.family)}</button>`).join('')}</div>`
        : '<p class="muted">Nothing yet. Posters you save appear here so you can reopen and adjust them.</p>'}
    </section>
  </div>`;

  // The page's CSP is style-src 'self', so a style="" attribute is dropped
  // before it ever reaches layout. Script-driven CSSOM is not covered by that
  // directive, and .poster-thumb carries a cream fallback for the strictest
  // case, so nothing depends on this succeeding.
  document.querySelectorAll('.poster-thumb[data-bg]').forEach(el => {
    try { el.style.background = el.dataset.bg; } catch { /* fallback stands */ }
  });

  bind('poster-back', 'click', () => adminPage());
  bind('new-template', 'click', () => templateBuilder(null, { family: '', size: '4x5' }));
  navigate('.poster-pick', el => posterEditor(el.dataset.family));
  navigate('.make-poster', el => posterEditor(el.dataset.family));
  navigate('.edit-template', el => templateBuilder(el.dataset.id));
  navigate('.add-size', el => templateBuilder(null, { family: el.dataset.family, size: el.dataset.size }));
  navigate('.open-poster', el => posterEditor(el.dataset.family, el.dataset.id));

  // Switching look re-points the card's actions at the other family, so the
  // menu never acts on the look that is not showing.
  document.querySelectorAll('.poster-look').forEach(el => el.addEventListener('click', () => {
    const card = el.closest('.poster-card');
    const family = el.dataset.family;
    card.querySelectorAll('.poster-look').forEach(b => b.classList.toggle('active', b === el));
    card.querySelectorAll('.poster-pick').forEach(b => b.classList.toggle('active', b.dataset.family === family));
    const group = postTypes().find(g => g.key === card.dataset.type);
    const chosen = group.looks.find(l => l.family === family);
    card.querySelector('.poster-sizes').textContent =
      chosen.list.map(t => PS.sizeLabels[t.size] || t.size).join(' · ');
    card.querySelector('.make-poster').dataset.family = family;
    card.querySelector('.dup-template').dataset.family = family;
    card.querySelector('.retire-template').dataset.family = family;
    const menu = card.querySelector('.poster-menu-items');
    menu.querySelectorAll('.edit-template').forEach((b, i) => {
      const t = chosen.list[i];
      if (t) { b.dataset.id = t.id; b.textContent = 'Edit ' + (PS.sizeLabels[t.size] || t.size); b.hidden = false; }
      else b.hidden = true;
    });
  }));

  document.querySelectorAll('.dup-template').forEach(el => el.addEventListener('click', () => {
    const card = el.closest('.poster-card');
    card.querySelector('.poster-menu').open = false;
    const form = card.querySelector('.poster-dup-form');
    form.hidden = false;
    form.querySelector('input').focus();
  }));
  document.querySelectorAll('.dup-cancel').forEach(el => el.addEventListener('click', () => {
    el.closest('.poster-dup-form').hidden = true;
  }));
  document.querySelectorAll('.poster-dup-form').forEach(form =>
    form.addEventListener('submit', async e => {
      e.preventDefault();
      try {
        const card = form.closest('.poster-card');
        await busy(e.submitter, async () => {
          const out = await post('/admin/posters/templates/duplicate', {
            family: card.querySelector('.dup-template').dataset.family,
            name: form.querySelector('input').value.trim(),
          });
          message(`Copied to "${out.name}" with ${out.sizes} size${out.sizes === 1 ? '' : 's'}. It is yours to edit.`);
          await posterHome();
        });
      } catch (err) { message(err.message, true); }
    }));

  document.querySelectorAll('.retire-template').forEach(el => el.addEventListener('click', async () => {
    el.closest('.poster-menu').open = false;
    if (!confirm(`Retire "${el.dataset.title}"? It disappears from this list. `
      + `Posters already made from it keep their wording, and it can be brought back by a manager.`)) return;
    try {
      await post('/admin/posters/templates/retire', { family: el.dataset.family, size: '' });
      message('Template retired.');
      await posterHome();
    } catch (err) { message(err.message, true); }
  }));
}

/* ----------------------------------------------------------- poster editor */

async function posterEditor(family, posterId) {
  builderKeys?.abort();
  const templates = (studio.families.get(family) || []).slice();
  if (!templates.length) throw Error('That template family has no sizes yet.');
  await loadFonts(templates);

  const fields = familyFields(templates);
  const state = { id: posterId || '', title: '', values: {}, transforms: {}, active: templates[0].size };
  for (const f of fields) if (f.type === 'text' || f.type === 'qr') state.values[f.key] = f.value;

  if (posterId) {
    const saved = await api('/admin/posters/' + posterId);
    const content = typeof saved.content === 'string' ? JSON.parse(saved.content) : saved.content;
    state.title = saved.title || '';
    Object.assign(state.values, content.values || {});
    state.transforms = content.transforms || {};
  }

  function template() { return templates.find(t => t.size === state.active); }
  function transformsFor(size) {
    const out = {};
    for (const key of Object.keys(state.transforms)) out[key] = state.transforms[key][size] || { x: 0, y: 0, scale: 1 };
    return out;
  }
  function transform(key, size) {
    if (!state.transforms[key]) state.transforms[key] = {};
    if (!state.transforms[key][size]) state.transforms[key][size] = { x: 0, y: 0, scale: 1 };
    return state.transforms[key][size];
  }

  async function draw() {
    const t = template();
    const canvas = document.getElementById('poster-canvas');
    if (!canvas) return;
    const broken = await settleImages(t, state.values);
    renderPoster(canvas, t, state.values, transformsFor(t.size));
    // Without this a poster whose artwork failed to load looks merely plain,
    // and downloads that way.
    if (broken.length) {
      message(`${broken.length} image${broken.length === 1 ? '' : 's'} could not be loaded `
        + `(${broken.map(l => l.label || l.id).join(', ')}). The download will be missing them.`, true);
    }
  }

  function shell() {
    const t = template();
    const photoFields = fields.filter(f => f.type === 'photo');
    app.innerHTML = `<div class="admin-page poster-page">
      <header class="admin-heading">
        <div><p class="admin-kicker">Social posters</p><h1>${esc(templates[0].name)}</h1>
        <p class="admin-intro">Fill the wording once. It carries across every size; only the photo framing is per size.</p></div>
        <div class="admin-header-actions"><button id="poster-home" class="quiet">All posters</button></div>
      </header>
      <div class="poster-studio">
        <div class="poster-stage">
          <div class="poster-tabs" role="tablist">${templates.map(x => `<button role="tab" class="poster-tab${x.size === state.active ? ' active' : ''}" data-size="${esc(x.size)}" aria-selected="${x.size === state.active}">${esc(PS.sizeLabels[x.size] || x.size)}</button>`).join('')}</div>
          <canvas id="poster-canvas" width="${t.width}" height="${t.height}"></canvas>
          ${photoFields.length ? '<p class="help">Drag the poster to reframe the photo. Scroll or pinch over it to zoom.</p>' : ''}
        </div>
        <div class="poster-fields">
          <div class="card">
            <label>Poster title <span class="help-inline">for the console list only</span>
              <input id="poster-title" value="${esc(state.title)}" maxlength="200"></label>
            ${fields.map(f => fieldControl(f)).join('')}
          </div>
          <div class="card">
            <div class="actions">
              <button id="poster-save" class="secondary">Save poster</button>
              <button id="poster-download">Download ${esc(PS.sizeLabels[state.active] || state.active)}</button>
              <button id="poster-download-all" class="secondary">Download all sizes</button>
            </div>
            <p class="help">Saving keeps the wording and framing so this poster can be reopened and adjusted later. It does not publish anything.</p>
          </div>
        </div>
      </div>
    </div>`;
    wire();
    draw();
  }

  function fieldControl(f) {
    if (f.type === 'photo') {
      const v = state.values[f.key];
      return `<label>${esc(f.label)}
        <input type="file" class="poster-photo" data-key="${esc(f.key)}" accept="image/png,image/jpeg"></label>
        ${v ? `<div class="poster-photo-tools"><button type="button" class="poster-reset secondary" data-key="${esc(f.key)}">Recentre</button>
        <label class="check"><input type="checkbox" class="poster-duotone" data-key="${esc(f.key)}" ${v.duotone ? 'checked' : ''}>Apply the event duotone</label></div>` : ''}`;
    }
    if (f.type === 'qr') {
      return `<label>${esc(f.label)} <span class="help-inline">https URL</span>
        <input class="poster-field" data-key="${esc(f.key)}" type="url" value="${esc(state.values[f.key] || '')}" maxlength="512"></label>`;
    }
    const long = String(state.values[f.key] || '').length > 60;
    return `<label>${esc(f.label)}
      ${long ? `<textarea class="poster-field" data-key="${esc(f.key)}" rows="3" maxlength="600">${esc(state.values[f.key] || '')}</textarea>`
        : `<input class="poster-field" data-key="${esc(f.key)}" value="${esc(state.values[f.key] || '')}" maxlength="600">`}</label>`;
  }

  function wire() {
    bind('poster-home', 'click', () => posterHome());
    document.getElementById('poster-title').addEventListener('input', e => { state.title = e.target.value; });

    document.querySelectorAll('.poster-tab').forEach(el => el.addEventListener('click', () => {
      state.active = el.dataset.size; shell();
    }));

    document.querySelectorAll('.poster-field').forEach(el => el.addEventListener('input', () => {
      state.values[el.dataset.key] = el.value; draw();
    }));

    document.querySelectorAll('.poster-photo').forEach(el => el.addEventListener('change', async () => {
      try {
        const file = el.files[0];
        if (!file) return;
        message('Uploading photo…');
        const out = await uploadAsset('photo', file);
        const previous = state.values[el.dataset.key] || {};
        state.values[el.dataset.key] = { asset_id: out.id, duotone: !!previous.duotone };
        state.transforms[el.dataset.key] = {};
        message('Photo added. Drag to reframe.');
        shell();
      } catch (e) { message(e.message, true); }
    }));

    document.querySelectorAll('.poster-duotone').forEach(el => el.addEventListener('change', () => {
      const v = state.values[el.dataset.key];
      if (v) { v.duotone = el.checked; applyDuotoneFlag(el.dataset.key, el.checked); draw(); }
    }));

    document.querySelectorAll('.poster-reset').forEach(el => el.addEventListener('click', () => {
      state.transforms[el.dataset.key] = {}; draw();
    }));

    bindPhotoDragging();

    bind('poster-save', 'click', async e => busy(e.target, async () => {
      const out = await post('/admin/posters', {
        id: state.id, family, title: state.title,
        content: { values: state.values, transforms: state.transforms },
      });
      state.id = out.id;
      message('Poster saved.');
    }));

    bind('poster-download', 'click', async e => busy(e.target, async () => {
      const t = template();
      const url = await exportPNG(t, state.values, transformsFor(t.size));
      downloadDataURL(url, `${slugify(state.title || family)}-${t.size}.png`);
      message('Downloaded.');
    }, 'Rendering…'));

    bind('poster-download-all', 'click', async e => busy(e.target, async () => {
      for (const t of templates) {
        const url = await exportPNG(t, state.values, transformsFor(t.size));
        downloadDataURL(url, `${slugify(state.title || family)}-${t.size}.png`);
      }
      message(`Downloaded ${templates.length} sizes.`);
    }, 'Rendering…'));
  }

  // The duotone flag lives on the photo value, but the renderer reads it off the
  // layer, so mirror it onto every photo layer that uses this key.
  function applyDuotoneFlag(key, on) {
    for (const t of templates) for (const l of t.spec.layers) if (l.type === 'photo' && l.key === key) l.duotone = on;
  }
  for (const f of fields) if (f.type === 'photo' && state.values[f.key]) applyDuotoneFlag(f.key, !!state.values[f.key].duotone);

  function bindPhotoDragging() {
    const canvas = document.getElementById('poster-canvas');
    const t = template();
    const photoLayers = t.spec.layers.filter(l => l.type === 'photo' && state.values[l.key]);
    if (!canvas || !photoLayers.length) return;

    const toCanvas = e => {
      const r = canvas.getBoundingClientRect();
      return { x: (e.clientX - r.left) * (canvas.width / r.width), y: (e.clientY - r.top) * (canvas.height / r.height) };
    };
    const hit = p => photoLayers.slice().reverse()
      .find(l => p.x >= l.x && p.x <= l.x + l.w && p.y >= l.y && p.y <= l.y + l.h);

    let dragging = null, last = null;
    canvas.addEventListener('pointerdown', e => {
      const p = toCanvas(e);
      const layer = hit(p);
      if (!layer) return;
      dragging = layer; last = p;
      canvas.setPointerCapture(e.pointerId);
      canvas.classList.add('grabbing');
    });
    canvas.addEventListener('pointermove', e => {
      if (!dragging) return;
      const p = toCanvas(e);
      const tr = transform(dragging.key, t.size);
      tr.x += p.x - last.x; tr.y += p.y - last.y;
      last = p;
      draw();
    });
    const stop = e => {
      if (!dragging) return;
      dragging = null;
      canvas.classList.remove('grabbing');
      if (e.pointerId !== undefined && canvas.hasPointerCapture(e.pointerId)) canvas.releasePointerCapture(e.pointerId);
    };
    canvas.addEventListener('pointerup', stop);
    canvas.addEventListener('pointercancel', stop);
    canvas.addEventListener('wheel', e => {
      const layer = hit(toCanvas(e));
      if (!layer) return;
      e.preventDefault();
      const tr = transform(layer.key, t.size);
      tr.scale = Math.min(6, Math.max(0.2, (tr.scale || 1) * (e.deltaY < 0 ? 1.06 : 1 / 1.06)));
      draw();
    }, { passive: false });
  }

  shell();
}

/* --------------------------------------------------------- template builder */

// A stand-in image so a photo slot is not an empty hole while a template is
// being laid out - you cannot judge a crop or a shape against nothing. Drawn
// rather than shipped: no bytes to embed, and a data URL is what the CSP's
// `img-src 'self' data:` allows, the same reason uploads are read as data URLs.
// One live builder at a time, so one controller. Aborted whenever another
// screen takes over, or its key handler would outlive the screen it belongs to.
let builderKeys = null;

const samplePhotoCache = {};
function samplePhoto(fit) {
  const logo = fit === 'contain';
  const key = logo ? 'logo' : 'portrait';
  if (samplePhotoCache[key]) return samplePhotoCache[key];
  const c = document.createElement('canvas');
  const g = c.getContext('2d');
  if (logo) {
    // A contain-fit slot is the partner-logo slot, where a face is misleading.
    c.width = 640; c.height = 320;
    g.fillStyle = '#ffffff';
    g.fillRect(0, 0, c.width, c.height);
    g.strokeStyle = 'rgba(11,51,41,0.35)';
    g.lineWidth = 6;
    g.setLineDash([18, 14]);
    g.strokeRect(12, 12, c.width - 24, c.height - 24);
    g.setLineDash([]);
    g.fillStyle = PS.forest;
    g.font = '600 64px sans-serif';
    g.textAlign = 'center';
    g.textBaseline = 'middle';
    g.fillText('PARTNER LOGO', c.width / 2, c.height / 2);
  } else {
    // Head and shoulders in the portrait duotone's own three stops, at 4:5 with
    // the head high enough to survive a square or circular centre crop.
    c.width = 800; c.height = 1000;
    g.fillStyle = 'rgb(18,44,35)';
    g.fillRect(0, 0, c.width, c.height);
    g.fillStyle = 'rgb(118,150,122)';
    g.beginPath();
    g.ellipse(400, 1090, 330, 360, 0, 0, Math.PI * 2);
    g.fill();
    g.fillStyle = 'rgb(232,238,216)';
    g.beginPath();
    g.arc(400, 420, 172, 0, Math.PI * 2);
    g.fill();
    g.fillStyle = 'rgba(232,238,216,0.65)';
    g.font = '600 38px sans-serif';
    g.textAlign = 'center';
    g.fillText('SAMPLE', 400, 960);
  }
  samplePhotoCache[key] = c.toDataURL('image/png');
  return samplePhotoCache[key];
}

// A layer's key is its field: two layers sharing one draw the same content and
// share a default. That is deliberate - a name can appear twice on a poster -
// but it must not be what you get by accident. Every new layer takes the first
// free key, so adding three text layers gives three fields, not one field
// drawn three times.
function freeKey(base, taken) {
  if (!taken.has(base)) return base;
  for (let n = 2; ; n++) if (!taken.has(`${base}-${n}`)) return `${base}-${n}`;
}

const newLayer = (type, width, height, taken = new Set()) => {
  const base = { id: 'l' + Math.random().toString(36).slice(2, 8), type, x: Math.round(width * 0.1), y: Math.round(height * 0.1), w: Math.round(width * 0.5), h: Math.round(height * 0.12) };
  if (type === 'art') return { ...base, builtin: 'bio-connect', x: 0, y: 0, w: width, h: height };
  if (type === 'photo') {
    const key = freeKey('photo', taken);
    return { ...base, key, label: key === 'photo' ? 'Portrait' : 'Portrait ' + key.slice(6), fit: 'cover', shape: 'rect', radius: 0, h: Math.round(height * 0.5) };
  }
  if (type === 'qr') {
    const key = freeKey('qr_url', taken);
    return { ...base, key, label: 'Registration QR', w: 160, h: 160 };
  }
  const key = freeKey('headline', taken);
  return { ...base, key, label: key === 'headline' ? 'Headline' : 'Text ' + key.slice(9), font: 'display', weight: 600, size: 56, color: PS.forest, align: 'left', transform: 'none', tracking: 0, line_height: 1.15, autofit: true };
};

async function templateBuilder(templateId, seed) {
  if (!studio.templates.length) await loadTemplates();
  let tpl;
  if (templateId) {
    tpl = studio.templates.find(t => t.id === templateId);
    if (!tpl) throw Error('Template not found.');
    tpl = JSON.parse(JSON.stringify(tpl));
  } else {
    const size = (seed && seed.size) || '4x5';
    const [w, h] = PS.sizes[size];
    const sibling = seed && seed.family ? (studio.families.get(seed.family) || [])[0] : null;
    tpl = {
      id: '', family: seed ? seed.family : '', name: sibling ? sibling.name : '', size, width: w, height: h,
      spec: { background: PS.cream, defaults: sibling ? { ...sibling.spec.defaults } : {
        eyebrow: 'MEET THE SPEAKER', dates: '08-09 October 2026',
        venue: 'Hyatt Regency Trivandrum', qr_url: 'https://reg.bioconnect.kerala.gov.in/delegates',
      }, layers: [newLayer('text', w, h)] },
    };
  }

  // Family is the key that ties a template's three sizes together, and it is
  // the one field staff could not be expected to reason about. It is derived
  // from the name on a new template, inherited and locked when adding a size,
  // and frozen when editing - so it is never typed twice and cannot drift.
  const derived = !templateId && !(seed && seed.family);
  if (derived) tpl.family = slugify(tpl.name);

  const state = { selected: tpl.spec.layers[0].id };
  let nudgeSettle = null;
  const assets = await api('/admin/posters/assets?kind=art');
  const selected = () => tpl.spec.layers.find(l => l.id === state.selected);

  async function draw() {
    const canvas = document.getElementById('builder-canvas');
    if (!canvas) return;
    await loadFonts([tpl]);
    // Sample content so an empty template is not an empty preview.
    const sample = {};
    for (const l of tpl.spec.layers) {
      if (l.type === 'text') sample[l.key] = (tpl.spec.defaults || {})[l.key] || l.label || 'Sample text';
      if (l.type === 'qr') sample[l.key] = (tpl.spec.defaults || {})[l.key] || 'https://bioconnect.kerala.gov.in';
      if (l.type === 'photo') sample[l.key] = { data_url: samplePhoto(l.fit) };
    }
    await settleImages(tpl, sample);
    const ctx = renderPoster(canvas, tpl, sample, {});
    // Selection chrome is drawn after the poster and only on screen; exports go
    // through a fresh canvas, so it can never end up in a file.
    for (const l of tpl.spec.layers) {
      const on = l.id === state.selected;
      ctx.save();
      ctx.strokeStyle = on ? PS.gold : 'rgba(11,51,41,0.35)';
      ctx.lineWidth = on ? 4 : 2;
      ctx.setLineDash(on ? [] : [10, 8]);
      // Outline the shape the photo is actually cut to, or a circular slot
      // reads as a square while you drag it.
      const shaped = l.type === 'photo' && photoPath(ctx, l);
      if (shaped) ctx.stroke(); else ctx.strokeRect(l.x, l.y, l.w, l.h);
      if (on) {
        ctx.setLineDash([]);
        // The drag target is still the box, so a shaped slot shows it faintly.
        if (shaped) {
          ctx.lineWidth = 2;
          ctx.setLineDash([10, 8]);
          ctx.strokeStyle = 'rgba(228,173,84,0.5)';
          ctx.strokeRect(l.x, l.y, l.w, l.h);
          ctx.setLineDash([]);
        }
        ctx.fillStyle = PS.gold;
        ctx.fillRect(l.x + l.w - 18, l.y + l.h - 18, 18, 18);
      }
      ctx.restore();
    }
  }

  function shell() {
    const l = selected();
    app.innerHTML = `<div class="admin-page poster-page">
      <header class="admin-heading">
        <div><p class="admin-kicker">Social posters</p><h1>${templateId ? 'Edit template' : 'New template'}</h1>
        <p class="admin-intro">Drag the boxes to lay out the slots. The layer list reads like a stack, topmost first, so move a decoration above the photo to have it sit over the portrait. Photo slots preview a sample image.</p></div>
        <div class="admin-header-actions"><button id="builder-home" class="quiet">All posters</button></div>
      </header>
      <div class="poster-studio">
        <div class="poster-stage">
          <canvas id="builder-canvas" width="${tpl.width}" height="${tpl.height}"></canvas>
          <p class="help">Drag inside a box to move it, or its bottom-right corner to resize. Hold <kbd>Shift</kbd> while resizing to keep its proportions. Arrow keys nudge the selected layer by 1, with <kbd>Shift</kbd> by 10; <kbd>Delete</kbd> removes it.</p>
        </div>
        <div class="poster-fields">
          <div class="card">
            <h2>Template</h2>
            <div class="fields one-column">
              <label>Name<input id="tpl-name" value="${esc(tpl.name)}" maxlength="120"></label>
              ${derived
                ? `<p class="help">Saved as <code id="tpl-family-preview">${esc(tpl.family)}</code>, the key that keeps this template's three sizes together. The other sizes inherit it.</p>`
                : `<label>Family <span class="help-inline">shared by this template's sizes</span>
                <input id="tpl-family" value="${esc(tpl.family)}" maxlength="64" disabled></label>`}
              <label>Size<select id="tpl-size" ${templateId ? 'disabled' : ''}>${Object.keys(PS.sizes).map(s => `<option value="${s}" ${tpl.size === s ? 'selected' : ''}>${esc(PS.sizeLabels[s])} (${PS.sizes[s].join('x')})</option>`).join('')}</select></label>
              <label>Background<input id="tpl-bg" type="color" value="${esc(tpl.spec.background || PS.cream)}"></label>
            </div>
          </div>
          <div class="card">
            <h2>Layers <span class="help-inline">topmost first</span></h2>
            <ul class="layer-list">${stackOrder().map(({ x, i }) => `<li class="${x.id === state.selected ? 'active' : ''}">
              <button class="layer-pick" data-id="${esc(x.id)}">${esc(x.label || x.key || x.builtin || x.type)} <span class="layer-type">${esc(x.type)}</span></button>
              <span class="layer-order">
                <button class="layer-up secondary" data-id="${esc(x.id)}" ${i === tpl.spec.layers.length - 1 ? 'disabled' : ''} aria-label="Move up">&uarr;</button>
                <button class="layer-down secondary" data-id="${esc(x.id)}" ${i === 0 ? 'disabled' : ''} aria-label="Move down">&darr;</button>
                <button class="layer-remove danger" data-id="${esc(x.id)}" ${tpl.spec.layers.length === 1 ? 'disabled' : ''} aria-label="Remove">&times;</button>
              </span></li>`).join('')}</ul>
            <div class="actions">${['art', 'photo', 'text', 'qr'].map(t => `<button class="add-layer secondary" data-type="${t}">Add ${t}</button>`).join('')}</div>
          </div>
          <div class="card">
            <h2>Selected layer</h2>
            ${l ? layerControls(l) : '<p class="muted">Nothing selected.</p>'}
          </div>
          <div class="card">
            <div class="actions"><button id="tpl-save">Save template</button></div>
          </div>
        </div>
      </div>
    </div>`;
    wire();
    draw();
  }

  // spec.layers is draw order, so index 0 is the bottom of the stack. The list
  // reads the other way round - topmost first, like every design tool - so that
  // "move up" moves the row up and the layer up at the same time. Before this
  // the list was in array order and the up arrow visibly moved a row down.
  function stackOrder() {
    return tpl.spec.layers.map((x, i) => ({ x, i })).reverse();
  }

  // Mirrors photoPath's fallback so the picker shows what is actually drawn for
  // a seeded slot saved before `shape` existed.
  const photoShape = l => l.shape || (l.radius ? 'rounded' : 'rect');

  // Two layers on one key draw the same content. Say so where it is decided,
  // rather than leaving someone to discover it by typing a default and seeing
  // it appear twice.
  const shareNote = l => {
    const n = tpl.spec.layers.filter(x => x !== l && x.key && x.key === l.key).length;
    return n ? `<p class="help">This key is shared with ${n} other layer${n > 1 ? 's' : ''}, so they all draw the same content. Change it to make this one its own field.</p>` : '';
  };

  function layerControls(l) {
    const box = `<div class="fields grid-4">
      ${['x', 'y', 'w', 'h'].map(k => `<label>${k.toUpperCase()}<input class="layer-num" data-prop="${k}" type="number" value="${Math.round(l[k])}"></label>`).join('')}</div>`;
    if (l.type === 'art') {
      return `${box}<label>Source<select class="layer-src">
        <optgroup label="Brand logos">${studio.logos.map(n => `<option value="b:${esc(n)}" ${l.builtin === n ? 'selected' : ''}>${esc(n)}</option>`).join('')}</optgroup>
        <optgroup label="Uploaded artwork">${(assets.items || []).map(a => `<option value="a:${esc(a.id)}" ${l.asset_id === a.id ? 'selected' : ''}>${esc(a.label || a.id.slice(0, 8))} (${a.width}x${a.height})</option>`).join('')}</optgroup>
      </select></label>
      <label>Upload new artwork (PNG keeps transparency)<input type="file" id="layer-art-upload" accept="image/png,image/jpeg"></label>`;
    }
    if (l.type === 'photo') {
      return `${box}
        <div class="fields">
          <label>Field key<input class="layer-prop" data-prop="key" value="${esc(l.key)}" maxlength="64"></label>
          <label>Label<input class="layer-prop" data-prop="label" value="${esc(l.label || '')}" maxlength="64"></label>
          <label>Fit<select class="layer-prop" data-prop="fit"><option value="cover" ${l.fit === 'cover' ? 'selected' : ''}>Cover (fills, crops)</option><option value="contain" ${l.fit === 'contain' ? 'selected' : ''}>Contain (fits, for logos)</option></select></label>
          <label>Shape<select class="layer-shape" data-prop="shape">${[['rect', 'Rectangle'], ['rounded', 'Rounded corners'], ['circle', 'Circle / oval'], ['arch', 'Arch']]
            .map(([v, name]) => `<option value="${v}" ${photoShape(l) === v ? 'selected' : ''}>${name}</option>`).join('')}</select></label>
          ${photoShape(l) === 'rounded' ? `<label>Corner radius<input class="layer-num" data-prop="radius" type="number" min="0" value="${Math.round(l.radius || 0)}"></label>` : ''}
        </div>
        ${photoShape(l) === 'circle' && Math.round(l.w) !== Math.round(l.h) ? '<p class="help">This box is not square, so the slot is an oval. Match W and H for a circle.</p>' : ''}
        ${shareNote(l)}
        <label class="check"><input type="checkbox" class="layer-bool" data-prop="duotone" ${l.duotone ? 'checked' : ''}>Apply the event duotone by default</label>`;
    }
    if (l.type === 'qr') {
      return `${box}<div class="fields">
        <label>Field key<input class="layer-prop" data-prop="key" value="${esc(l.key)}" maxlength="64"></label>
        <label>Label<input class="layer-prop" data-prop="label" value="${esc(l.label || '')}" maxlength="64"></label></div>
        <p class="help">A QR layer stays square; width follows height.</p>${shareNote(l)}`;
    }
    return `${box}<div class="fields">
      <label>Field key<input class="layer-prop" data-prop="key" value="${esc(l.key)}" maxlength="64"></label>
      <label>Label<input class="layer-prop" data-prop="label" value="${esc(l.label || '')}" maxlength="64"></label>
      <label>Font<select class="layer-prop" data-prop="font">${Object.entries(PS.fonts)
        .map(([role, f]) => `<option value="${role}" ${l.font === role ? 'selected' : ''}>${esc(f.label)}</option>`).join('')}</select></label>
      <label>Weight<select class="layer-num" data-prop="weight">${[400, 500, 600, 700].map(x => `<option value="${x}" ${l.weight === x ? 'selected' : ''}>${x}</option>`).join('')}</select></label>
      <label>Size<input class="layer-num" data-prop="size" type="number" min="6" max="400" value="${Math.round(l.size)}"></label>
      <label>Colour<input class="layer-prop" data-prop="color" type="color" value="${esc(l.color)}"></label>
      <label>Align<select class="layer-prop" data-prop="align">${['left', 'center', 'right'].map(x => `<option value="${x}" ${l.align === x ? 'selected' : ''}>${x}</option>`).join('')}</select></label>
      <label>Case<select class="layer-prop" data-prop="transform"><option value="none" ${l.transform !== 'upper' ? 'selected' : ''}>As typed</option><option value="upper" ${l.transform === 'upper' ? 'selected' : ''}>UPPERCASE</option></select></label>
      <label>Tracking<input class="layer-num" data-prop="tracking" type="number" step="0.01" min="-0.2" max="1" value="${l.tracking || 0}"></label>
      <label>Line height<input class="layer-num" data-prop="line_height" type="number" step="0.05" min="0.6" max="3" value="${l.line_height || 1.2}"></label>
      </div>
      ${shareNote(l)}
      <label class="check"><input type="checkbox" class="layer-bool" data-prop="autofit" ${l.autofit !== false ? 'checked' : ''}>Shrink to fit the box rather than overflow</label>
      <label>Default text<textarea class="tpl-default" data-key="${esc(l.key)}" rows="2" maxlength="600">${esc((tpl.spec.defaults || {})[l.key] || '')}</textarea></label>`;
  }

  function wire() {
    bind('builder-home', 'click', () => posterHome());
    const on = (sel, ev, fn) => document.querySelectorAll(sel).forEach(el => el.addEventListener(ev, () => fn(el)));

    document.getElementById('tpl-name').addEventListener('input', e => {
      tpl.name = e.target.value;
      if (!derived) return;
      tpl.family = slugify(tpl.name);
      // textContent, not innerHTML: the name is staff input and this runs on
      // every keystroke.
      document.getElementById('tpl-family-preview').textContent = tpl.family;
    });
    document.getElementById('tpl-bg').addEventListener('input', e => { tpl.spec.background = e.target.value; draw(); });
    document.getElementById('tpl-size').addEventListener('change', e => {
      tpl.size = e.target.value;
      [tpl.width, tpl.height] = PS.sizes[tpl.size];
      shell();
    });

    on('.layer-pick', 'click', el => { state.selected = el.dataset.id; shell(); });
    on('.add-layer', 'click', el => {
      const l = newLayer(el.dataset.type, tpl.width, tpl.height, new Set(tpl.spec.layers.map(x => x.key).filter(Boolean)));
      tpl.spec.layers.push(l); state.selected = l.id; shell();
    });
    on('.layer-up', 'click', el => { move(el.dataset.id, 1); });
    on('.layer-down', 'click', el => { move(el.dataset.id, -1); });
    on('.layer-remove', 'click', el => {
      tpl.spec.layers = tpl.spec.layers.filter(x => x.id !== el.dataset.id);
      state.selected = tpl.spec.layers[tpl.spec.layers.length - 1].id;
      shell();
    });

    on('.layer-num', 'input', el => {
      const l = selected();
      l[el.dataset.prop] = Number(el.value) || 0;
      if (l.type === 'qr' && (el.dataset.prop === 'w' || el.dataset.prop === 'h')) l.w = l.h = Number(el.value) || 1;
      draw();
    });
    // Never rebuild the panel from an 'input' event on a text field: shell()
    // replaces the input, and focus() on the new one puts the caret at 0, so
    // every keystroke lands in front of the last and the text comes out
    // backwards. Renaming a key is finished on 'change', which fires on blur.
    let keyBefore = null;
    on('.layer-prop', 'focus', el => { if (el.dataset.prop === 'key') keyBefore = selected().key; });
    on('.layer-prop', 'input', el => { selected()[el.dataset.prop] = el.value; draw(); });
    on('.layer-prop', 'change', el => {
      const l = selected();
      l[el.dataset.prop] = el.value;
      if (el.dataset.prop !== 'key') { draw(); return; }
      // Defaults are stored per key, so a rename has to carry its own with it
      // or the text silently detaches from the layer that showed it.
      const d = tpl.spec.defaults || {};
      if (keyBefore && keyBefore !== l.key && d[keyBefore] !== undefined && d[l.key] === undefined) {
        d[l.key] = d[keyBefore];
        delete d[keyBefore];
      }
      keyBefore = null;
      shell();
    });
    on('.layer-bool', 'change', el => { selected()[el.dataset.prop] = el.checked; draw(); });
    // Shape needs the whole panel back: the radius field only belongs to
    // "rounded", and the not-square warning only to "circle".
    on('.layer-shape', 'change', el => {
      const l = selected();
      l.shape = el.value;
      if (l.shape !== 'rounded') l.radius = 0;
      shell();
    });
    on('.tpl-default', 'input', el => {
      tpl.spec.defaults = tpl.spec.defaults || {};
      tpl.spec.defaults[el.dataset.key] = el.value;
      draw();
    });
    on('.layer-src', 'change', el => {
      const l = selected();
      const [kind, value] = [el.value.slice(0, 1), el.value.slice(2)];
      if (kind === 'b') { l.builtin = value; delete l.asset_id; }
      else { l.asset_id = value; delete l.builtin; }
      draw();
    });

    const artUpload = document.getElementById('layer-art-upload');
    if (artUpload) artUpload.addEventListener('change', async () => {
      try {
        const file = artUpload.files[0];
        if (!file) return;
        message('Uploading artwork…');
        const out = await uploadAsset('art', file);
        assets.items = [{ id: out.id, label: file.name, width: out.width, height: out.height }, ...(assets.items || [])];
        const l = selected();
        l.asset_id = out.id; delete l.builtin;
        message('Artwork added.');
        shell();
      } catch (e) { message(e.message, true); }
    });

    bindBoxDragging();
    bind('tpl-save', 'click', async e => busy(e.target, async () => {
      if (!tpl.name.trim()) throw Error('Give the template a name.');
      if (derived && !/[a-z0-9]/i.test(tpl.name)) {
        throw Error('The name needs a letter or a digit; it becomes this template\'s key.');
      }
      if (!tpl.family.trim()) throw Error('This template has no family key.');
      // Saving a family+size that already exists retires the old one. That is
      // how you replace a seeded template deliberately, and a nasty surprise
      // when two different names happen to slug the same way.
      if (derived && studio.families.has(tpl.family)) {
        throw Error(`"${tpl.family}" already exists. Pick a different name, or open that template `
          + `and use "Add a size" if you meant to extend it.`);
      }
      const out = await post('/admin/posters/templates', {
        id: tpl.id, family: tpl.family.trim(), name: tpl.name.trim(), size: tpl.size, spec: tpl.spec,
      });
      tpl.id = out.id;
      await loadTemplates();
      message('Template saved.');
      await posterHome();
    }));
  }

  function move(layerId, direction) {
    const i = tpl.spec.layers.findIndex(x => x.id === layerId);
    const j = i + direction;
    if (i < 0 || j < 0 || j >= tpl.spec.layers.length) return;
    [tpl.spec.layers[i], tpl.spec.layers[j]] = [tpl.spec.layers[j], tpl.spec.layers[i]];
    state.selected = layerId;
    shell();
  }

  function bindBoxDragging() {
    const canvas = document.getElementById('builder-canvas');
    if (!canvas) return;
    const toCanvas = e => {
      const r = canvas.getBoundingClientRect();
      return { x: (e.clientX - r.left) * (canvas.width / r.width), y: (e.clientY - r.top) * (canvas.height / r.height) };
    };
    const handleSize = () => 18 * (canvas.width / canvas.getBoundingClientRect().width) * 1.6;

    let mode = null, layer = null, last = null, ratio = 1;
    canvas.addEventListener('pointerdown', e => {
      const p = toCanvas(e);
      const grab = handleSize();
      // Topmost first, so a layer drawn over another is the one you grab.
      const target = tpl.spec.layers.slice().reverse()
        .find(l => p.x >= l.x - 4 && p.x <= l.x + l.w + 4 && p.y >= l.y - 4 && p.y <= l.y + l.h + 4);
      if (!target) return;
      const corner = p.x >= target.x + target.w - grab && p.y >= target.y + target.h - grab;
      mode = corner ? 'resize' : 'move';
      // Held from the grab, not recomputed per frame, or rounding walks the
      // proportions away while you drag.
      ratio = target.w / Math.max(1, target.h);
      layer = target; last = p;
      if (state.selected !== target.id) { state.selected = target.id; shell(); return; }
      canvas.setPointerCapture(e.pointerId);
      canvas.classList.add('grabbing');
    });
    canvas.addEventListener('pointermove', e => {
      if (!mode || !layer) return;
      const p = toCanvas(e);
      const dx = p.x - last.x, dy = p.y - last.y;
      if (mode === 'move') { layer.x += dx; layer.y += dy; }
      else if (e.shiftKey) {
        // Follow whichever axis moved more, so the corner tracks the pointer.
        layer.w = Math.max(16, layer.w + (Math.abs(dx) > Math.abs(dy) ? dx : dy * ratio));
        layer.h = Math.max(16, layer.w / ratio);
        if (layer.type === 'qr') layer.w = layer.h = Math.max(layer.w, layer.h);
      } else {
        layer.w = Math.max(16, layer.w + dx);
        layer.h = Math.max(16, layer.h + dy);
        if (layer.type === 'qr') layer.w = layer.h = Math.max(layer.w, layer.h);
      }
      last = p;
      draw();
    });
    const stop = e => {
      if (!mode) return;
      mode = null; layer = null;
      canvas.classList.remove('grabbing');
      if (e.pointerId !== undefined && canvas.hasPointerCapture(e.pointerId)) canvas.releasePointerCapture(e.pointerId);
      shell();
    };
    canvas.addEventListener('pointerup', stop);
    canvas.addEventListener('pointercancel', stop);
  }

  // Keys are bound to the document so they work wherever focus sits on the
  // screen, which means exactly one listener per builder: shell() runs on
  // every edit, and binding there would stack a handler per render and move a
  // layer further on each keypress. The controller is aborted when another
  // screen takes over.
  function bindKeys() {
    builderKeys?.abort();
    builderKeys = new AbortController();
    document.addEventListener('keydown', e => {
      const t = e.target;
      if (t && (t.isContentEditable || /^(INPUT|TEXTAREA|SELECT)$/.test(t.tagName))) return;
      const l = selected();
      if (!l) return;
      const step = e.shiftKey ? 10 : 1;
      const nudge = { ArrowLeft: [-step, 0], ArrowRight: [step, 0], ArrowUp: [0, -step], ArrowDown: [0, step] }[e.key];
      if (nudge) {
        e.preventDefault();
        l.x += nudge[0];
        l.y += nudge[1];
        draw();
        // The X/Y boxes are stale until the panel catches up, but redrawing it
        // per keypress would fight a held arrow key, so it waits for the key
        // to come back up.
        clearTimeout(nudgeSettle);
        nudgeSettle = setTimeout(shell, 250);
        return;
      }
      if (e.key === 'Delete' || e.key === 'Backspace') {
        e.preventDefault();
        if (tpl.spec.layers.length === 1) return message('A template needs at least one layer.', true);
        tpl.spec.layers = tpl.spec.layers.filter(x => x.id !== l.id);
        state.selected = tpl.spec.layers[tpl.spec.layers.length - 1].id;
        shell();
      }
    }, { signal: builderKeys.signal });
  }

  bindKeys();
  shell();
}
