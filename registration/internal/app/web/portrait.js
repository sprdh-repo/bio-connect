'use strict';
// Portrait cropper for the mobile content editor. Turns any photo into the
// website's 480 × 600 speaker portrait: the person frames it by dragging and
// zooming inside a 4:5 window, and the forest-green duotone that
// scripts/portrait.py gives every speaker photo is applied in the browser, so
// uploads match the existing set without any server-side image work.

const PORTRAIT_W = 480, PORTRAIT_H = 600;
// Same stops as scripts/portrait.py, so new and existing portraits read as one set.
const PORTRAIT_STOPS = [[0, [5, 28, 22]], [0.55, [118, 150, 122]], [1, [232, 238, 216]]];
const portraitLUT = (() => {
  const lut = new Uint8ClampedArray(256 * 3);
  for (let i = 0; i < 256; i++) {
    const t = i / 255;
    for (let s = 0; s < PORTRAIT_STOPS.length - 1; s++) {
      const [a, ca] = PORTRAIT_STOPS[s], [b, cb] = PORTRAIT_STOPS[s + 1];
      if (t < a || t > b) continue;
      const k = (t - a) / (b - a);
      for (let c = 0; c < 3; c++) lut[i * 3 + c] = ca[c] * (1 - k) + cb[c] * k;
    }
  }
  return lut;
})();

// Greyscale, stretch the 1st-99th percentile to full range, then map through
// the duotone gradient - the same steps as portrait.py's duotone().
function portraitDuotone(ctx) {
  const img = ctx.getImageData(0, 0, PORTRAIT_W, PORTRAIT_H), d = img.data, n = d.length / 4;
  const grey = new Uint8Array(n), hist = new Uint32Array(256);
  for (let i = 0, p = 0; i < n; i++, p += 4) {
    const g = (d[p] * 299 + d[p + 1] * 587 + d[p + 2] * 114) / 1000 | 0;
    grey[i] = g; hist[g]++;
  }
  const at = q => { let sum = 0; for (let v = 0; v < 256; v++) if ((sum += hist[v]) >= q * n) return v; return 255; };
  const lo = at(0.01), span = Math.max(at(0.99) - lo, 1);
  for (let i = 0, p = 0; i < n; i++, p += 4) {
    const g = Math.min(255, Math.max(0, (grey[i] - lo) / span * 255)) | 0;
    d[p] = portraitLUT[g * 3]; d[p + 1] = portraitLUT[g * 3 + 1]; d[p + 2] = portraitLUT[g * 3 + 2]; d[p + 3] = 255;
  }
  ctx.putImageData(img, 0, 0);
}

function portraitBlob(canvas) {
  // Safari cannot encode WebP and silently returns PNG; JPEG is the fallback
  // the server accepts, and the website build converts it to WebP.
  const encode = (type, q) => new Promise(done => canvas.toBlob(done, type, q));
  return encode('image/webp', 0.85).then(b => b?.type === 'image/webp' ? b : encode('image/jpeg', 0.88));
}

// cropPortrait opens the cropper for a picked file. It resolves to the
// finished portrait Blob, or null when the person cancels.
async function cropPortrait(file) {
  let source;
  try { source = await createImageBitmap(file, { imageOrientation: 'from-image' }); } catch { throw Error('This file is not an image the browser can open. Use a JPEG, PNG or WebP photo.'); }
  if (source.width < 240 || source.height < 300) throw Error('This photo is too small. Use one at least 480 × 600 pixels for a sharp portrait.');

  const dialog = document.createElement('dialog');
  dialog.className = 'portrait-dialog';
  dialog.setAttribute('aria-labelledby', 'portrait-title');
  dialog.innerHTML = `<h2 id="portrait-title">Frame the portrait</h2>
    <p class="help">Drag to move the photo and use the slider to zoom. Fit the face inside the guide.</p>
    <div class="portrait-stage"><canvas class="portrait-canvas" width="${PORTRAIT_W}" height="${PORTRAIT_H}" tabindex="0" aria-label="Portrait preview. Arrow keys move the photo, plus and minus zoom."></canvas><span class="portrait-guide" aria-hidden="true"></span></div>
    <label class="portrait-zoom">Zoom<input type="range" min="1" max="4" step="0.01" value="1"></label>
    <label class="check"><input type="checkbox" checked> Apply the Bio Connect green tone</label>
    <div class="actions"><button type="button" class="secondary" data-portrait-cancel>Cancel</button><button type="button" data-portrait-use>Use portrait</button></div>`;
  document.body.append(dialog);
  const canvas = dialog.querySelector('canvas'), ctx = canvas.getContext('2d', { willReadFrequently: true });
  const zoomInput = dialog.querySelector('[type=range]'), tone = dialog.querySelector('[type=checkbox]');
  ctx.imageSmoothingQuality = 'high';

  // The photo always covers the frame. x, y place its top-left corner in
  // portrait pixels; zoom multiplies the smallest covering scale.
  const cover = Math.max(PORTRAIT_W / source.width, PORTRAIT_H / source.height);
  let zoom = 1, x = 0, y = 0;
  const size = () => [source.width * cover * zoom, source.height * cover * zoom];
  const clamp = () => {
    const [w, h] = size();
    x = Math.min(0, Math.max(PORTRAIT_W - w, x));
    y = Math.min(0, Math.max(PORTRAIT_H - h, y));
  };
  // Start centred across, and high up the photo, where a headshot's face usually is.
  { const [w, h] = size(); x = (PORTRAIT_W - w) / 2; y = (PORTRAIT_H - h) * 0.2; clamp(); }

  let frame = 0;
  const draw = () => {
    frame = 0;
    const [w, h] = size();
    ctx.fillStyle = '#fff';
    ctx.fillRect(0, 0, PORTRAIT_W, PORTRAIT_H);
    ctx.drawImage(source, x, y, w, h);
    if (tone.checked) portraitDuotone(ctx);
  };
  const redraw = () => { frame ||= requestAnimationFrame(draw); };
  // Zoom about a point in portrait pixels, so the face under it stays put.
  const zoomTo = (next, cx = PORTRAIT_W / 2, cy = PORTRAIT_H / 2) => {
    next = Math.min(4, Math.max(1, next));
    const k = next / zoom;
    x = cx - (cx - x) * k; y = cy - (cy - y) * k; zoom = next;
    zoomInput.value = String(zoom);
    clamp(); redraw();
  };
  const toPortrait = e => {
    const r = canvas.getBoundingClientRect();
    return [(e.clientX - r.left) * PORTRAIT_W / r.width, (e.clientY - r.top) * PORTRAIT_H / r.height];
  };

  let drag = null;
  canvas.addEventListener('pointerdown', e => { canvas.setPointerCapture(e.pointerId); drag = toPortrait(e); });
  canvas.addEventListener('pointermove', e => {
    if (!drag) return;
    const [px, py] = toPortrait(e);
    x += px - drag[0]; y += py - drag[1]; drag = [px, py];
    clamp(); redraw();
  });
  canvas.addEventListener('pointerup', () => { drag = null; });
  canvas.addEventListener('pointercancel', () => { drag = null; });
  canvas.addEventListener('wheel', e => { e.preventDefault(); zoomTo(zoom * Math.exp(-e.deltaY / 400), ...toPortrait(e)); }, { passive: false });
  canvas.addEventListener('keydown', e => {
    const step = e.shiftKey ? 40 : 10, move = { ArrowLeft: [step, 0], ArrowRight: [-step, 0], ArrowUp: [0, step], ArrowDown: [0, -step] }[e.key];
    if (move) { e.preventDefault(); x += move[0]; y += move[1]; clamp(); redraw(); }
    else if (e.key === '+' || e.key === '=') { e.preventDefault(); zoomTo(zoom * 1.1); }
    else if (e.key === '-') { e.preventDefault(); zoomTo(zoom / 1.1); }
  });
  zoomInput.addEventListener('input', () => zoomTo(+zoomInput.value));
  tone.addEventListener('change', redraw);

  draw();
  dialog.showModal();
  canvas.focus();
  try {
    return await new Promise(resolve => {
      dialog.addEventListener('close', () => resolve(null));
      dialog.querySelector('[data-portrait-cancel]').addEventListener('click', () => dialog.close());
      dialog.querySelector('[data-portrait-use]').addEventListener('click', async e => {
        if (frame) { cancelAnimationFrame(frame); draw(); }
        e.target.disabled = true;
        resolve(await portraitBlob(canvas));
      });
    });
  } finally {
    source.close();
    if (dialog.open) dialog.close();
    dialog.remove();
  }
}
