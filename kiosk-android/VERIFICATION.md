# Verification record

Scope: `kiosk-android/` only.
Existing website changes in the working tree were not edited by this work.
No commit, push, deployment, live check-in, or real printer job was performed.

## Automated acceptance

- `:app:testDebugUnitTest`: eight tests cover row padding, MSB-first packing, alpha compositing, threshold boundaries, label dimensions/centering/cropping, bridge parsing, origin rules, and Zebra status errors.
- `:app:connectedDebugAndroidTest`: five tests on an Android 17/API 37 emulator cover real PNG decoding, trusted-origin bridge info/print replies, duplicate suppression, rejected untrusted-origin bridge injection, out-of-label errors, eight-second blocked connection/reconnect, durable uncertain receipts after restart, and receipt retention beyond 128 jobs.
- `:app:lintRelease`: release Android lint; **0 errors, 27 warnings**; warnings cover dependency updates, intentional durable preference writes, KTX suggestions, and English-only native UI text.
- `:app:assembleRelease` and `:app:bundleRelease`: signed artifacts with R8 code/resource shrinking.
- APK signing verified with `apksigner verify --verbose --print-certs`: valid APK Signature Scheme v2, RSA 3072.
- AAB verified with `jarsigner -verify`: `jar verified`; standard self-signed-certificate/no-timestamp warnings apply.
- APK manifest checked with `aapt dump badging`: correct package/name, minSdk 26, compile/targetSdk 37.
- The signed release installed and launched on the emulator; tablet portrait/landscape first-run and landscape admin layouts were visually checked.
- The final signed release’s two PIN inputs report `password=true` in Android UIAutomator, and their touch targets are 64 dp.

All eight unit tests, five instrumented tests, signed release tasks, and release lint passed on the final source.
Signing certificate SHA-256: `9b0678d1528a465c6d22ec12c3daa14bd873398870bac5dd4bda9bd7c26c95dd`.

The initial emulator runs failed because Android’s low-memory killer killed the WebView renderer.
The identical smoke test passed after restarting the test emulator with 4 GB RAM.
A subsequent validation run could not install the debug APK over the release APK because their signing certificates differ; after Gradle removed that disposable emulator installation, the smoke tests were rerun.
A debug-only fixture Activity avoids the event backend entirely; it is absent from the release APK.

## Guardrail pass-fail-pass evidence

`./gradlew :app:testDebugUnitTest` runs `permitsOnlyExactHttpsOriginPassFailPass`.
The approved HTTPS origin passes, injected HTTP/userinfo/lookalike-host/alternate-port/file/intent/javascript origins fail, and the restored approved kiosk URL passes.
No shared site or device policy is weakened for this test.

`./gradlew :app:connectedDebugAndroidTest` runs `localFixturePrintsOnceRepliesAndRejectsOtherOrigins`.
The local approved-origin fixture receives ready info and prints once, the injected `https://evil.test/` fixture has no `BioConnectKiosk` object, and the restored approved-origin fixture receives ready info again.
The test also injects an out-of-label status and observes a failure reply with no extra label.
All printing in these tests uses fake transports.

Two independent read-only reviews checked spec fidelity and security/concurrency.
Review fixes include Android 8 font compatibility, failure when bridge support is absent, synchronized PIN verification, durable event receipts, deadline checks/transport closing with coordinated watchdog completion, and PIN masking after single-line setup.

## Hardware checks still required

1. Actual Zebra 203/300 dpi status/SGD responses, Bluetooth/USB/TCP connections, permission persistence, and reconnects.
2. Physical one-label output, centered/cropped mismatches, QR readability, ribbon/media/head/paused failures, and gap calibration.
3. Actual tablet front camera, beep playback, station cookies across reboot, website’s deployed bridge, and offline recovery.
4. Device-owner provisioning, boot/Home behavior, screen pinning fallback, Home/Recents/back restrictions, immersive bars, and admin exit.

The native app cannot prove physical label delivery from status replies alone.
An interrupted write is not automatically retried; staff must inspect the printer before requesting a fresh job ID.
