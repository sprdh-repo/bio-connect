# Bio Connect 4.0

Responsive landing page for Bio Connect 4.0, Kerala 2026.

## Run locally

This is a dependency-free static site. Serve the repository with any local HTTP server:

```sh
python -m http.server 4173
```

Then open <http://localhost:4173>.

## Pages

- `index.html` - the landing page.
- `speakers.html` - the speakers expected at the conclave, with a client-side search.
  The list is static markup; the count in the hero, the toolbar and the home page teaser is
  typed out, so update all three when speakers are added or removed.
- `committee.html` - the State leadership and the Advisory Committee constituted by
  G.O.(Rt) No. 879/2026/ID.
  It is titled *Leadership* everywhere in the UI; the filename is deliberately unchanged,
  because `/committee.html` is the URL already in the sitemap and in the index.
- `exhibitors.html` - searchable confirmed exhibitor profiles loaded from the registration backend at `/api/v1/public/exhibitors`.
  Only approved exhibitors are included; company names, descriptions and logos update without editing the site.
  Loading, empty, error/retry, search and logo fallback states are handled in `script.js`.
- `sponsors.html` - sponsor profiles, currently Kerala Rubber Limited.
  The supplied logo is preserved in `assets/kerala-rubber-logo.png`; profile information comes from [Kerala Rubber's official website](https://krl.kerala.gov.in/about.php).
- `404.html` - served by CloudFront for any unknown path.
  The domain previously hosted Bio Connect 3.0, so search engines still request old
  URLs like `/about/` and `/agenda/`; without this they get an S3 `AccessDenied` 403,
  which Google reads as "blocked" and keeps the stale entry alive instead of dropping it.

All pages share `styles.css` and `script.js`.
`script.js` is page-agnostic: the home-page-only widgets are feature-detected, and
scroll-spy only tracks nav links that point into the page you are on.

## Search engines

`robots.txt` and `sitemap.xml` are part of the deployed site and both name the canonical
host `https://bioconnect.kerala.gov.in`, which is the only hostname the distribution
answers on. Every page also carries a `rel=canonical` pointing at it.

`infra/legacy-redirects.js` is a CloudFront viewer-request function on the default cache
behaviour. The domain hosted Bio Connect 3.0 until September 2026, so press coverage still
links to pages like `/about` and `/speakers`. The function 301s each of them to the section
of the single-page 4.0 site that covers the same ground, so an old "Delegate Registration"
link lands on the registration block rather than the top of the page. Google drops the
fragment when it consolidates the redirect, so this changes nothing for the index - it is
for the person clicking. Anything else that is missing still returns a 404, which is what
lets search engines drop the old pages from their index.

The map covers every 3.0 page that Search Console still listed as indexed on 2026-09-07.
Adding a section to the page is a chance to give one of the unmapped URLs a real target:
`/venue` currently goes to the top of the page for want of anywhere better.
`/speakers` goes to `speakers.html`.

The 3.0 PDFs under `/assets/files/` - the old brochure, the stall layout and the
sponsorship details - are deliberately left to 404. Those exact files no longer exist, and
the 3.0 URLs are not worth reviving, so an honest "not found" beats redirecting a document
request to a marketing section. The 4.0 brochure is published at its own path,
`/assets/bio-connect-4-brochure.pdf`, and is linked from the Brochure section on the home
page rather than from any of the old URLs.

The function is deployed by hand, not by `scripts/deploy.sh`:

```sh
aws cloudfront create-function --name bioconnect4-legacy-redirects \
  --function-config Comment="301 Bio Connect 3.0 URLs to the 4.0 home page",Runtime=cloudfront-js-2.0 \
  --function-code fileb://infra/legacy-redirects.js
# then: test-function against a sample event, publish-function, and confirm the
# distribution still associates it with the default cache behaviour.
```

`index.html` carries two JSON-LD blocks. The `Event` one must keep its dates and venue in
step with the event bar in the hero; a mismatch between the two is what search engines
flag. The `WebSite` one exists only to set the site name shown above the search result -
without it Google falls back to the registrable domain and labels the site "Kerala-gov".
`og:site_name` and `application-name` back it up.

When the sitemap changes, submit it again in Google Search Console for the
`bioconnect.kerala.gov.in` property.

## Assets

The transparent Government of Kerala emblem in `assets/government-of-kerala-logo.png` is by Sanu N, via [Wikimedia Commons](https://commons.wikimedia.org/wiki/File:Government_of_Kerala_Logo.png), licensed under [CC BY-SA 4.0](https://creativecommons.org/licenses/by-sa/4.0/).
The image is unmodified; the logo links to its source and attribution.

The hero artwork in `assets/hero-biotech.webp` was generated specifically for this project with OpenAI image generation.

`assets/bio-connect-4-brochure.pdf` is the official 4-page A4 event brochure, downscaled
from the 10.5 MB press original to 2.0 MB (image resolution reduced to ~200 DPI, then
linearised for progressive loading). It is visually identical on screen and still prints
cleanly at A4. `assets/brochure-cover.webp` is a 900px render of its first page, used as
the preview on the home page. Regenerate both from a new press original rather than
editing them; the render pipeline is Ghostscript `-dPDFSETTINGS` downsampling plus
`qpdf --linearize`, and `pdftoppm -r 200` for the cover.

The Bio Connect logo and favicon are the official, unmodified brand assets from <https://bioconnect.kerala.gov.in/>.

`assets/icon-144.png`, `assets/icon-192.png` and `favicon.ico` are derived from the logo
rather than from `assets/favicon.png`: the shipped favicon is 110x114 and clipped on three
sides, and Google requires a square icon, ideally a multiple of 48px, or it declines to
show one. They are a 148px square crop of the sphere and molecule connector taken from
`assets/bio-connect-logo.png` at (158, 8) - the only part of the lockup that survives being
drawn at 16px. Regenerate them from the logo if the brand asset changes; do not upscale the
small ones.

The two portraits on `committee.html`:

- `assets/leader-chief-minister.webp` - official portrait published by KSIDC,
  `https://www.ksidc.org/media/ministers/sre_VD-Satheesan_CywOjsa.avif`.
  Government of Kerala material; no attribution obligation.
- `assets/leader-industries-minister.webp` - Shihab tharayil,
  [CC BY-SA 4.0](https://creativecommons.org/licenses/by-sa/4.0/), via
  [Wikimedia Commons](https://commons.wikimedia.org/wiki/File:P._K._Kunhalikutty.jpg).

**The second one is licensed, not public-domain.** BY-SA requires the page to name the
photographer, link the licence, say the image was modified, and state that the adaptation
is shared under the same licence. That is the second line in the `.page-notes` block at
the foot of `committee.html` - if the photo stays, the line has to stay with it.
An official Government of Kerala portrait would remove the obligation; Kerala IT Mission
publishes one at
`https://itmission.kerala.gov.in/sites/default/files/inline-images/P K Kunhalikutty.jpg`.

Both are cropped to 4:5 at a matched head size and given the same forest-green duotone,
so two portraits shot on different days read as one set. Regenerate them from the
originals rather than editing the `.webp` files by hand.

The speaker portraits in `assets/speakers/` come from the organisers' speaker spreadsheet.
Each is cropped to 4:5 around the detected face and given the same forest-green duotone as
the leadership portraits, at 480x600.
Regenerate them from the originals rather than editing the `.webp` files by hand.
A speaker without a usable photo gets a lettered placeholder (`.speaker-monogram`) until one
arrives.

Both the leadership and speaker portraits are produced with `scripts/portrait.py`:

```sh
python3 scripts/portrait.py path/to/original.jpg assets/speakers/name.webp
```

It face-detects the subject (OpenCV Haar cascade; pass `--face X,Y,W,H` to override when
detection picks the wrong face), crops to 4:5 with the face at a matched size and vertical
position, then applies the shared forest-green duotone: a three-stop gradient
(`#051c16` shadows, `#76967a` mid, `#e8eed8` highlights) over a 1st/99th-percentile
contrast stretch. Requires `opencv-python`, `numpy` and `Pillow`.

## Deploy

Deploy the registration backend before publishing the exhibitor directory for the first time; it provides the anonymous directory and logo endpoints.
The marketing site fetches these from `https://reg.bioconnect.kerala.gov.in` without credentials.
The directory response permits anonymous cross-origin reads and exposes no contact, attendee or payment fields.
Both directory and logo responses retain `Cache-Control: no-store` so approval changes take effect on the next request.
The legacy redirect function is still deployed separately, as described above.

Deploy the current working tree to the production S3 bucket and invalidate CloudFront:

```sh
./scripts/deploy.sh
```

The script requires authenticated AWS CLI credentials and `curl`.
It uploads only the website files and does not delete other objects from the bucket.

The production defaults can be overridden when needed:

```sh
BIOCONNECT_S3_BUCKET=example-bucket \
BIOCONNECT_CLOUDFRONT_DISTRIBUTION_ID=E123456789 \
BIOCONNECT_SITE_URL=https://example.com \
./scripts/deploy.sh
```
