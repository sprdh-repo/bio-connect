# Bio Connect 4.0 registration and pass management

Server-rendered Go application behind `reg.bioconnect.kerala.gov.in`.
It handles delegate and exhibitor registration, manual SBI payment review, branded QR passes, and pass delivery over email and WhatsApp.
The marketing site in the repository root is unchanged apart from the two registration links in `index.html`.

The event is Bio Connect 4.0, 8 to 9 October 2026, Hyatt Regency Trivandrum.
All email subjects, message bodies, and passes carry Bio Connect 4.0 branding.
Delivery uses the existing Zinvos Postmark sender and the existing Zinvos WhatsApp sender; no new SES identity or WhatsApp number is introduced.

## What it does

- Public forms at `/delegates` and `/exhibitors` matching the site's visual language, with mobile-friendly steps, a review screen, and recoverable errors.
- Server-side price calculation in INR using `Asia/Kolkata`, with early-bird eligibility tied to the verified payment date through 30 September 2026.
- Registration saved before payment; SBI Collect paid out of band; payment reference, date, amount, and receipt collected for manual staff review.
- Registration states: `awaiting_payment`, `awaiting_review`, `correction_requested`, `approved`, `rejected`, `cancelled`.
- Staff console at `/admin` with individual accounts, mandatory TOTP, server sessions, CSRF protection, and login throttling. Account-management and registration-review permissions are separate roles (`manager`, `reviewer`).
- Reviewers can record and approve a payment found directly in SBI when the registrant did not submit payment evidence; the verified payment and staff action are retained in the payment history and audit trail.
- Approval atomically creates one pass per attendee plus the delivery jobs. Concurrent approvals and repeated clicks never create extra passes.
- Branded A5 PDF passes with an opaque QR (no contact information). Attendance scanning is deferred.
- Short human identifiers in one series per category: a registration is `BC4-EX-0007` (the 7th exhibitor), and each of its passes is that reference plus the holder's place in it, `BC4-EX-0007-3`, printed large on the pass and repeated in the pass email.
The two letters are the word already printed on the pass (`EX`, `FC`, `IN`, `SP`, `ST`), so a code and a badge can never disagree.
Staff search matches either, with or without the hyphens.
They are handles, never credentials - short and predictable on purpose, while the QR identifier and the download token stay full-entropy and unguessable.
A sequential series does publish how many registrations a category has, and a printed number is guessable, so admission must scan the QR or match the holder in the console rather than trust the number on a print.
- PostgreSQL-backed delivery queue with ret/uncertain handling, channel-specific resend, reissue (revokes the previous pass), and failed-delivery retry.
- Postmark and Meta status webhooks, authenticated and correlated to delivery records.
- Filtered CSV and XLSX exports with separate sheets and spreadsheet-formula-injection protection.
- A social poster studio in the console, open to both staff roles, with 24 templates shipped in the binary: four post types in a light and a dark look at three sizes each. Staff fill a template per post and download a PNG at every size, or lay out a new one in the builder.

API contract: [`docs/api.md`](docs/api.md).
Public exhibitor directory: [`docs/public-exhibitors.md`](docs/public-exhibitors.md).
Deployment, backups, restore, and rollback: [`docs/operations.md`](docs/operations.md).

## Local development

Requires Go 1.26+, Docker, and Poppler (`pdfinfo`, used to validate PDF receipts; `pdftoppm`/`zbarimg` are only needed for the pass-rendering tests).

```sh
cd registration
cp .env.example .env
# put a key in .env:  openssl rand -base64 32
docker compose up -d db
export $(grep -v '^#' .env | xargs)
go run ./cmd/bioconnect
```

The app migrates the database on start.
Registration is disabled by default (`REGISTRATION_ENABLED=false`); the forms and fees are still viewable.
With `LIVE_DELIVERY=false` the delivery worker uses a fake provider that marks every job delivered, so nothing leaves the machine.

Create the first staff account (prints the `otpauth://` URI to enrol in an authenticator):

```sh
echo '{"Email":"you@example.com","Password":"a-long-passphrase","Role":"manager"}' \
  | go run ./cmd/bioconnect staff-create
```

`docker compose up` (without `db`) also builds and runs the app container against a private Postgres.

## Tests

Integration tests need a throwaway PostgreSQL; they drop and recreate the `public` schema on every test.

```sh
docker compose up -d db
TEST_DATABASE_URL='postgres://bioconnect:local-development-only@localhost:55432/bioconnect?sslmode=disable' \
  go test ./...
```

They cover both registration journeys and every category, the exact exhibitor roster counts (3 / 2 / 2) with registrations made before a roster change keeping their original roster, fee-cutoff boundaries, mismatched amounts, duplicate bank references, payment corrections, interrupted submissions, secure recovery, concurrent approval, approve-only then send, cancellation, reissue vs resend, bulk send, provider failures, worker-lease expiry, duplicate webhooks, staff permission separation, TOTP replay, private-file scoping, export contents and formula-injection neutralisation, the per-category reference series under concurrent registration, pass numbering across a roster and a reissue, staff search by either identifier, and QR readability.
For posters they cover template-spec validation rule by rule, the 8 MB image checks, poster QR payload rejection, inline asset serving (a redirect here would taint the export canvas), a template save retiring the family's previous active size, field and per-size framing round-tripping, both staff roles reaching the poster routes while registrations stay separated, and every offered builtin logo actually being embedded.
The shipped artwork is checked too: every spec validates, references two embedded PNGs of exactly its declared canvas size, carries defaults for the event furniture and an https QR payload, and leaves no orphan PNG in the binary; and `poster-seed` is idempotent, keeps staff-edited templates, duplicates no assets on `--replace`, and leaves no template pointing at a missing asset.

To eyeball a rendered pass:

```sh
DUMP_PASS_DIR=/tmp go test -run TestDumpSamplePasses ./internal/app/
```

This generates Exhibitor, Faculty, Industry, Startup, and Student samples on the same A5 grid, including a long attendee name.
The Industry sample keeps the pre-shortening 32-character pass number, so the footer can be checked against passes issued before short numbers.
The pass uses "Bio Connect 4.0 · Kerala's international life sciences summit" and "Building Kerala's / Global Life Sciences Hub".
Booth allocation is handled separately, so exhibitor passes show the representative's role.
Already cached pass PDFs retain their original artwork; this renderer applies when a PDF is first generated.

Check PDF wording, field overflow, category alignment, and QR scanning without a database:

```sh
go test -run 'TestRenderedPassVariants|TestPassFitsMaximumLengthFields' ./internal/app/
```

## Social posters

The console at `/admin` carries a poster studio for speaker reveals, session
announces, countdowns and partner welcomes.
A template is an **ordered layer stack** laid out on the canvas by staff, not a flat
background image: array order is draw order, which is what lets a decoration sit
above the portrait and a caption above that.
Layer types are `art` (uploaded artwork or one of the embedded brand logos),
`photo`, `text` and `qr`.
A `family` groups the sizes that share field names - 4:5 (1080x1350), 1:1 and 9:16 -
so a poster is filled once and exported at each of them, with the photo framing kept
per size because one portrait needs a different crop at 4:5 and at 9:16.

A layer's `key` is its field, so two layers sharing one draw the same content and
share a default.
That is a real design - a name can appear twice on a poster - so the builder allows
it, names it in the panel when it happens, and hands every new layer its own free
key so it is never what you get by accident.

In the builder, drag a box to move it and its bottom-right corner to resize.
Hold Shift while resizing to keep the box's proportions.
Arrow keys nudge the selected layer by a pixel and Shift by ten; Delete removes it.

**Rendering happens in the browser**, on a canvas at true output resolution, so the
preview a staff member drags a portrait around in is the file they download. There
is no second renderer to drift from it and no image encoder in `go.mod`. Three
consequences are load-bearing and easy to undo by accident:

- Poster assets are served **inline** by `serveInline`, never redirected to a
  presigned S3 URL. A cross-origin image with no CORS headers taints the canvas and
  makes `toDataURL` throw, which breaks every export. `TestPosterAssetIsServedInlineNotRedirected` guards this.
- The CSP allows `img-src 'self' data:` and not `blob:`, so uploads are read with
  `FileReader.readAsDataURL` and downloads go through a `data:` URL.
- Canvas does not trigger webfont loading. `poster.js` awaits `document.fonts.load`
  for every size a template uses before the first draw; without that, Manrope
  silently falls back to a system font in the exported PNG.

Portraits can carry the same forest-green duotone as `speakers.html`, applied in the
renderer from the three stops in `scripts/portrait.py` rather than by shelling out.

`web/logos/` holds the marks a template can place without an upload. **The Government
of Kerala emblem is deliberately not among them.** The copy in the site's `assets/` is
Wikimedia's, CC BY-SA 4.0, and the marketing site discharges that by naming the
photographer and linking the licence in `committee.html`; a social image has nowhere to
carry an attribution, and ShareAlike would reach the poster itself. An official
Government of Kerala emblem, which carries no such obligation, can be uploaded as
artwork - and doing that is the right fix if posters need the emblem.

**Twenty-four templates ship in the binary** - four post types (speaker reveal,
session announce, countdown, partner welcome), each in a light and a dark look,
each at 4:5, 1:1 and 9:16. Install them on any environment with:

```sh
bioconnect poster-seed            # creates what is missing, keeps what exists
bioconnect poster-seed --replace  # rolls out changed artwork
```

It is safe to re-run. Without `--replace` it never touches a template that is
already there, so a deploy cannot undo a layout staff changed in the builder.
Assets are keyed by a hash of their bytes, so unchanged artwork is never
uploaded twice, and an asset whose object has gone missing is restored rather
than handed out as a broken reference.

The artwork is generated, not hand-drawn, and its source is in
[`artwork/`](artwork/README.md) along with the script that builds all 48 PNGs
and all 24 specs. Artwork is template *content*, so it lives in the database and
object storage; the binary only carries the copy `poster-seed` installs.

Staff are not limited to these. The studio's gallery shows every template as a
thumbnail of its own artwork, grouped by post type with the light and dark looks
on a toggle, and each card can:

- **Make a poster** - the common case, one click from the picture.
- **Edit** any of its sizes in the builder.
- **Duplicate** the whole family under a new name. This is the safe way to start
  from a shipped template: the copy is staff-owned from birth, shares the
  original's artwork rather than re-uploading it, and no rollout can touch it.
- **Retire** it, which hides it without deleting the row - posters already made
  from it keep their wording.

**Editing a shipped template makes it staff-owned**, recorded as
`poster_templates.origin`. `poster-seed --replace` then reports it as
`kept (edited)` and leaves it alone, so a deploy can never revert work done in
the builder. A template built from scratch is staff-owned already.

One CSP consequence worth knowing before touching `poster.js`: the page is
served `style-src 'self'`, so a `style=""` attribute in generated markup is
dropped before it reaches layout - silently, with only a console warning. The
gallery applies a template's background colour through CSSOM for that reason and
carries a CSS fallback. `app.js` contains no inline styles at all; keep it that
way.

## Layout

```
cmd/bioconnect/         entry point; `migrate` and `staff-create` subcommands
internal/app/           the application (one package)
  migrations/           embedded SQL, applied once under an advisory lock
  web/                  embedded server-rendered page, CSS, and progressive-enhancement JS
    poster.js           the poster studio: one renderer, the editor and the template builder
    logos/              brand marks a poster template can place without an upload
  fonts/                embedded pass font (Noto Sans, OFL)
ops/                    production compose file, Caddyfile, backup/monitor scripts, systemd units
compose.yaml            local development
Dockerfile              distroless-style runtime image
```

Credentials live only in the environment and are never committed.
Uploads and passes go to private S3 in production and to `STORAGE_DIR` locally.
