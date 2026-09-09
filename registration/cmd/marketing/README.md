# Registration invitation sender

Run from `registration/` with the existing Go toolchain.
The HTML and plain-text invitation follow the website and registration email styling.
Edit `invitation.html` and `invitation.txt` together to change the copy.
This early-bird version refuses live sends after 30 September 2026 in India time.

## Preview and validate contacts

```sh
go run ./cmd/marketing
go run ./cmd/marketing --contacts /path/to/contacts.xlsx --email-column Email --name-column Name
```

Both commands are dry runs and send nothing.
Open `var/marketing/preview.html` to review the design.
The second command validates the entire list before any sending, deduplicates email addresses case-insensitively, and previews the first recipient.
The first row must contain headers; blank rows are ignored and invalid/missing email cells stop the import with a row number.
Use `--sheet 'Sheet name'` to select an Excel sheet, or supply a `.csv` file.
Omit `--name-column` for an unpersonalised greeting.
Legacy `.xls` files must first be saved as `.xlsx`.
Use contacts who have agreed to receive event invitations.

## Send a test

Load the existing trusted environment file without printing credentials:

```sh
set -a
source ops/secrets/bioconnect-infra.env
set +a
go run ./cmd/marketing --test-to shiyaf@sprdh.com
```

Alternatively export `POSTMARK_SERVER_TOKEN`, `POSTMARK_FROM_ADDRESS` and `POSTMARK_FROM_NAME` yourself.
Test mode sends exactly one message with a `[TEST]` subject and a Shiyaf greeting, and cannot be combined with a contacts file or `--send`.
Use `--test-name Sumi` to personalise a test for another recipient, or `--test-name ''` for a generic greeting.
Repeated test commands intentionally send another test.
The default stream is `broadcast`; override it with `--stream` if needed.
The command verifies the stream is active, of type Broadcasts, and uses Postmark-managed unsubscribe handling.
The unsubscribe link is resolved by Postmark during sending.
See [Postmark unsubscribe documentation](https://postmarkapp.com/support/article/1208-how-to-add-an-unsubscribe-link).

## Send the reviewed campaign

After approving the design, content and recipient list:

```sh
go run ./cmd/marketing --contacts /path/to/contacts.xlsx --email-column Email --name-column Name --campaign registration-september-2026 --send
```

Each contact receives an individual message; recipient addresses are never shared through CC/BCC.
The sender uses the existing configured identity and replies go to `bioconnect@bio360.in`.
No registration application deployment is required.

## Logs and reports

All files are private by default under the git-ignored `registration/var/marketing/` directory.
Use `--state /private/persistent/path` to choose another persistent location.
Keep this directory between runs and use the same campaign ID when resuming.

- `report-<UTC timestamp>.csv`: one row per unique recipient for this run, including campaign, email, name, status, UTC timestamp, Postmark message ID, error and attempt key.
- `sends.jsonl`: append-only, synchronously flushed event log containing each attempted send and its outcome with the same recipient details.
- `preview.html` and `preview.txt`: the first recipient's rendered email, replaced on the next run.

Reports are atomically updated before and after each send.
Statuses are `not_attempted`, `attempted`, `accepted`, `rejected`, `uncertain`, or `skipped_previous_<status>`.
An accepted status means Postmark accepted the request, not that the recipient's mailbox delivered or opened it.
Use the message ID in Postmark activity to check delivery, bounces and other later events; this CLI does not sync delivery events into its reports.
The process stops on the first send failure and exits nonzero, leaving subsequent rows marked `not_attempted`.
If the process is interrupted, an `attempted` row must be treated as uncertain.
The durable JSON log is authoritative if a crash prevents the CSV update.

Every prior attempt for the same stream, campaign and email is skipped on rerun, including uncertain and rejected attempts.
The lock prevents concurrent senders sharing the same state directory.
Do not delete the state, change campaign IDs, or use separate state directories to retry uncertain sends.
First reconcile those attempts against Postmark activity; retry recovery is intentionally manual.
Tests have unique attempt keys so they never mark a recipient as already sent for the real campaign.
Open CSV reports directly in Excel; formula-like cells are neutralised.

```sh
go test ./cmd/marketing
```
