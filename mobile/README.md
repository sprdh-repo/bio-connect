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
- Guide: venue details, floor plan, activities, searchable FAQs, exhibitor directory, passes, brochure and partners.
- Backend event-guide editor with draft/published controls, public APIs and staff audit history.
- Native delegate registration with live pass availability, validation and a secure handoff to payment.
- Loading, retry, empty and no-match states are included for live content and directories.

Unpublished event-day sections show “To be announced”.
Staff can add and publish content from the registration backend at `/admin?view=event-guide`.
See [Event-day guide](docs/event-guide.md) for the API contract, publishing workflow and offline behavior.
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
The live exhibitor endpoint is `https://reg.bioconnect.kerala.gov.in/api/v1/public/exhibitors` and returns an `exhibitors` array with `name`, `description` and `logo_url` fields.
Registration options and current prices come from `/api/v1/categories`. `free_only` invitation categories are intentionally excluded from public mobile flows; private complimentary links remain admin-generated web URLs.

For local or staging backends, run with `--dart-define=BIO_CONNECT_API_BASE_URL=https://your-host`.

## Visual reference

`docs/design-reference.png` is an ImageGen UI concept, used for layout and mood.
It contains illustrative names, portraits and interface text, not authoritative event information.
The implemented screens use the website's actual content and assets.
The design system follows the website's forest green, cream, lime and gold palette, with its DM Sans and Manrope fonts.

The exact built-in ImageGen prompt is in `docs/design-prompt.md`.

## Verify

```sh
dart format --output=none --set-exit-if-changed lib test
flutter analyze
flutter test
flutter build apk --debug
```
