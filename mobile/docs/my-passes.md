# My passes

Home and Guide include **My passes**.
The fixed header on all four main tabs also has a **My passes** button at the top right, available while scrolling and when event content is loading or unavailable.
Attendees can retrieve their issued pass using email OTP or WhatsApp OTP.
They can optionally scan an existing printed or downloaded admission QR before verifying its attendee email or mobile.
The scanner accepts the raw 43-character admission identifier printed on the existing pass, not a registration link or a PDF download URL.
Scanning does not disclose attendee details or grant access by itself.

After verification, the app displays the attendee name, organisation, designation, category, pass number and admission QR.
**Open official PDF** opens the existing private pass PDF in the phone's browser or owning app and needs a connection.
Multiple verified identities or scanned passes can be saved on one phone.
A verified email/mobile sees only active passes whose attendee identity matches, never all passes belonging to an exhibitor contact.
An optional scanned QR restricts that session to the scanned pass.
A pass appears only after approval and is hidden if revoked, removed or cancelled.

## Backend setup

The backend automatically applies `028_mobile_pass_access.sql` through the existing migration command.
The delivery worker must run for OTP messages to be sent.
Existing Postmark sender configuration delivers email OTP.
WhatsApp OTP uses the existing Meta sender credentials with a separate approved **AUTHENTICATION** template containing a **COPY_CODE** button.
The document-header pass-delivery template cannot send authentication codes.
Configure the template's exact approved name and language in the backend environment:

```dotenv
META_OTP_TEMPLATE=your_approved_authentication_template
META_OTP_TEMPLATE_LANGUAGE=en_US
```

The template is not created or approved by this change.
If live delivery is enabled but `META_OTP_TEMPLATE` is unset, WhatsApp verification returns 503 and the attendee can choose email instead.
Development delivery remains fake, consistent with existing backend behavior.
No code is returned in the public API, including in development.
Use the isolated integration test fixtures to inspect fake OTP delivery safely.
The mobile app requests explicit consent for each WhatsApp verification request; this does not change the attendee's existing pass-delivery consent.

## API

All endpoints use the existing `/api/v1` error and no-store conventions.
No staff or registration-management token grants attendee pass access.

- `POST /mobile/pass-access/challenges`: `{ "channel": "email" | "whatsapp", "identifier": "...", "qr_id": "optional admission identifier", "whatsapp_consent": true }`.
- Returns 202 with `challenge`, `expires_at`, `resend_after_seconds` and a generic message.
- `POST /mobile/pass-access/verify`: `{ "challenge": "...", "code": "123456" }`.
- Returns 200 with `token` and `expires_at` after successful verification.
- `GET /mobile/passes`: send `Authorization: Bearer <token>`.
- Returns `{ "passes": [{ "id", "name", "institution", "designation", "category", "number", "qr_id", "download_url" }], "checked_at": "RFC3339 timestamp" }`.
- `DELETE /mobile/passes`: revokes the authenticated session on this device.

Codes expire after 10 minutes and allow five attempts.
Successful verification consumes the code once.
A resend invalidates older challenges for the same channel and identity.
Requests allow one per minute, five per identity per hour and twenty per IP per hour.
Verification allows sixty attempts per IP per hour.
Unknown identities and mismatched scanned QR/identity combinations receive the same 202 shape as matching identities, but no code is sent.
Local Indian ten-digit mobile numbers are normalised to `+91`; other numbers need their country code.
Sessions expire after thirty days and are scoped by attendee identity and optional scanned QR.
The worker cancels queued OTPs whose challenge has expired, been superseded, been consumed or exhausted its attempts.

## Device storage and offline access

Session tokens and pass snapshots are stored together in platform secure storage, separately for each configured API backend.
Android backups are disabled and iOS storage uses device-only Keychain accessibility.
Camera permission is requested only when the scanner opens.
Users can use email or WhatsApp verification if camera access is denied or unavailable.
The scanner stops with its screen and follows app lifecycle changes.

Opening My passes, returning to the foreground or pulling down refreshes saved sessions.
A network failure retains the saved QR and shows the last status-check time and a saved-copy notice.
An expired or unauthorized session removes its cached passes.
A successful online refresh replaces that session's snapshot and removes revoked passes.
A new installation without connectivity cannot retrieve passes.
The event desk confirms admission status; an offline saved copy cannot know about a later revocation.
**Remove saved passes** clears this phone's copies and attempts to revoke the saved sessions without cancelling the registration.
If offline, those session tokens expire normally on the server.

## Verification

Flutter tests cover email and WhatsApp flows, consent, retry after a consumed OTP, offline restoration, revocation and expired sessions.
Backend integration tests use only a disposable PostgreSQL database and fake/local provider endpoints.
They cover attendee isolation within exhibitor groups, scan scope, consent, rate limits, wrong codes, expiry, resend invalidation, replay, session deletion, provider payloads and cancellation of stale queued OTPs.
Android build and static analysis are part of the existing mobile checks.
Physical camera scanning, real Postmark delivery, real WhatsApp delivery and iOS signing must be verified on devices with the configured production providers before release.
