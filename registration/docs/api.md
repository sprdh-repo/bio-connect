# API contract

Base path `/api/v1`. Requests and responses are JSON unless noted.
Errors are `{"error": "<message>"}` with a 4xx or 5xx status.
All amounts are integer paise (1 INR = 100 paise). Times are RFC 3339; business dates use `Asia/Kolkata`.

Every response carries `Cache-Control: no-store`, `X-Content-Type-Options: nosniff`, a strict `Content-Security-Policy`, and `X-Frame-Options: DENY`.
State-changing `/api/v1` requests with a foreign `Origin` header are rejected with 403.

Amounts, roster counts, category state, and every registration state transition are enforced on the server; the client cannot override them.

## Mobile attendee passes

Email and WhatsApp OTP, optional admission QR matching, and identity-scoped pass retrieval are documented in [My passes](../../mobile/docs/my-passes.md).
The endpoints are `POST /mobile/pass-access/challenges`, `POST /mobile/pass-access/verify`, and authenticated `GET`/`DELETE /mobile/passes`.
These sessions are separate from staff and registration-management access.

Each pass in `GET /mobile/passes` carries `share_email` and `share_phone`, the holder's consent to give these to people who scan their badge in the app.
Both are `false` until the holder turns them on.
`PUT /mobile/passes/{id}/sharing` with `{share_email, share_phone}` and the same bearer token changes them for one pass.
It answers 404 for a pass outside the verified identity, so a holder can never consent for a colleague on the same stall.

## Public

### `GET /public/app-content`

Returns everything the mobile app shows, as staff publish it from the console's Mobile app editor (`/admin?view=mobile`).
The document includes `event`, `themes`, `programme_highlights`, `product_launch`, `leadership`, `sponsors_intro`, `sponsors`, `sponsor`, `ecosystem_partners`, `menus`, `copy`, `event_guide`, and the published `speakers` array.

- `event` adds `privacy_url`; `product_launch` adds `apply_label`; partners add an optional `website_url`; `event_guide.venue` adds `help_whatsapp`.
- `sponsors` lists every visible sponsor as `{name, category, description, logo_url, website_url}`. `sponsor` repeats the first one (or blank strings) for app versions released before the list.
- `leadership` holds `intro`, `advisory_note`, `convened_by {name, note}` (or `null`), `people` (`name`, `role`, `badge`, optional `image_url`, `image_credit`, `image_credit_url`) and `committee {title, order_note, members[{role, name, organization}]}`.
- `menus` maps `tabs`, `home_shortcuts`, `home_links` and `guide` to ordered `[{key, title, subtitle, url}]`. `key` is an app destination (`sessions`, `speakers`, `agenda`, `contacts`, `feedback`, `hub`, `venue`, `activities`, `faqs`, `exhibitors`, `my_passes`, `registration`, `brochure`, `product_launch`, `sponsors`, `leadership`, `explore`, `privacy`) or `link`, which opens `url` (`https:`, `mailto:` or `tel:`). Only `sessions`, `speakers` and `agenda` can be tabs; Home and Guide always are. A blank title keeps the app's label.
- `feedback` is `{open, intro}`. The app accepts ratings only while `open` is true, and hides the `feedback` destination otherwise.
- Each `event_guide.sessions` entry adds `speaker_ids`, the speaker directory IDs staff linked to it (an empty list when none). The free-text `speakers` line is unchanged.
- `copy` maps keys such as `sessions.title` to staff wording for the app's headings. A missing key keeps the built-in text.

Staff visibility controls never reach the app: list entries switched off are omitted, and fields staff hide are sent as an empty string (an empty list, or `null` for `convened_by`), so released app versions still find every key they require.
The app leaves empty values out instead of showing "to be announced", and drops menu entries whose page would be empty.
Speaker rows are injected from the speaker directory at request time so both endpoints share one source of truth.

The endpoint permits anonymous cross-origin reads. If the content document is unpublished, it returns 503 and the app retains its bundled offline snapshot.

### `GET /public/exhibitors`

Returns approved exhibitors, ordered by name, without contact, payment or attendee data:

```json
{
  "exhibitors": [{
    "id": "BC4-EX-0007",
    "name": "Biotech Labs Pvt Ltd",
    "description": "Molecular diagnostics",
    "logo_url": "/api/v1/public/exhibitors/logos/5c74c0f6a86b38b82962912af36e8931",
    "stall_number": "B-12",
    "stall_type": "Standard stall"
  }]
}
```

`id` is the registration reference, already printed on the exhibitor's passes; the app uses it to keep a saved exhibitor across refreshes.
`logo_url` is empty when no logo was uploaded, and `stall_number` is empty until staff allocate one.
Logo URLs are relative and re-check approval on every request.

### `GET /public/speakers`

Returns the published speaker directory in its curated display order:

```json
{
  "speakers": [{
    "id": "jayakrishna-ambati",
    "name": "Dr. Jayakrishna Ambati",
    "role": "Center Director & Professor",
    "organization": "University of Virginia Health System",
    "image_url": "https://bioconnect.kerala.gov.in/assets/speakers/jayakrishna-ambati.webp",
    "linkedin": ""
  }],
  "version": "9a204980a14f84bf"
}
```

The endpoint permits anonymous cross-origin reads. Unpublished rows are omitted.
`version` fingerprints the list and changes only when a published speaker changes.
The website stamps it into `speakers.html` at deploy time and re-renders its cards only when the live version differs.
Responses carry `Cache-Control: public, max-age=60`.

### `GET /public/images/{sha256}.webp|.jpg`

Serves a portrait uploaded through the mobile app editor.
The file name is the image's SHA-256, so the response never changes: it carries `Cache-Control: public, max-age=31536000, immutable` and anonymous cross-origin access.

### `GET /public/badges/{qr_id}`

Resolves the QR on a badge or pass (the `/p/<qr_id>` URL reduces to `qr_id`) for the app's contact exchange:

```json
{"qr_id": "…", "name": "Asha Nair", "designation": "CTO", "institution": "Helix Labs", "category": "Industry", "email": "", "phone": ""}
```

The first five fields are what the badge prints.
`email` and `phone` are filled only when the holder consented from My passes, and are empty strings otherwise.
Unknown, revoked or removed badges answer 404.
Responses are `no-store` and rate limited per address (3,000 an hour, since a venue network shares one address).

### `POST /public/feedback`

`{device_id, kind, session_id, rating, comment}` records an anonymous rating while `feedback.open` is true (409 otherwise).
`kind` is `event` (with an empty `session_id`) or `session` (with the ID of a published session; 404 otherwise).
`rating` is 1-5 and `comment` up to 2,000 characters.
`device_id` is a random 16-64 character value the app keeps per install; a second answer from it for the same target replaces the first.
Rate limited per address.

### `GET /categories`

```json
{
  "categories": [
    {"id":"student","kind":"delegate","label":"Students",
     "early_paise":100000,"regular_paise":150000,"roster_count":1,"open":true,"free_only":false,"free_open":false,
     "payable_paise":100000,"coupon_eligible":false}
  ],
  "registration_enabled": false,
  "server_time": "2026-09-07T18:00:00+05:30",
  "cutoff": "2026-10-01T00:00:00+05:30",
  "sbi_url": ""
}
```

`payable_paise` is the fee for the current server date.
It flips from `early_paise` to `regular_paise` at `cutoff` (1 October 2026, 00:00 IST).
`coupon_eligible` says whether the form should offer a coupon field for that category; codes and exclusions stay on the server.

### `POST /coupons`

```json
{"code":"ksum30","category_id":"startup"}
```

Response `200`: `{"code":"KSUM30","percent_off":30,"payable_paise":245000}`, the offer price for the current server date.
Codes are case-insensitive.
An unknown code, or one that does not apply to the category, fails with 400.
Rate-limited per client to 60 checks an hour.
This only previews the price; `POST /registrations` validates the code again.

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
  "coupon_code": "",
  "attendees": [
    {"name":"Rep 0","email":"rep0@example.com","phone":"+919812345670",
     "designation":"Scientist","whatsapp_consent":true}
  ]
}
```

- Delegates: `attendees` must hold exactly one person; contact fields are taken from that person.
- Exhibitors: `attendees` must hold exactly 5 (premium), 3 (standard), or 2 (table space); `description` is required. The contact person receives a pass only if listed among the attendees.
  The allowance is recorded on the registration as `roster_count`. It rose from 3 / 2 / 2 in migration 009, which granted it to existing registrations too, so an older registration can hold fewer attendees than its allowance until the places are filled (see `POST /registrations/{id}/attendees`).
- `phone` must be `+` and 8 to 15 digits. `whatsapp_consent` records messaging permission; without it WhatsApp is skipped for that person.
- `coupon_code` is optional. A valid code freezes its discount on the registration (`coupon_code`, `discount_percent`); an invalid one fails with 400 and saves nothing.
  `KSUM30` gives 30% and `KMTC25` gives 25% off every category except students, applied to whichever fee (early-bird or regular) is current on the verified payment date, rounded to the nearest rupee.
  A coupon registration pays by direct bank transfer instead of SBI Collect.
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
| `GET /registrations/{id}` | `{registration, category, payable_paise, payments, files, sbi_url, reply_to}`, plus `bank_transfer` (`account_name`, `bank`, `branch`, `account_number`, `ifsc`) for a coupon registration. `payable_paise` is what this registration owes today, after any discount. |
| `POST /registrations/{id}/files?kind=logo\|receipt` | raw body, `Content-Type: application/octet-stream`, <= 5 MB. Logos: PNG/JPEG. Receipts: PDF/PNG/JPEG. File type is validated by content, not extension. `201 {"id":"<fid>"}` |
| `POST /registrations/{id}/payments` | see below |
| `GET /registrations/{id}/files/{file}` | returns the uploaded file (302 to a 60-second S3 URL in production) |
| `POST /registrations/{id}/attendees` | see below |

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

`POST /registrations/{id}/attendees` fills one unassigned pass on an exhibitor registration:

```json
{"name":"Meera Pillai","designation":"Engineer","email":"meera@example.com","phone":"+919812345679","whatsapp_consent":true}
```

- 409 if every pass is assigned, the person's email is already on the registration, or the registration is `rejected` or `cancelled`. Delegate registrations have no additional passes.
- Before approval the person joins the roster, and approval issues their pass with the rest.
- After approval their pass is issued at once at the next free place (`BC4-EX-0007-4`). If the team's passes have already been sent, it is delivered to them alone and the contact receives a refreshed pack; nobody else is sent anything. After `approve_only`, the next `send` delivers it with the others.
- `GET /registrations/{id}` returns `registration.roster_count` alongside `registration.attendees`, so the page can show how many passes are unassigned.

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
Staff sessions and both authentication cookies expire 3 days (72 hours) after sign-in, without automatic extension.
A TOTP code cannot be replayed within its 30-second window.
Non-GET requests must send `X-CSRF-Token` matching the `bc_csrf` cookie.
Login is throttled per IP and per email.

Roles are disjoint:

- `manager`: `GET/POST /admin/staff` only. Creating an account returns `201 {"totp_uri":"otpauth://..."}`. Editing (`{id, role, active}`) revokes that account's sessions.
- `reviewer`: everything else below, except the poster routes.
- Both roles reach `/admin/mobile-content`, `/admin/feedback` and `/admin/posters/*`. A poster is neither registration data nor account data, and the person making a speaker reveal is as likely to hold either account. Saves are still attributed to the staff id in the audit trail.

| Route | Purpose |
|---|---|
| `GET /admin/me` | `{id, role}` |
| `POST /admin/content-images` | Both roles. The raw bytes of a speaker portrait, a WebP or JPEG of exactly 480 x 600 up to 2 MB (the editor crops and tones it in the browser). Stored once per content hash; returns `{url}` under `/public/images/`. Speaker `image_url` accepts these URLs as well as public HTTPS links |
| `GET/PUT /admin/mobile-content` | Both roles. Session `speaker_ids` must name speakers in the saved directory. The mobile app editor: `{revision, content, guide, speakers}`, where `content` is the stored app document (list entries carry `published`; `event`, `product_launch` and `leadership` carry `hidden`, the field names withheld from the app), `guide` is the event guide (`sessions`, `activities`, `faqs`, `venue`, the venue also with `hidden`), and `speakers` is the ordered directory `[{id, name, role, organization, image_url, linkedin, published}]`. `PUT` replaces all three atomically, returns the saved editor, and answers 409 when `revision` is stale. Links must be public HTTPS; up to 2 MB. Audited as `mobile_content.update` |
| `GET /admin/feedback` | Both roles. `{open, summary, responses}`: `summary` is one row per rated target (`kind`, `session_id`, `title`, `responses`, `average` to one decimal, `ratings` counted by star), the event first and then programme order; `responses` lists every answer newest first. `?format=csv` downloads the responses with formula-injection protection |
| `POST /admin/logout` | clears the session |
| `GET /admin/registrations?q=&category=&status=&from=&to=&page=&page_size=` | `{items, page, page_size, pages, total, status_counts}`, newest first. `page_size` is 25 (default), 50 or 100. `status_counts` maps each status to its count under every filter except `status`, so it stays stable while switching status; `total` is the count under all filters. `from`/`to` are `YYYY-MM-DD`. `q` matches reference, institution, contact, attendee name/email, and pass number; references and pass numbers match case-insensitively with the hyphens optional |
| `GET /admin/summary` | `{categories:[{id, kind, label, registered, confirmed, passes}]}` in category order, event-wide (ignores list filters). `registered` excludes `rejected` and `cancelled`; `confirmed` counts `approved`; `passes` counts the active (unrevoked) passes on confirmed registrations |
| `POST /admin/registrations` (`Idempotency-Key` header) | staff registration in one step for any delegate or exhibitor category: JSON, or multipart with the JSON in `payload` and an optional `logo`. Body is the public registration fields plus `payment` (`paid` or `complimentary`), for `paid` the `verified_reference`, `verified_date` and `verified_amount_paise` (must equal the fee for that date, less any `coupon_code`), `send` and `note`. Returns `201 {id}`. The registration is approved at once, whether or not registration or the category is open; phones are optional (WhatsApp delivery still needs one); an exhibitor needs at least one attendee and may leave its other places to fill later; the logo is optional. Nothing is emailed unless `send` is true, which delivers the passes and an exhibitor's pack |
| `GET /admin/registrations/{id}` | full record: registration, category, payments, files, passes, deliveries, audit trail |
| `POST /admin/registrations/{id}/review` | state transitions, see below |
| `GET /admin/registrations/{id}/files/{file}` | private receipt/logo download |
| `GET /admin/registrations/{id}/passes/{pass_id}.pdf` | one attendee's pass PDF, for attendees who cannot receive email; approved registrations and unrevoked passes only; audited as `pass_downloaded` |
| `GET /admin/registrations/{id}/passes/all.zip` | every active pass on the registration as a ZIP; same rules and audit |
| `POST /admin/registrations/{id}/attendees` | same body and rules as the registrant route; fills an unassigned pass on the exhibitor's behalf, attributed to the staff member in the audit trail. After approval the pass is issued at once but never sent; staff send it with the per-pass `send` review action |
| `POST /admin/registrations/{id}/attendees/{attendee}` | `{name, designation, email, phone, whatsapp_consent}` corrects one attendee on any registration that is not `cancelled`. Emails stay unique within the registration. Their active pass keeps its number and QR and re-renders with the new name or designation; queued deliveries to a changed email or phone are cancelled. Nothing is sent. A delegate's contact details follow their one attendee |
| `POST /admin/registrations/{id}/attendees/{attendee}/remove` | `{note}` (may be empty) removes the attendee and revokes their pass, cancelling its queued deliveries; their place becomes unassigned. A registration keeps at least one attendee. Removed attendees are retained (`removed_at`) for the pass history but leave every listing, count and export |
| `POST /admin/registrations/{id}/allowance` | `{roster_count, note}` sets an exhibitor registration's pass allowance (1-50), including more passes than its stall type includes. It cannot go below the attendees already on it. The new places are filled like any unassigned place; nothing is sent. A later stall change carries the extra passes over |
| `POST /admin/registrations/{id}/stall-number` | `{stall_number, note}` sets, changes or (with an empty value) clears the stall allocated to an exhibitor registration that is not `cancelled`. Values are trimmed and upper-cased; up to 16 letters, digits, spaces, hyphens or slashes, starting with a letter or digit. Audited as `stall_number_changed`. Nothing is sent |
| `POST /admin/registrations/{id}/category` | `{category_id, note}` moves an exhibitor registration that is not `cancelled` to another stall type. The pass allowance becomes the new stall's plus any extra passes granted earlier, and never drops below the attendees on the registration: moving to a smaller stall removes nobody, and the places beyond the new stall's allowance are kept as extra passes. The reference, issued passes and payments are unchanged; the fee due before approval follows the new stall. Nothing is sent |
| `GET /admin/export?format=csv\|xlsx&sheet=&<same filters>` | CSV is one `sheet` (`Registrations`, `Attendees`, `Payments`, `Deliveries`); XLSX has all four. Cells that begin with `= + - @` are prefixed with `'`. 50,000-row cap. |
| `POST /admin/categories` | `{id, open, free_open}` opens or closes a category; `open` governs paid registration and `free_open` governs free registration links. Omit either to leave it unchanged |
| `GET /admin/speakers` | Every speaker in the directory with their saved `contact` (`email`, `phone`, `whatsapp_consent`) and speaker pass: `registration` (`id`, `reference`, `status`), `attendee` (`id`, `email`, `phone`, `whatsapp_consent`), active `pass` (`id`, `number`) and the latest email `delivery` (`status`, `at`); each is `null` until it exists |
| `POST /admin/speakers/{id}` | `{email, phone, whatsapp_consent}` saves the speaker's contact and issues nothing. When the speaker already holds a pass, its holder is corrected too (queued deliveries to the old address are cancelled). Audited as `speaker_contact_saved` |
| `POST /admin/speakers/{id}/pass` | With an `Idempotency-Key`, issues the speaker's complimentary `speaker` pass from their saved contact, approved and not sent; returns `{registration_id}`. A speaker who holds a pass keeps it; a cancelled one is replaced. Send with the ordinary review `send`/`resend` actions or `bulk-send` |
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
| `POST /admin/posters/assets?kind=&label=` | raw-body PNG/JPEG upload, 8 MB, ≤8000x8000 and ≤32 MP. Each limit reports itself by name. Returns `{id, width, height, mime}`. Registration uploads instead have a 5 MB file-size limit and no pixel-dimension limit. |
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
| `approve_send` | verifies the payment, approves, atomically creates one pass per attendee on the registration (unassigned places get theirs when filled) and the pack (exhibitors), then queues delivery. Requires `successful` and `beneficiary_confirmed`. `verified_amount_paise` must equal the category fee for `verified_date`, less any coupon discount frozen on the registration. `verified_reference` must not already belong to an approved payment. Idempotent once approved. |
| `approve_only` | as above without queueing delivery |
| `reinstate` | approves a `rejected` registration with the same payment fields and checks as `approve_only` (none for a free-link registration) and, like it, queues no delivery |
| `record_approve_send` | from `awaiting_payment`, records a payment found directly in SBI as both reported and verified, then approves and queues delivery. Takes the same verified payment fields and confirmation flags as `approve_send`; no `payment_id` is required. Exhibitors must already have an institution logo. The payment and reviewer action are recorded atomically in the payment history and audit trail. |
| `record_approve_only` | as above without queueing delivery |
| `send` | queues the initial delivery for an approved registration; repeats are no-ops. With `pass_id`, only that pass is sent (no contact pack), still at most once per pass and channel |
| `resend` | needs a unique `request_id`; queues a fresh delivery of the existing passes. With `pass_id`, only that pass, to the holder's current email and permitted WhatsApp |
| `reissue` | `{pass_id, note}`; revokes that pass, its QR and its pass number, and issues the next version at the next free place in the registration (`...-4` after a roster of 3). Nothing is sent; send the replacement with `send` |
| `payment_reminder` | from `awaiting_payment` only; emails the contact the reference, the fee payable today, and a single-use `/recover#<token>` link valid for 7 days. The existing management link keeps working until that link is used. At most one reminder per registration per 24 hours. A queued reminder is cancelled if the registration has left `awaiting_payment` by the time it is sent. |
| `roster_notice` | exhibitors with unassigned passes, not `rejected` or `cancelled`; emails the contact how many passes are unassigned and a single-use `/recover#<token>` link valid for 7 days. At most one per registration per 24 hours. A queued notice is dropped if the places are filled, or the registration ends, before it is sent. `bioconnect roster-notice` sends it to every registration that has never had one. |
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
