/* CloudFront viewer-request function on the default cache behaviour.

   bioconnect.kerala.gov.in hosted Bio Connect 3.0 until September 2026. Press coverage
   and directory listings still link to that site's pages, and those links now land on a
   404. Sending them to the 4.0 page that replaced them keeps the referral - and passes
   the link equity on.

   Each 3.0 page maps to the section of the single-page 4.0 site that covers the same
   ground, so someone following an old "Delegate Registration" link arrives at the
   registration block rather than the top of the page. Google drops the fragment when it
   consolidates the redirect, so the target is still the home page as far as the index is
   concerned; the fragment is there for the person clicking.

   The keys are the 3.0 pages Wayback recorded for the domain. Anything else that is
   missing keeps returning the 404 that lets search engines drop it from the index. */
var MOVED = {
  "/about": "/#about",
  "/agenda": "/#themes",
  "/highlights": "/#highlights",
  "/delegate-registration": "/#registration",
  "/expo-registration": "/#registration",
  "/product-launch": "/#registration",
  /* The 3.0 exhibitor list and photo gallery: the expo section and the past-edition
     highlights are the closest the 4.0 site has. */
  "/exhibitors": "/#focus",
  "/creatives": "/#highlights",
  /* No speakers list or venue section on the 4.0 site yet - the event bar in the
     hero carries the venue, so the top of the page is the honest answer. */
  "/speakers": "/",
  "/venue": "/"
};

function handler(event) {
  var uri = event.request.uri;
  /* The 3.0 site answered on both /about and /about/, so both are matched. */
  var path = uri.length > 1 && uri.charAt(uri.length - 1) === "/" ? uri.slice(0, -1) : uri;

  if (!Object.prototype.hasOwnProperty.call(MOVED, path)) {
    return event.request;
  }

  return {
    statusCode: 301,
    statusDescription: "Moved Permanently",
    headers: {
      location: { value: MOVED[path] },
      "cache-control": { value: "max-age=3600" }
    }
  };
}
