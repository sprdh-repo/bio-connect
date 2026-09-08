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

API contract: [`docs/api.md`](docs/api.md).
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

They cover both registration journeys and every category, the exact exhibitor roster counts (6 / 4 / 2), fee-cutoff boundaries, mismatched amounts, duplicate bank references, payment corrections, interrupted submissions, secure recovery, concurrent approval, approve-only then send, cancellation, reissue vs resend, bulk send, provider failures, worker-lease expiry, duplicate webhooks, staff permission separation, TOTP replay, private-file scoping, export contents and formula-injection neutralisation, the per-category reference series under concurrent registration, pass numbering across a roster and a reissue, staff search by either identifier, and QR readability.

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

## Layout

```
cmd/bioconnect/         entry point; `migrate` and `staff-create` subcommands
internal/app/           the application (one package)
  migrations/           embedded SQL, applied once under an advisory lock
  web/                  embedded server-rendered page, CSS, and progressive-enhancement JS
  fonts/                embedded pass font (Noto Sans, OFL)
ops/                    production compose file, Caddyfile, backup/monitor scripts, systemd units
compose.yaml            local development
Dockerfile              distroless-style runtime image
```

Credentials live only in the environment and are never committed.
Uploads and passes go to private S3 in production and to `STORAGE_DIR` locally.
