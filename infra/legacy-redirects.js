/* CloudFront viewer-request function on the default cache behaviour.

   bioconnect.kerala.gov.in hosted Bio Connect 3.0 until September 2026. Press coverage
   and directory listings still link to that site's pages, and those links now land on a
   404. Sending them to the 4.0 home page keeps the referral - and passes the link equity
   on to the page that replaced them.

   The paths below are the 3.0 pages Wayback recorded for the domain; anything else that
   is missing keeps returning the 404 that lets Google drop it from the index. */
var MOVED = [
  "/about",
  "/agenda",
  "/highlights",
  "/speakers",
  "/venue",
  "/delegate-registration",
  "/expo-registration",
  "/product-launch"
];

function handler(event) {
  var uri = event.request.uri;
  /* The 3.0 site answered on both /about and /about/, so both are matched. */
  var path = uri.length > 1 && uri.charAt(uri.length - 1) === "/" ? uri.slice(0, -1) : uri;

  if (MOVED.indexOf(path) === -1) {
    return event.request;
  }

  return {
    statusCode: 301,
    statusDescription: "Moved Permanently",
    headers: {
      location: { value: "/" },
      "cache-control": { value: "max-age=3600" }
    }
  };
}
