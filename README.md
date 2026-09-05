# Bio Connect 4.0

Responsive landing page for Bio Connect 4.0, Kerala 2026.

## Run locally

This is a dependency-free static site. Serve the repository with any local HTTP server:

```sh
python -m http.server 4173
```

Then open <http://localhost:4173>.

## Assets

The hero artwork in `assets/hero-biotech.webp` was generated specifically for this project with OpenAI image generation.

The Bio Connect logo and favicon are the official, unmodified brand assets from <https://bioconnect.kerala.gov.in/>.

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
