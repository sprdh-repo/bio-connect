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
- `committee.html` - the Advisory, Organising and Monitoring, and Programme Committees
  constituted by G.O.(Rt) No. 879/2026/ID.

Both pages share `styles.css` and `script.js`.
`script.js` is page-agnostic: the home-page-only widgets are feature-detected, and
scroll-spy only tracks nav links that point into the page you are on.

## Assets

The hero artwork in `assets/hero-biotech.webp` was generated specifically for this project with OpenAI image generation.

The Bio Connect logo and favicon are the official, unmodified brand assets from <https://bioconnect.kerala.gov.in/>.

The two leadership portraits on the committee page:

- `assets/leader-chief-minister.webp` - official portrait published by KSIDC,
  `https://www.ksidc.org/media/ministers/sre_VD-Satheesan_CywOjsa.avif`.
  Government of Kerala material; no attribution obligation.
- `assets/leader-industries-minister.webp` - Shihab tharayil,
  [CC BY-SA 4.0](https://creativecommons.org/licenses/by-sa/4.0/), via
  [Wikimedia Commons](https://commons.wikimedia.org/wiki/File:P._K._Kunhalikutty.jpg).

**The second one is licensed, not public-domain.** BY-SA requires the page to name the
photographer, link the licence, say the image was modified, and state that the adaptation
is shared under the same licence. That is the second line in the `.page-notes` block at
the foot of the committee page - if the photo stays, the line has to stay with it.
An official Government of Kerala portrait would remove the obligation; Kerala IT Mission
publishes one at
`https://itmission.kerala.gov.in/sites/default/files/inline-images/P K Kunhalikutty.jpg`.

Both are cropped to 4:5 at a matched head size and given the same forest-green duotone,
so two portraits shot on different days read as one set. Regenerate them from the
originals rather than editing the `.webp` files by hand.

## Deploy

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
