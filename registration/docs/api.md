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
- Exhibitors: `attendees` must hold exactly 6 (premium), 4 (standard), or 2 (table space); `description` is required. The contact person receives a pass only if listed among the attendees.
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
{"reference":"SBICR000123","date":"2026-09-20","amount_paise":600000,"receipt_id":"<fid>"}
```

Requires a `receipt` file (and a `logo` for exhibitors).
Moves the registration to `awaiting_review` and appends to the payment history.
A receipt upload or a browser redirect never approves payment on its own.

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
- `reviewer`: everything else below.

| Route | Purpose |
|---|---|
| `GET /admin/me` | `{id, role}` |
| `POST /admin/logout` | clears the session |
| `GET /admin/registrations?q=&category=&status=&from=&to=&page=` | paginated list (25/page); `from`/`to` are `YYYY-MM-DD`. `q` matches reference, institution, contact, attendee name/email, and pass number; references and pass numbers match case-insensitively with the hyphens optional |
| `GET /admin/registrations/{id}` | full record: registration, category, payments, files, passes, deliveries, audit trail |
| `POST /admin/registrations/{id}/review` | state transitions, see below |
| `GET /admin/registrations/{id}/files/{file}` | private receipt/logo download |
| `GET /admin/export?format=csv\|xlsx&sheet=&<same filters>` | CSV is one `sheet` (`Registrations`, `Attendees`, `Payments`, `Deliveries`); XLSX has all four. Cells that begin with `= + - @` are prefixed with `'`. 50,000-row cap. |
| `POST /admin/categories` | `{id, open}` opens or closes a category |
| `POST /admin/bulk-send` | `{ids:[...], channel:""}` runs `send` for 1-100 approved registrations; returns per-id `queued` or the error |
| `POST /admin/retry` | `{id, confirm_uncertain, note}` requeues a `failed` or `uncertain` delivery. `uncertain` needs `confirm_uncertain:true` and a `note`. The prior job and its provider id are kept for late webhooks. |

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
| `send` | queues the initial delivery for an approved registration; repeats are no-ops |
| `resend` | needs a unique `request_id`; queues a fresh delivery of the existing passes |
| `reissue` | `{pass_id, note}`; revokes that pass, its QR and its pass number, issues the next version at the next free place in the registration (`...-7` after a roster of 6), queues delivery |
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
