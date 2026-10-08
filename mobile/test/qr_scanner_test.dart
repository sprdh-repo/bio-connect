import 'dart:async';

import 'package:bio_connect_app/screens/qr_scanner_screen.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:mobile_scanner/mobile_scanner.dart';

void main() {
  late ScannerPlatform camera;
  late MobileScannerPlatform original;
  setUp(() {
    original = MobileScannerPlatform.instance;
    camera = ScannerPlatform();
    MobileScannerPlatform.instance = camera;
    MobileScannerController.resetPlatformSessionOwner();
  });
  tearDown(() async {
    MobileScannerPlatform.instance = original;
    MobileScannerController.resetPlatformSessionOwner();
    await camera.captures.close();
  });

  Future<void> open(WidgetTester tester, List<String> results) async {
    addTearDown(() async {
      tester.binding.handleAppLifecycleStateChanged(AppLifecycleState.resumed);
      await tester.pumpWidget(const SizedBox());
      await tester.pump();
    });
    await tester.pumpWidget(
      MaterialApp(
        home: Builder(
          builder: (context) => TextButton(
            onPressed: () async {
              final value = await Navigator.of(context).push<String>(
                MaterialPageRoute(
                  builder: (_) => QrScannerScreen(
                    title: 'Scan a badge',
                    eyebrow: 'CONTACT',
                    instructions: 'Fit the QR inside the frame',
                    invalid: 'Invalid badge',
                    fallback: 'Back',
                    cameraHelp: 'Allow camera access',
                    accept: (raw) => raw == 'badge' ? raw : null,
                  ),
                ),
              );
              if (value != null) results.add(value);
            },
            child: const Text('Scan'),
          ),
        ),
      ),
    );
    await tester.tap(find.text('Scan'));
    await tester.pump();
    await tester.pump(const Duration(milliseconds: 400));
    await tester.pump();
  }

  testWidgets('badge first seen outside frame can be scanned inside it', (
    tester,
  ) async {
    final results = <String>[];
    await open(tester, results);
    camera.detect('badge', insideWindow: false);
    await tester.pump();
    camera.detect('badge', insideWindow: true);
    await tester.pump();
    await tester.pump(const Duration(milliseconds: 400));
    await tester.pump(const Duration(milliseconds: 400));
    expect(results, ['badge']);
    await tester.pumpWidget(const SizedBox());
    await tester.pump();
  });

  testWidgets('camera stops on interruption and scans after resuming', (
    tester,
  ) async {
    final results = <String>[];
    await open(tester, results);
    expect(camera.starts, 1);
    tester.binding.handleAppLifecycleStateChanged(AppLifecycleState.inactive);
    await tester.pump();
    expect(camera.stops, 1);
    tester.binding.handleAppLifecycleStateChanged(AppLifecycleState.paused);
    tester.binding.handleAppLifecycleStateChanged(AppLifecycleState.resumed);
    await tester.pump();
    expect(camera.starts, 2);
    camera.detect('badge', insideWindow: true);
    camera.detect('badge', insideWindow: true);
    await tester.pump();
    tester.binding.handleAppLifecycleStateChanged(AppLifecycleState.inactive);
    tester.binding.handleAppLifecycleStateChanged(AppLifecycleState.resumed);
    await tester.pump();
    expect(camera.starts, 2, reason: 'An accepted scan must not restart');
    await tester.pump(const Duration(milliseconds: 400));
    await tester.pump(const Duration(milliseconds: 400));
    expect(results, ['badge']);
    await tester.pumpWidget(const SizedBox());
    await tester.pump();
  });

  testWidgets('quick resume waits for camera shutdown to finish', (
    tester,
  ) async {
    await open(tester, <String>[]);
    final shutdown = Completer<void>();
    camera.shutdown = shutdown;
    tester.binding.handleAppLifecycleStateChanged(AppLifecycleState.inactive);
    await tester.pump();
    tester.binding.handleAppLifecycleStateChanged(AppLifecycleState.resumed);
    await tester.pump();
    expect(camera.starts, 1);
    shutdown.complete();
    await tester.pump();
    expect(camera.starts, 2);
  });
}

/// Models Android's duplicate check before its scan-window filter.
class ScannerPlatform extends MobileScannerPlatform {
  final captures = StreamController<BarcodeCapture?>.broadcast();
  StartOptions? options;
  String? lastSeen;
  int starts = 0, stops = 0;
  Completer<void>? shutdown;
  @override
  Stream<BarcodeCapture?> get barcodesStream => captures.stream;
  @override
  Stream<TorchState> get torchStateStream => const Stream.empty();
  @override
  Stream<double> get zoomScaleStateStream => const Stream.empty();
  @override
  Widget buildCameraView() => const SizedBox.expand();
  @override
  Future<MobileScannerViewAttributes> start(StartOptions startOptions) async {
    options = startOptions;
    lastSeen = null;
    starts++;
    return const MobileScannerViewAttributes(
      cameraDirection: CameraFacing.back,
      currentTorchMode: TorchState.unavailable,
      size: Size(640, 480),
    );
  }

  @override
  Future<void> stop() async {
    stops++;
    await shutdown?.future;
  }

  @override
  Future<void> updateScanWindow(Rect? window) async {}
  @override
  Future<void> dispose() async {}
  void detect(String value, {required bool insideWindow}) {
    if (options?.detectionSpeed == DetectionSpeed.noDuplicates) {
      if (lastSeen == value) return;
      lastSeen = value;
    }
    if (insideWindow) {
      captures.add(BarcodeCapture(barcodes: [Barcode(rawValue: value)]));
    }
  }
}
