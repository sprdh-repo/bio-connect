# Event-day guide

The mobile app reads published sessions, activities, FAQs and venue details from `GET /api/v1/public/app-content`, under `event_guide`.
The same guide is available independently at `GET /api/v1/public/event-guide`.
Empty sections show “To be announced”; no schedule, hall, floor plan or event-day policy is invented.
Existing programme highlights remain visible in Activities while activity details are unannounced.

## Staff workflow

1. Apply the backend migrations using the existing deployment process, including `014_event_guide.sql`.
2. Open `/admin?view=event-guide` on the registration backend and sign in with an individual staff account.
3. Add sessions, activities and FAQs, and fill in venue details.
4. Select **Published in the mobile app** for each entry that is ready.
5. Save the event guide.

Both manager and reviewer accounts can edit this content.
The editor uses the existing staff session and CSRF protection.
Unpublished entries stay in the admin response and are omitted from both public APIs.
Venue fields publish together.
Removing an entry applies only after saving.
Saving fails with HTTP 409 if another editor has saved since the page was loaded; reload saved content before editing again.
Every save records a staff audit event.

## API contract

`GET /api/v1/admin/event-guide` returns the complete editable guide and its integer `revision`.
`PUT /api/v1/admin/event-guide` replaces the guide using that revision and returns the saved guide with the next revision.
Requests use JSON, an authenticated `bc_session` cookie and the matching `X-CSRF-Token` header.

```json
{
  "revision": 1,
  "sessions": [],
  "activities": [],
  "faqs": [],
  "venue": {
    "address": "",
    "arrival": "",
    "accessibility": "",
    "floor_plan_url": "",
    "help_email": "",
    "help_phone": "",
    "published": false
  }
}
```

Session fields: `id`, `title`, `description`, `starts_at`, `ends_at`, `location`, `speakers`, `published`.
Both timestamps can be empty; otherwise both must be RFC 3339 timestamps with timezone offsets and the end must follow the start.
The editor accepts India time and sends `+05:30`; mobile displays IST regardless of the device timezone.

Activity fields: `id`, `title`, `description`, `schedule`, `location`, `published`.
FAQ fields: `id`, `question`, `answer`, `published`.
IDs are unique within each section and remain stable when an entry is edited.
The array order controls activity and FAQ order; mobile sorts sessions chronologically, followed by untimed sessions.
Floor plans must use an HTTPS URL.
The guide supports up to 300 sessions, 100 activities and 100 FAQs, subject to the existing 128 KiB JSON request limit.

## Mobile updates and offline behavior

Pull down on Home, Sessions, Activities, Venue or FAQs to refresh.
Returning to the app also refreshes content.
An unsuccessful refresh retains content already loaded in the current app session.
A fresh app launch without connectivity uses the bundled event snapshot, which currently contains no confirmed event-day guide entries.
Updating that release snapshot is necessary if confirmed guide details must be available on a first offline launch.
An older backend without `event_guide` remains compatible and displays the unannounced states.

For a local backend, start Flutter with the existing `BIO_CONNECT_API_BASE_URL` setting pointing at that backend.
The default URL still points at the public registration backend, so local API changes are not visible there until deployed.

## Branding

The mobile wordmark uses the transparent approved asset from `registration/internal/app/web/logos/bio-connect-mark.png`.
The launcher mark is maintained as `assets/app-icon.svg` and `assets/app-icon-foreground.svg`.
Render these SVG sources to their corresponding PNG files, then run:

```sh
flutter pub run flutter_launcher_icons
```

Android adaptive icons use a transparent foreground and cream background.
iOS icons use an opaque background.
A rebuild and reinstall are required to update device launcher icons.

## Navigation and touch feedback

Back from a detail page returns to its originating screen.
At the tab level, Android back returns to Home before showing an exit confirmation on the next back action.
iOS retains native detail-page back gestures and does not offer a programmatic app exit.
Tab positions are preserved; tapping the selected tab again scrolls to the top.
Searches have clear controls, and scrolling dismisses the keyboard.
Discrete haptics accompany tab and filter selection, page navigation, refresh and registration actions.
No haptic feedback is emitted while typing or continuously scrolling.

Registration asks before discarding edited details and prevents leaving during a submission.
After saving, closing the confirmation sheet returns to the previous screen so the completed form cannot be accidentally submitted again.
