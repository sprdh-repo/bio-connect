# Store reviewer access

The existing Add a pass email verification flow supports an optional synthetic reviewer identity.
Enable it by configuring `STORE_REVIEW_CODE` with a private six-digit code in the backend environment and deploying the backend with migration 036.
Keep the code out of source control and provide it only through the stores' reviewer access forms.
No mobile release is required for this backend feature.

## Play Console instructions

Choose Yes under Sign-in details because adding a pass requires verification.
Use `review@bioconnect.example` as the email and the configured code as the reusable verification credential.

Paste these instructions, replacing the code placeholder:

> Open Registration & passes / My passes, then Add a pass.
> Select Email and enter review@bioconnect.example.
> Tap Send verification code and enter [configured six-digit code].
> No email inbox or WhatsApp access is required for this reviewer identity.
> The code can be reused after requesting a new challenge.
> The sample pass demonstrates the pass wallet, sharing preferences and PDF download.
> It is marked as a sample and cannot grant physical event admission.
> All event information can also be browsed without verification.

Use the same instructions in Apple App Review access information.

## Isolation and operation

The reviewer receives a virtual pass; no registration, attendee, payment, issued pass or delivery job is created.
The QR is deliberately outside the admission QR format and has no issued-pass database record.
The reusable code applies only to the reserved reviewer email, never WhatsApp or real attendees.
Normal OTP challenge expiry, single-use challenges, attempt limits and request limits still apply.
A fresh challenge is needed for each login, but the configured credential remains reusable.
Reviewer sharing preferences are stored per session and cannot modify real attendees.
Moments uses a synthetic attendee identifier for this pass, with the existing Moments feature and provider configuration.
Reviewer-created Moments data should be removed through the normal deletion flow after testing.
Downloads authenticate a session-scoped capability and return only a visibly labelled synthetic PDF.
Download links contain session tokens and must not be pasted into public channels.

Setting `STORE_REVIEW_CODE` to empty disables reviewer login and existing reviewer sessions immediately after the backend restarts.
Rotating the code changes future challenges; already-issued challenges expire within ten minutes and sessions retain their normal lifetime.
The backend remains disabled by default.

## Verification before resubmission

Exercise the deployed email challenge, verification, pass list, sharing settings and PDF download using the existing release app.
Confirm a wrong code fails and real attendee identities still require their delivered OTP.
Confirm the sample QR is rejected by event admission.
Confirm enabled Moments functionality is accessible with the synthetic identity if Moments is part of the submitted release.
Save the reviewer access declaration and resubmit the rejected release.
