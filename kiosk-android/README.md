# Bio Connect Kiosk

Native Kotlin shell for the existing Bio Connect self-service badge kiosk.
The single Activity hosts the live web design at https://reg.bioconnect.kerala.gov.in/kiosk.
Native views provide tablet setup, PIN entry, and an offline retry screen only.
No website files or server workflows are changed by this project.

## Build and install

Use Android Studio with Android 17 SDK (API 37.0), SDK build tools, and JDK 17 or newer.
The project uses AGP 9.2.1, Gradle 9.4.1, and built-in Kotlin support.
Minimum Android version is Android 8.0 (API 26); target is Android 17 (API 37).
Set `sdk.dir` in a local, ignored `local.properties`, or set `ANDROID_HOME`.

```sh
cd kiosk-android
./gradlew :app:testDebugUnitTest :app:lintDebug :app:assembleDebug
adb install -r app/build/outputs/apk/debug/app-debug.apk
```

Release builds require these environment variables.
Use your retained signing key for every subsequent update; another key cannot update the installed app.
Keys, credentials, local SDK paths, build outputs, and artifacts are ignored by Git.

```sh
export KIOSK_KEYSTORE=/absolute/private/path/release.jks
export KIOSK_KEY_ALIAS=bioconnect-kiosk
# Set KIOSK_STORE_PASSWORD and KIOSK_KEY_PASSWORD securely in the environment.
./gradlew :app:assembleRelease :app:bundleRelease
```

Release outputs are `app/build/outputs/apk/release/app-release.apk` and `app/build/outputs/bundle/release/app-release.aab`.
Release builds shrink code and resources with R8; remote WebView debugging is enabled only in debug builds.
If signing variables are absent, Gradle produces an unsigned release; it never substitutes a debug key.

For this delivery, the generated private key and a mode-0600 `signing.env` are stored in `~/.local/share/bio-connect-kiosk/signing/`, outside this repository.
Back up both privately now; losing the key prevents in-place app updates.
Load those local credentials before a release rebuild with `source ~/.local/share/bio-connect-kiosk/signing/signing.env`.

## First-run tablet setup

1. Install the release APK and allow the camera permission prompt.
2. Set and confirm a 6-digit **tablet admin PIN**.
   This PIN is separate from the website’s staff passcode.
3. Choose a printer connection, select the paired/connected device or enter its IPv4 address, and save.
4. Detect printer resolution automatically, or override it to 203 or 300 dpi.
5. Print a test label and calibrate the label media if needed.
6. Open **staff web setup**, sign in with the shared website passcode, and choose **Start kiosk on this device**.
7. Tap **Start kiosk** in tablet setup to enter lock task mode.

To return to native setup, tap the upper-left 96 dp corner **five times within three seconds**, then enter the tablet admin PIN.
The website’s existing two-second logo hold and staff passcode menu continue to work inside the page.
Five incorrect native PIN attempts impose a one-minute lockout, persisted across app restarts.
The PIN uses a random salt and PBKDF2-HMAC-SHA256 with 120,000 iterations; only its hash and salt are stored in EncryptedSharedPreferences.
There is no default PIN or remote reset backdoor.
If the PIN is lost, clear application data using authorized tablet management, then set up the station again.

**Clear session** deletes WebView cookies, DOM storage, and cached pages after confirmation.
Printer settings and the native admin PIN remain intact.
Normal app pauses flush cookies; the WebView session otherwise survives app restarts.
Session expiration still follows the website’s cookie and authentication policy.

## Device-owner kiosk provisioning

Provision on a fresh dedicated tablet before adding Google accounts or other users.
Device-owner provisioning changes tablet management; do it only on the intended kiosk tablet.

```sh
adb install -r artifacts/bio-connect-kiosk-1.0.0.apk
adb shell dpm set-device-owner in.gov.kerala.bioconnect.kiosk/.KioskDeviceAdminReceiver
adb shell am start -n in.gov.kerala.bioconnect.kiosk/.MainActivity
```

When device owner, the app allowlists itself for lock task, disables lock-task system features, and registers itself as the persistent Home launcher.
Home and Recents are unavailable during kiosk lock; Back and back gestures are always consumed.
The boot receiver starts the app after `BOOT_COMPLETED`; the Home role provides another launch path after reboot.
Unlocking the tablet may still be required by Android if a device credential is configured.

Without device owner, `startLockTask()` uses Android screen pinning and may require confirmation.
Enable screen pinning in Android settings first.
Unmanaged Android may block background launches from boot receivers; select Bio Connect Kiosk as the default Home app for automatic return after boot.
Screen pinning has Android’s user-controlled escape mechanism and is less restrictive than device-owner kiosk mode.

**Exit kiosk lock** stops lock task after native PIN authentication.
It does not remove device ownership or the persistent Home assignment.
Use approved device-management tools to deprovision a tablet when required.

## Printer connections

Use a Zebra ZD-series printer in ZPL mode with 3 x 2 inch (76 x 51 mm) gap media.
203 dpi labels use 608 x 406 dots; 300 dpi labels use 900 x 600 dots, per the supplied web contract.
Configure one printer per tablet and keep other apps from writing to it.

- **Bluetooth Classic:** pair in Android Bluetooth settings before setup; select from paired devices and grant Nearby Devices/Bluetooth access on Android 12+.
  The app uses the standard SPP UUID `00001101-0000-1000-8000-00805f9b34fb`, not BLE.
- **USB:** attach through an OTG adapter or powered hub, choose the printer, and approve Android’s USB access prompt.
  Both bulk OUT and bulk IN endpoints are required because status queries are mandatory.
  USB permission may need to be granted again after unplugging or rebooting.
  The saved vendor/product ID is stable across USB re-enumeration; attach only one printer with that ID.
- **TCP/IP:** enter a literal IPv4 address; the printer must be reachable on port 9100.
  Reserve its IP in DHCP and use the same private LAN as the tablet.
  On Android 17+, allow the local network permission prompt when saving TCP setup.
  Port 9100 printing is unencrypted raw ZPL, so use a trusted printer network.

Bluetooth and USB transports stay open between jobs.
All transports reconnect on the next job/status check after disconnecting.
Jobs run serially off the main thread, with an eight-second deadline including queue wait.
A watchdog closes blocked sockets/USB connections rather than relying solely on coroutine cancellation.
Jobs are never automatically resent after any badge bytes may have reached the printer.
An admin status panel polls every five seconds.

Before each label, the app queries `~HS` and refuses to print when paused, out of labels/ribbon, head open, busy, or in an error state.
Resolution is queried with `! U1 getvar "head.resolution.in_dpi"`; a manual override is required if firmware does not return 203/300.
The native test label includes a border, checkerboard, and `BIO CONNECT TEST 203dpi` (or `300dpi`).
**Calibrate media** sends `~JC`; calibration itself may advance several labels as the printer locates the gap.

## JavaScript bridge

Update the live web page to use this contract before deploying the APK at the event.
The existing page’s RawBT intent is blocked by this shell until that web update is deployed.
This Android project does not change or deploy the live site.
Only the exact HTTPS site origin receives `window.BioConnectKiosk`; only main-frame messages are accepted.
There is no `addJavascriptInterface` fallback.
Update Android System WebView if web-message listeners are unsupported.

```js
BioConnectKiosk.onmessage = event => {
  const reply = JSON.parse(event.data);
  // info.printer.dpi selects the canvas size; printed.ok drives the web UI.
};
BioConnectKiosk.postMessage(JSON.stringify({type: "hello"}));
BioConnectKiosk.postMessage(JSON.stringify({type: "status"}));
BioConnectKiosk.postMessage(JSON.stringify({
  type: "print",
  jobId: crypto.randomUUID(),
  png: canvas.toDataURL("image/png").split(",")[1],
  widthDots: canvas.width,
  heightDots: canvas.height
}));
```

Example replies:

```json
{"type":"info","app":"1.0.0","printer":{"connected":true,"name":"ZD421","dpi":203,"connection":"bluetooth","ready":true,"problem":""}}
{"type":"printed","jobId":"a3f5d8c1-6559-4aa4-8e4e-31f7429e167b","ok":true,"message":"Badge sent to printer"}
{"type":"printed","jobId":"a3f5d8c1-6559-4aa4-8e4e-31f7429e167b","ok":false,"message":"Printer is out of labels"}
```

PNG pixels are decoded, composited onto white, thresholded at luminance < 128, and packed MSB-first with padded row bytes.
The app sends one `^GFA` graphic in `^XA^MNY^PW…^LL…^LH0,0^FO…^GFA,…^FS^PQ1^XZ` without feeds, cuts, or extra copies.
Mismatched dimensions are centered without scaling and logged with job ID and dimensions only.
An oversized PNG is cropped equally from its center to fit the label; a small PNG is centered with white margins.
Pixel dimensions must agree with the supplied width/height, each between 1 and 1200 dots.
Raw JSON is limited to 2.5 million characters.

All event job IDs are persisted as SQLite receipts with durable writes, including uncertain outcomes.
A successful duplicate gets the previous reply without sending another label.
A receipt is saved before sending bytes, so an interrupted/crashed job returns an uncertain-outcome warning on duplicate submission.
A fresh ID authorizes another physical label; staff must inspect the printer before explicitly retrying an uncertain job.
`ok:true` means the ZPL was sent and the printer returned a ready status with no remaining queued labels.
It cannot prove the badge physically emerged or was legible; that requires printer/tablet acceptance testing.

## Troubleshooting

- **Camera blocked:** allow Camera in Android app permissions, update Android System WebView, then reload the kiosk.
  The native permission grants video capture only, never microphone capture; the web page chooses the front-facing camera with `facingMode: "user"`.
- **Cannot choose Bluetooth printer:** pair it in Android settings, confirm it supports Classic SPP, and allow Bluetooth/Nearby Devices access.
- **USB permission error:** reopen native setup, choose the USB printer, and accept the system prompt again.
  Check the OTG cable and printer power.
- **Printer not responding:** check printer power/connection, Bluetooth range or TCP 9100 access, and whether another app holds the connection.
  A fresh job/status request reconnects; do not repeatedly resend badges without checking for an already printed label.
- **Out of labels / head open / paused / ribbon:** fix the stated printer condition, then refresh status or explicitly retry from the web flow.
- **Two labels per print:** the printer’s media settings, `^LL`, DPI, or gap calibration are wrong.
  Select the correct 203/300 dpi, load 3 x 2 inch gap labels, and calibrate media.
  Do not add feed commands or extra copies as a workaround.
- **Offline:** the full-screen retry view tries again every ten seconds and supports immediate retry or authenticated staff setup.
  HTTPS certificate errors are never bypassed.
- **Boot did not open kiosk:** confirm device ownership or the default Home choice; unmanaged Android restricts background launches.
- **No bridge:** update System WebView and confirm the live page uses the native contract instead of RawBT.

## Verification and hardware acceptance

```sh
./gradlew :app:testDebugUnitTest :app:lintDebug
# With an emulator or disposable Android test tablet attached:
./gradlew :app:connectedDebugAndroidTest
```

Unit tests cover monochrome packing, row padding, transparency, luminance threshold, 203/300 dpi label bounds, bridge parsing, status flags, and malicious origin variations.
Instrumented tests load a local HTML fixture under the production origin and print through a fake Zebra transport.
They verify hello/status replies, one-label ZPL, duplicate suppression, out-of-media failure, blocked-origin bridge injection, and real Android PNG decoding.
The fixture intercepts local HTML only; it never calls the event backend or prints to a real printer.

Before event use, verify these on the actual tablet and each printer model:

- 203 and 300 dpi output, QR scan readability, margins, no extra label, and media calibration.
- Bluetooth reconnect, USB reconnect/permission, TCP timeout, and real `~HS` firmware status responses.
- Head-open, paused, ribbon/media-out faults before and during printing.
- Front camera selection, beep playback, staff login cookies across force-stop/reboot, offline recovery, and the deployed web bridge.
- Device-owner provisioning, boot launch, Home/Recents lockout, swipe bars, and PIN-protected lock exit.

Bundled fonts are Manrope 800 and DM Sans, licensed under the SIL Open Font License; see `licenses/`.
