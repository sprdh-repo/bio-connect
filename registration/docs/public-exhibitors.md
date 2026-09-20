# Public exhibitor directory

The marketing site's `exhibitors.html` consumes `GET /api/v1/public/exhibitors` without authentication or cookies.
Deploy the backend before publishing that page.

```json
{
  "exhibitors": [
    {
      "name": "Example organisation",
      "description": "Company profile supplied with registration.",
      "logo_url": "/api/v1/public/exhibitors/logos/opaque-file-id"
    }
  ]
}
```

Only registrations with `status=approved` in an exhibitor category are returned, ordered by organisation name and then registration ID.
There is one entry per approved registration, with no deduplication by name.
The response is an explicit public projection, not a serialised registration record.
It excludes contacts, attendees, payments, internal notes and registration identifiers.
An empty directory returns HTTP 200 with `{"exhibitors":[]}`.
A missing logo is represented by an empty `logo_url`.
The most recently uploaded logo is selected, with file ID as a deterministic tie-breaker.

`GET /api/v1/public/exhibitors/logos/{file}` streams the submitted PNG or JPEG from private storage.
Every request checks that the file is a logo attached to a currently approved exhibitor.
Receipts, passes, unknown files and logos belonging to unapproved or cancelled registrations return HTTP 404.
Storage or database failures return HTTP 503 with the normal JSON error format.
The logo route never redirects to private S3 objects.

Both routes retain the application's `Cache-Control: no-store` and `X-Content-Type-Options: nosniff` headers.
The JSON route adds `Access-Control-Allow-Origin: *` for anonymous public reads; credentialed cross-origin access is not enabled.
Images are embedded directly by the marketing page.
No database migration or new configuration is needed.

Integration coverage is in `internal/app/public_exhibitors_test.go`.
Use the existing test command with a disposable PostgreSQL database, because the integration harness drops its public schema.
