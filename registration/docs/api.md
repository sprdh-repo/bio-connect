# API contract

Base path `/api/v1`. Requests and responses are JSON unless noted.
Errors are `{"error": "<message>"}` with a 4xx or 5xx status.
All amounts are integer paise (1 INR = 100 paise). Times are RFC 3339; business dates use `Asia/Kolkata`.

Every response carries `Cache-Control: no-store`, `X-Content-Type-Options: nosniff`, a strict `Content-Security-Policy`, and `X-Frame-Options: DENY`.
State-changing `/api/v1` requests with a foreign `Origin` header are rejected with 403.

Amounts, roster counts, category state, and every registration state transition are enforced on the server; the client cannot override them.

## Public

### `GET /categories`

```json
{
  "categories": [
    {"id":"student","kind":"delegate","label":"Students",
     "early_paise":100000,"regular_paise":150000,"roster_count":1,"open":true,
     "payable_paise":100000}
  ],
  "registration_enabled": false,
  "server_time": "2026-09-07T18:00:00+05:30",
  "cutoff": "2026-10-01T00:00:00+05:30",
  "sbi_url": ""
}
```

`payable_paise` is the fee for the current server date.
It flips from `early_paise` to `regular_paise` at `cutoff` (1 October 2026, 00:00 IST).

### `POST /registrations`

Header `Idempotency-Key: <32-128 chars>` is required.
Retrying with the same key and an identical body returns the original registration with `"replayed": true` and no new token.
Reusing the key with a different body is rejected.

```json
{
  "category_id": "premium",
  "institution": "Biotech Labs Pvt Ltd",
  "contact_name": "Ravi Menon",
  "email": "ravi@example.com",
  "phone": "+919812345678",
  "description": "Molecular diagnostics",
  "attendees": [
    {"name":"Rep 0","email":"rep0@example.com","phone":"+919812345670",
     "designation":"Scientist","whatsapp_consent":true}
  ]
}
```

- Delegates: `attendees` must hold exactly one person; contact fields are taken from that person.
- Exhibitors: `attendees` must hold exactly 3 (premium), 2 (standard), or 2 (table space); `description` is required. The contact person receives a pass only if listed among the attendees.
- `phone` must be `+` and 8 to 15 digits. `whatsapp_consent` records messaging permission; without it WhatsApp is skipped for that person.
- Fails with 400 if registration is closed or the category is closed.

Response `201`:

```json
{"id":"<rid>","management_token":"<43 chars>","replayed":false,
 "manage_url":"https://reg.bioconnect.kerala.gov.in/manage/<rid>#<token>"}
```

The `management_token` is shown once. It grants registration-management access only, never pass download.
The registration is given the next reference in its category's series (`BC4-EX-0007`); the series is allocated inside the creating transaction, so an abandoned registration leaves no gap.

### Registration management

All routes require `Authorization: Bearer <management_token>` scoped to that `{id}`.

| Route | Notes |
|---|---|
| `GET /registrations/{id}` | `{registration, category, payments, files, sbi_url, reply_to}` |
| `POST /registrations/{id}/files?kind=logo\|receipt` | raw body, `Content-Type: application/octet-stream`, <= 5 MB. Logos: PNG/JPEG. Receipts: PDF/PNG/JPEG. File type is validated by content, not extension. `201 {"id":"<fid>"}` |
| `POST /registrations/{id}/payments` | see below |
| `GET /registrations/{id}/files/{file}` | returns the uploaded file (302 to a 60-second S3 URL in production) |

Uploads are only accepted while the registration is `awaiting_payment` or `correction_requested`.

`POST /registrations/{id}/payments`:

```json
{"reference":"SBICR000123","date":"2026-09-20","amount_paise":600000}
```

Requires `reference` and `date` (and a `logo` upload for exhibitors).
`amount_paise` is required; the form prefills it with the current fee.
`receipt_id` is optional: pass one from a prior `kind=receipt` upload to attach the SBI receipt, or omit it.
Moves the registration to `awaiting_review` and appends to the payment history.
Submitting evidence, uploading a receipt, or a browser redirect never approves payment on its own.

### Recovery (no account)

- `POST /recovery` `{"email":"..."}` -> always `202 {"message": "..."}`. If registrations match, a 20-minute single-use link is emailed for each.
- `POST /recovery/exchange` `{"token":"..."}` -> `200 {"id":"<rid>","management_token":"<new>"}`. Issues a fresh management token and invalidates the previous one. The link cannot be reused.

### Passes

Token-in-path, no other credential. Tokens are opaque and per pass.
The pass number printed on the PDF is its registration's reference plus the holder's place in it (`BC4-EX-0007-3`).
It identifies a pass to staff and grants nothing on its own; it is not accepted by these routes.

- `GET /passes/{token}` -> `application/pdf` for one attendee's current pass.
- `GET /passes/pack/{token}` -> `application/zip` of every active pass for an exhibitor registration.

Revoked passes and non-approved registrations return 404.

## Staff console (`/api/v1/admin`)

Authentication is a server session cookie (`bc_session`) set by `POST /api/v1/auth/login` with `{email, password, code}` (TOTP).
A TOTP code cannot be replayed within its 30-second window.
Non-GET requests must send `X-CSRF-Token` matching the `bc_csrf` cookie.
Login is throttled per IP and per email.

Roles are disjoint:

- `manager`: `GET/POST /admin/staff` only. Creating an account returns `201 {"totp_uri":"otpauth://..."}`. Editing (`{id, role, active}`) revokes that account's sessions.
- `reviewer`: everything else below, except the poster routes.
- Both roles reach `/admin/posters/*`. A poster is neither registration data nor account data, and the person making a speaker reveal is as likely to hold either account. Saves are still attributed to the staff id in the audit trail.

| Route | Purpose |
|---|---|
| `GET /admin/me` | `{id, role}` |
| `POST /admin/logout` | clears the session |
| `GET /admin/registrations?q=&category=&status=&from=&to=&page=` | paginated list (25/page); `from`/`to` are `YYYY-MM-DD`. `q` matches reference, institution, contact, attendee name/email, and pass number; references and pass numbers match case-insensitively with the hyphens optional |
| `GET /admin/summary` | `{categories:[{id, kind, label, registered, confirmed}]}` in category order, event-wide (ignores list filters). `registered` excludes `rejected` and `cancelled`; `confirmed` counts `approved` |
| `GET /admin/registrations/{id}` | full record: registration, category, payments, files, passes, deliveries, audit trail |
| `POST /admin/registrations/{id}/review` | state transitions, see below |
| `GET /admin/registrations/{id}/files/{file}` | private receipt/logo download |
| `GET /admin/export?format=csv\|xlsx&sheet=&<same filters>` | CSV is one `sheet` (`Registrations`, `Attendees`, `Payments`, `Deliveries`); XLSX has all four. Cells that begin with `= + - @` are prefixed with `'`. 50,000-row cap. |
| `POST /admin/categories` | `{id, open}` opens or closes a category |
| `POST /admin/bulk-send` | `{ids:[...], channel:""}` runs `send` for 1-100 approved registrations; returns per-id `queued` or the error |
| `POST /admin/bulk-remind` | `{ids:[...]}` runs `payment_reminder` for 1-100 registrations; returns per-id `queued` or the error |
| `POST /admin/retry` | `{id, confirm_uncertain, note}` requeues a `failed` or `uncertain` delivery. `uncertain` needs `confirm_uncertain:true` and a `note`. The prior job and its provider id are kept for late webhooks. |

### Social posters (`/admin/posters`)

Open to both roles. Rendering happens in the browser on a canvas at true output
resolution; these routes store the definitions and the artwork and never produce
an image, which is why there is no image encoder in `go.mod`.

| Route | Purpose |
|---|---|
| `GET /admin/posters/templates` | `{items, builtin_logos, sizes}`. Each item carries `spec` (see below) and `origin` (`seed` or `staff`); `sizes` maps `4x5`/`1x1`/`9x16` to pixel dimensions. |
| `POST /admin/posters/templates` | `{id?, family, name, size, spec}`. Omitting `id` creates a template and retires the family's previous active template at that size, so relaying out a size is one step. `spec` is validated in full and rejected with the offending rule. **Saving with an `id` sets `origin='staff'`**, which is what stops a later `poster-seed --replace` reverting the edit. |
| `POST /admin/posters/templates/duplicate` | `{family, name}` copies every active size of `family` into a new one whose slug comes from `name`. The copy is `origin='staff'` and points at the same artwork rows - nothing mutates an asset, so sharing them avoids duplicating megabytes. 409 if the name is taken, 404 if the source has no active sizes. |
| `POST /admin/posters/templates/retire` | `{family, size?}` sets `active=false` for that family, or for one size of it. The rows stay: posters made from the template keep their values, and the audit trail keeps its history. |
| `GET /admin/posters/assets?kind=art\|photo\|logo` | uploaded artwork, newest first, 200 max |
| `POST /admin/posters/assets?kind=&label=` | raw-body PNG/JPEG upload, 8 MB, ≤8000x8000 and ≤32 MP (a 4500x4500 print export is 20.25 MP, so the registration path's 20 MP ceiling was too low). Each limit reports itself by name. Returns `{id, width, height, mime}`. The larger cap applies here only; registration uploads stay at 5 MB. |
| `GET /admin/posters/assets/{id}` | **the image bytes inline, never a redirect.** The editor draws these into a canvas and reads it back with `toDataURL`; a redirect to a presigned S3 URL would taint that canvas and break every export. |
| `GET /admin/posters/qr?data=` | `image/png` QR, same encoder and error-correction level as the passes. `data` must be an `https://` URL of at most 512 characters. |
| `GET /admin/posters` | saved posters, newest first, 100 max |
| `GET /admin/posters/{id}` | `{id, family, title, content}` to reopen for editing |
| `POST /admin/posters` | `{id?, family, title, content}`; `content` is `{values, transforms}` |

A template `spec` is one ordered `layers` array plus optional `background`
(`#rrggbb`) and `defaults`. **Array order is draw order**, which is how a
decoration sits above the portrait and a caption above that; there is no separate
background or overlay concept. At most 40 layers.

| Layer `type` | Fields |
|---|---|
| `art` | exactly one of `asset_id` (uploaded) or `builtin` (an embedded brand logo: `bio-connect`, `bio-connect-mark`, `bio360`, `ksidc`, `klip`, `invest-kerala`). The Government of Kerala emblem is not embedded: the site's copy is CC BY-SA 4.0 and a poster cannot carry the attribution. Upload the official emblem as artwork instead. |
| `photo` | `key`, `fit` (`cover`/`contain`), `shape` (`rect`/`rounded`/`circle`/`arch`), `radius` (rounded only), `duotone`. `contain` is also the partner-logo slot, so there is no separate type. An absent `shape` with `radius` set is the pre-shape form the seeded circular slots use, and still renders as a rounded rect. |
| `text` | `key`, `font` (`display`=Manrope / `body`=DM Sans / `serif`=Fraunces / `condensed`=Archivo Narrow), `weight` (400/500/600/700), `size`, `color`, `align`, `transform` (`none`/`upper`), `tracking`, `line_height`, `autofit`. The spec stores the role, not the face; each one needs an `@font-face` in `web/fonts/fonts.css` or the export silently falls back to a system font. |
| `qr` | `key`; must be square |

Every layer has `id`, `x`, `y`, `w`, `h` and an optional `label`. Keyed layers name
a field the poster fills; a `family` groups the sizes that share field names, so a
poster is filled once and exported at each size. `content.transforms` holds the
Layers are keyed by `key`, and two layers sharing one draw the same content and
share a default.
That is deliberate - a name can appear twice on a poster - but the builder hands
every new layer its own free key so it is never what you get by accident.

photo framing per key per size, because the same portrait needs a different crop at
4:5 and at 9:16.

### `POST /admin/registrations/{id}/review`

```json
{"action":"approve_send",
 "payment_id":"<latest submission id>",
 "verified_reference":"SBICR000123","verified_date":"2026-09-20","verified_amount_paise":600000,
 "successful":true,"beneficiary_confirmed":true,
 "note":"verified against bank statement"}
```

| `action` | Effect |
|---|---|
| `approve_send` | verifies the payment, approves, atomically creates one pass per attendee and the pack (exhibitors), then queues delivery. Requires `successful` and `beneficiary_confirmed`. `verified_amount_paise` must equal the category fee for `verified_date`. `verified_reference` must not already belong to an approved payment. Idempotent once approved. |
| `approve_only` | as above without queueing delivery |
| `record_approve_send` | from `awaiting_payment`, records a payment found directly in SBI as both reported and verified, then approves and queues delivery. Takes the same verified payment fields and confirmation flags as `approve_send`; no `payment_id` is required. Exhibitors must already have an institution logo. The payment and reviewer action are recorded atomically in the payment history and audit trail. |
| `record_approve_only` | as above without queueing delivery |
| `send` | queues the initial delivery for an approved registration; repeats are no-ops |
| `resend` | needs a unique `request_id`; queues a fresh delivery of the existing passes |
| `reissue` | `{pass_id, note}`; revokes that pass, its QR and its pass number, issues the next version at the next free place in the registration (`...-4` after a roster of 3), queues delivery |
| `payment_reminder` | from `awaiting_payment` only; emails the contact the reference, the fee payable today, and a single-use `/recover#<token>` link valid for 7 days. The existing management link keeps working until that link is used. At most one reminder per registration per 24 hours. A queued reminder is cancelled if the registration has left `awaiting_payment` by the time it is sent. |
| `correction_requested` / `rejected` | `{note}` required; only from `awaiting_review` |
| `cancelled` | `{note}` required; revokes all passes and cancels queued pass/pack jobs |

## Webhooks

- `GET /api/v1/webhooks/meta` -> echoes `hub.challenge` when `hub.verify_token` matches.
- `POST /api/v1/webhooks/meta` -> requires a valid `X-Hub-Signature-256` (HMAC-SHA256 of the raw body with the Meta app secret). Message statuses update the matching delivery record.
- `POST /api/v1/webhooks/postmark` -> HTTP Basic auth against the configured webhook credentials. `Delivery` and `Bounce` events for `Metadata.application == "bioconnect4"` update the matching delivery record.

Events are de-duplicated; replaying a webhook does not change the outcome or create extra rows.
A delivery already marked `delivered` or `failed` is never downgraded.

## Health

`GET /healthz` -> `200 {"status":"ok"}` when the database is reachable, otherwise `503`.

### Shipped templates

24 templates ride in the binary - four post types (speaker reveal, session
announce, countdown, partner welcome), each in a light and a dark look, each at
`4x5`, `1x1` and `9x16`. They are installed by the CLI, not by a migration and
not on boot:

```sh
bioconnect poster-seed            # creates what is missing, keeps what exists
bioconnect poster-seed --replace  # rolls out changed artwork
```

Each look is its own `family` (`speaker-reveal-light`, `speaker-reveal-dark`)
because of the `UNIQUE (family, size) WHERE active` index; the studio lists them
as separate families. Without `--replace` the seeder leaves any existing
template alone, so a deploy never reverts a layout staff changed in the builder.
Seeded assets carry a `seed:<label>:<sha256 prefix>` label, which is how
unchanged artwork is reused rather than uploaded again.

`poster_templates.origin` records who owns a layout. The seeder inserts `seed`;
saving any edit in the console, and every duplicate, sets `staff`. `--replace`
skips `staff` rows and reports them as `kept (edited)`, so rolling out new
artwork can never revert a template staff have customised. Duplicate is the
intended way to start from a shipped template and keep the original updating.
