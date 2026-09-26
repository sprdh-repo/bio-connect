# Bio Connect 4.0 mobile alpha

Flutter attendee app for Bio Connect 4.0, built alongside the event website.
The alpha uses the public information already available on the website and opens the official registration flow in the browser.

## Run

```sh
flutter pub get
flutter run
```

The Android and iOS projects are scaffolded.
An Android debug build can be created with `flutter build apk --debug`.
App IDs and release signing are still placeholders and must be set before store distribution.

## Current alpha

- Home: event date, venue, registration, themes and speaker preview.
- Explore: the five website themes and programme highlights.
- Speakers: the website's 41 profiles, search and LinkedIn links where supplied.
- More: confirmed exhibitor directory, delegate and exhibition pricing, brochure, directions, product launch, leadership and sponsors.
- Loading, retry, empty and no-match states are included for the content and exhibitor directory.

The website does not publish a timed session agenda, attendee tickets or meeting data, so the alpha does not invent these.
Registration remains on the official site.

## Content and backend boundary

`lib/models/event_content.dart` contains typed event, theme, speaker and exhibitor models.
`lib/services/content_service.dart` defines the `ContentService` interface.
`CurrentContentService` loads `assets/content/event.json` for event information and uses the existing anonymous public exhibitor endpoint.
`lib/providers/content_provider.dart` owns loading, error and retry state for the UI.

The bundled JSON is a curated snapshot of the current website.
Update it when the website's speakers, themes, prices or event details change.
Speaker portraits and theme art are copied from the website's approved assets.
The live exhibitor endpoint is `https://reg.bioconnect.kerala.gov.in/api/v1/public/exhibitors` and returns an `exhibitors` array with `name`, `description` and `logo_url` fields.

`ApiContentService` is a future adapter expecting the same JSON shape as `assets/content/event.json` at `/api/v1/public/app-content`.
To select it once that endpoint exists, run with `--dart-define=BIO_CONNECT_API_BASE_URL=https://your-host`.
The endpoint is an app contract proposal, not a currently deployed API.
Because screens depend on the provider and service interface, the data source can change without rebuilding the UI.

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
