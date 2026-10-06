# Bio Connect 4.0 mobile alpha

Flutter attendee app for Bio Connect 4.0, built alongside the event website.
The app uses the public information already available on the website and submits delegate registrations natively to the event backend.

## Run

```sh
flutter pub get
flutter run
```

The Android and iOS projects are scaffolded.
An Android debug build can be created with `flutter build apk --debug`.
App IDs and release signing are still placeholders and must be set before store distribution.

## Current alpha

- Home: attendee quick links, event dates, venue, registration and access to themes.
- Sessions: published timetable, day filters, search and session details in India time.
- Speakers: the backend's published profiles, search and LinkedIn links where supplied.
- Guide: venue details, activities, searchable FAQs, exhibitor directory, passes, brochure, sponsors and leadership.
- Exhibitors: searchable directory with stall-type filters and a detail page per exhibitor.
  Stall numbers, stall sorting and the floor plan link appear only once the backend publishes them.
- Sponsors and Leadership: separate pages driven by app content (sponsors, ecosystem partners, state leadership and the advisory committee).
- Pull to refresh on every page backed by live data; if the backend is unreachable the page keeps saved content and says so.
- Remote images (exhibitor and sponsor logos, leadership portraits, speaker photos) use a disk cache (`imageCacheManager` in `lib/widgets/directory.dart`, 30 days, 500 files), so they show instantly on later launches and offline at the venue.
- Store updates: release builds check the Play Store / App Store (India listing) with [`upgrader`](https://pub.dev/packages/upgrader) and offer Update or Later, at most once a day.
  To force an update, add `[Minimum supported app version: x.y.z]` to the store description.
- Backend event-guide editor with draft/published controls, public APIs and staff audit history.
- Native delegate registration with live pass availability, validation and a secure handoff to payment.
- My passes: email or WhatsApp OTP, optional admission QR scanning, secure saved passes and on-phone admission QR display.
- Moments: consent-based selfie matching against the organizer's event album, processing updates, a private photo gallery, zoom, save and share.
- Selfie frame: take a selfie (front or back camera, live inside the frame) or choose a photo, pick a caption ("I'm attending", "See you at", "I'm speaking at", "I'm exhibiting at"), one of three designs and post (1080 x 1350) or story (1080 x 1920) size, then save or share.
  Pinch to reposition the photo. Staff can replace the shared message with the `frame.share_text` copy key.
- My agenda: bookmark sessions, speakers and exhibitors; Now and Up next; clash warnings; offline session reminders; add sessions to the phone calendar.
- Contacts: scan a badge to save someone's event profile, with private notes and tags and CSV or vCard export. Email and phone are shared only when the holder opts in from My passes.
- Feedback and After Bio Connect: anonymous ratings for the event and sessions while staff keep feedback open, and a follow-up page for after the event.
- Loading, retry, empty and no-match states are included for live content and directories.

Unpublished event-day sections show “To be announced”.
Staff can add and publish content from the registration backend at `/admin?view=event-guide`.
For Moments, staff set the provider's numeric album ID under Event details and publish a Guide menu entry with the `moments` destination.
Staff can independently show or hide exhibitor registration under Event details without removing delegate registration or the exhibitor directory.
Debug builds also show Moments in Guide and provide a sample state when no pass has been saved, so the screen can be reviewed during local development.
See [Event-day guide](docs/event-guide.md) for the API contract, publishing workflow and offline behavior.
See [My passes](docs/my-passes.md) for attendee verification, WhatsApp template setup and saved pass behavior.
See [My agenda, contacts and feedback](docs/agenda-contacts-feedback.md) for reminders, consent and the staff controls.
Bank payment and exhibitor registration remain on the secure official portal; maps, LinkedIn, the brochure and third-party applications open in their owning apps.

## Content and backend boundary

`lib/models/event_content.dart` contains typed event, theme, speaker and exhibitor models.
`lib/services/content_service.dart` defines the `ContentService` interface.
`CurrentContentService` loads `assets/content/event.json` immediately, then refreshes event details, themes, programme highlights, speakers, Product Launch, leadership, sponsors and ecosystem partners from the anonymous public app-content endpoint. The verified bundle is retained as an offline fallback.
`lib/providers/content_provider.dart` owns loading, error and retry state for the UI.
`lib/services/registration_service.dart` owns live pass categories and native delegate submission.

The bundled JSON is a curated offline snapshot of the website.
The backend `app_content` and `speakers` tables are the runtime sources of truth; update the snapshot as part of a release so offline users receive the same content.
Speaker portraits and theme art are copied from the website's approved assets.
The live content endpoint is `https://reg.bioconnect.kerala.gov.in/api/v1/public/app-content`.
The live exhibitor endpoint is `https://reg.bioconnect.kerala.gov.in/api/v1/public/exhibitors` and returns an `exhibitors` array with `name`, `description`, `logo_url`, `stall_number` and `stall_type` fields.
Staff allocate stall numbers on the admin registration page.
Registration options and current prices come from `/api/v1/categories`. `free_only` invitation categories are intentionally excluded from public mobile flows; private complimentary links remain admin-generated web URLs.

For local or staging backends, run with `--dart-define=BIO_CONNECT_API_BASE_URL=https://your-host`.

## Visual reference

`docs/design-reference.png` is an ImageGen UI concept, used for layout and mood.
It contains illustrative names, portraits and interface text, not authoritative event information.
The implemented screens use the website's actual content and assets.
The design system follows the website's forest green, cream, lime and gold palette, with its DM Sans and Manrope fonts.

The exact built-in ImageGen prompt is in `docs/design-prompt.md`.
The selfie frame artwork (`assets/images/frame-forest.webp`, `assets/images/frame-cream.webp`) is also ImageGen output; its prompt is in the same file.
The frame's text, logo and layout are drawn by the app, so event details stay accurate and sharp.

## Verify

```sh
dart format --output=none --set-exit-if-changed lib test
flutter analyze
flutter test
flutter build apk --debug
```
