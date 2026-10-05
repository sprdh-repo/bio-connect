# My agenda, contacts and feedback

Three attendee features that keep working at a crowded venue with a weak connection.
Everything the attendee saves stays on their phone; the server only answers lookups and stores anonymous feedback.
The API contract is in [`registration/docs/api.md`](../../registration/docs/api.md).

## My agenda

Attendees bookmark sessions, speakers and exhibitors from their pages, or from the bookmark on each session in the programme.
My agenda shows:

- **Now** and **Up next** from the saved sessions, refreshed every minute and also shown on Home.
- Saved sessions by day, with a warning on any two that overlap.
  Saving a session that overlaps another says so straight away, with Undo.
- **From speakers you saved**: sessions of saved speakers not yet saved.
- Saved speakers and exhibitors.
  A saved exhibitor keeps its last known details, so it shows before the directory loads; a newly allocated stall replaces them on the next directory load.
- **Session reminders**: a local notification 5, 10 (default), 15 or 30 minutes before each saved session.
  Reminders are scheduled on the phone, so they arrive offline.
  Android delivers them inexactly to avoid the exact-alarm permission, so one may arrive a few minutes early or late.
  Tapping one opens its session, including when it launches the app.
- **Add to calendar** on a session, or for several sessions picked in My agenda.
  The phone's calendar opens its new-event screen for each, in India time, so no calendar permission is needed.

Sessions saved before staff link speakers still match by name, as the speaker page always has.

### Staff

- **Sessions**: link each session to its speakers with **Link a speaker…**.
  Linked speakers show the session on their profile and in the agenda of attendees who saved them.
  New speakers need a save before they can be linked; removing a speaker unlinks them on save.
- **Menus**: My agenda and Contacts are Home tiles and the first Guide entries.
  Home's tiles are Speakers, Exhibitors, Venue, My agenda, Contacts and Moments (migration `035`), and the Home list ends with Sponsors.
  My agenda can also be made a bottom tab from the console, though five tabs crowd the bar.

## Contacts

Attendees scan another attendee's badge or pass to save their event profile, then add private notes and tags such as Follow up, Investor or Supplier.
A badge scanned without a connection is saved and loads its profile when the phone is back online.
Pull to refresh re-reads every saved profile, so a withdrawn consent is honoured.
Contacts export through the share sheet as CSV (spreadsheets and CRMs) or vCard (phone contacts); both include notes and tags.

Contact details are shared only with consent.
A scan returns what the badge already prints: name, designation, organisation and category.
The holder's email and phone are added only after they turn on **Share my email** or **Share my phone number** under their pass in My passes, which requires the same OTP verification as adding the pass.
Both are off by default, and apply per pass.

Saved contacts are kept in the keychain or keystore, as passes are.

## Feedback and After Bio Connect

Feedback stays closed until staff open it from the console (**Mobile app → Feedback**).
While open, attendees rate the event and any published session from 1 to 5, with an optional comment.
Answers are anonymous: the app sends a random per-install ID so a changed answer replaces the earlier one.
The same page shows the summary per session and every comment, with a CSV download.

**After Bio Connect** (`hub`) gathers the follow-up: a feedback prompt while feedback is open, the people met (Follow up first) with export, and the sessions, speakers and exhibitors saved.
Migration `033` adds **Share feedback** and **After Bio Connect** to the Guide switched off.
Tick **Show in app** on them, and add them to Home if wanted, when the event closes.
The feedback entry is hidden in the app whenever feedback is closed.

## Release checklist

- Deploy the backend first: it applies `033_agenda_contacts_feedback.sql` on start.
- Refresh `assets/content/event.json` from `GET /api/v1/public/app-content` so a first offline launch has the new menus.
- Android: core library desugaring is enabled for scheduled notifications; the manifest adds `POST_NOTIFICATIONS`, `RECEIVE_BOOT_COMPLETED`, the reminder receivers and the calendar intent query.
- iOS: `Info.plist` adds calendar and contacts usage descriptions (the calendar's location search may read contacts); the app delegate becomes the notification centre delegate so reminders show in the foreground.
