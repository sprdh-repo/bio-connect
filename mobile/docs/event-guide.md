# Event-day guide and app content

The mobile app reads everything it shows from `GET /api/v1/public/app-content`: event details, menus, headings, sessions, activities, FAQs, venue and help contacts, speakers, sponsors, partners, leadership, product launch and themes.
The guide part is also available on its own at `GET /api/v1/public/event-guide`.
The full response contract is in [`registration/docs/api.md`](../../registration/docs/api.md#get-publicapp-content).

Nothing is invented and nothing is announced before it exists.
Empty values are left out of the screens, menu entries whose page would be empty are hidden, and the Sessions tab shows a short notice until the programme is published.

## Staff workflow

1. Deploy the backend; it applies `031_mobile_content_control.sql` on start.
2. Sign in to the staff console and choose **Mobile app** (or open `/admin?view=mobile`).
3. Pick a section: Event, Menus, Headings & text, Programme, Activities, FAQs, Venue & help, Speakers, Sponsors & partners, Leadership, Product launch, Themes & highlights, or Feedback.
4. Edit, reorder with the arrows, and tick **Show in app** on each entry that is ready.
   Tick **Hide in app** beside a field to keep its value in the console while withholding it from phones.
5. Select **Save all changes**.
   Phones pick the changes up on their next refresh: pull down, or return to the app.

Both manager and reviewer accounts can edit this content.
One save writes the content, the guide and the speaker directory together and records a `mobile_content.update` audit event.
Saving fails with HTTP 409 if someone else has saved since the page was loaded; use **Discard & reload** and edit again.
Removing an entry applies only after saving.
**View live data** opens exactly what phones receive.

### What staff control

- **Bottom tabs**: show, hide or rename Sessions, Speakers and My agenda.
  Home and Guide always remain.
- **Home shortcuts, home links and the Guide tab**: which entries appear, their order, titles and subtitles.
  A **Custom link** entry opens any `https:`, `mailto:` or `tel:` address, for a help line, live stream or survey.
- **Headings & text**: every screen heading and intro, the home announcement, and call-to-action wording.
  A blank field keeps the built-in wording, shown in grey.
- **Venue & help**: address, arrival, accessibility, floor plan, help phone, help WhatsApp and help email, each shown only when filled in and not hidden.
- **Sponsors, partners, leaders, committee members, themes, speakers, sessions, activities and FAQs**: each entry has its own **Show in app** switch.

Session times are entered in India time and stored with `+05:30`; the app shows IST regardless of the device timezone.

The **Programme** section edits the timetable that both the app and the website's Programme page show.
Each session has a type, an optional label above its title (such as "Panel Discussion 1"), a track, and its people, each with the designation printed for that session and an optional link to the speaker directory.
Linked people show their portrait and profile, and the session appears on their speaker page.
A **Break or registration** entry is a divider in the timetable that attendees cannot save, rate or set reminders for.
A **Ceremony** has a running order: timed items, each with its own speakers, and untimed notes such as "Media interaction, if any".
Sessions are folded to one line; open one to edit it, and use **Sort by start time** after adding entries.
Both times can be left blank while unconfirmed.
Links must be public HTTPS URLs.
Theme images may be a bundled `assets/images/...` path or an HTTPS URL.

### Older app versions

Builds released before this editor ignore menus, headings and hidden-field controls but keep working: hidden fields reach them as empty strings and hidden entries are simply absent.
They still show “To be announced” for empty venue and session details.

## Mobile updates and offline behavior

Pull down on Home, Sessions, Activities, Venue or FAQs to refresh.
Returning to the app also refreshes content.
An unsuccessful refresh retains content already loaded in the current app session.
A fresh app launch without connectivity uses the bundled snapshot `assets/content/event.json`, which uses the built-in menus and headings.
Refresh it from `GET /api/v1/public/app-content` before a release if confirmed details must be available on a first offline launch.

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
