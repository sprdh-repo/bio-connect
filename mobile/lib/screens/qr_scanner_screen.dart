import 'dart:async';
import 'dart:math' as math;

import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:mobile_scanner/mobile_scanner.dart';
import 'package:permission_handler/permission_handler.dart';

import '../main.dart';
import '../widgets/interaction.dart';
import '../widgets/motion.dart';

/// Full-screen QR scanner for passes and badges. The camera fills the screen
/// under a forest wash with a rounded viewfinder; only codes inside it are
/// read. [accept] turns a scanned value into the result, or null when the
/// code is not one this screen is for, which shows [invalid].
class QrScannerScreen extends StatefulWidget {
  const QrScannerScreen({
    super.key,
    required this.title,
    required this.eyebrow,
    required this.instructions,
    required this.invalid,
    required this.fallback,
    required this.cameraHelp,
    required this.accept,
  });
  final String title, eyebrow, instructions, invalid, fallback, cameraHelp;
  final String? Function(String raw) accept;
  @override
  State<QrScannerScreen> createState() => _QrScannerScreenState();
}

class _QrScannerScreenState extends State<QrScannerScreen>
    with TickerProviderStateMixin, WidgetsBindingObserver {
  final _camera = MobileScannerController(
    // Android remembers duplicates before applying the scan-window filter.
    // Allow another read when a badge moves from outside into the frame.
    detectionSpeed: DetectionSpeed.normal,
    formats: const [BarcodeFormat.qrCode],
  );
  late final _sweep = AnimationController(
    vsync: this,
    duration: const Duration(milliseconds: 2200),
  )..repeat(reverse: true);
  late final _shake = AnimationController(
    vsync: this,
    duration: const Duration(milliseconds: 420),
  );
  late final _success = AnimationController(
    vsync: this,
    duration: const Duration(milliseconds: 380),
  );
  String? _error;
  Timer? _clearError;
  bool _done = false;
  Future<void> _cameraOperations = Future.value();

  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addObserver(this);
  }

  @override
  void didChangeAppLifecycleState(AppLifecycleState state) {
    // Permission dialogs can interrupt the initial camera start.
    // Let that start finish before attempting another camera operation.
    if (_done || !_camera.value.isInitialized) {
      return;
    }
    final resume = state == AppLifecycleState.resumed;
    // On resume, also retry after granting permission in phone settings.
    if (!resume && !_camera.value.hasCameraPermission) return;
    // Serialize stop/start so a quick return cannot race camera shutdown.
    _cameraOperations = _cameraOperations.then((_) async {
      if (!mounted || _done) return;
      try {
        if (resume) {
          await _camera.start();
        } else {
          await _camera.stop();
        }
      } on MobileScannerException catch (error) {
        debugPrint('Scanner lifecycle update failed: $error');
      }
    });
  }

  @override
  void dispose() {
    WidgetsBinding.instance.removeObserver(this);
    _clearError?.cancel();
    _camera.dispose();
    _sweep.dispose();
    _shake.dispose();
    _success.dispose();
    super.dispose();
  }

  void _detect(BarcodeCapture capture) {
    if (_done) return;
    for (final barcode in capture.barcodes) {
      final value = widget.accept(barcode.rawValue ?? '');
      if (value != null) {
        _done = true;
        AppFeedback.success();
        _sweep.stop();
        _camera.stop();
        setState(() => _error = null);
        _success.forward().then((_) {
          if (mounted) Navigator.pop(context, value);
        });
        return;
      }
    }
    // Repeated reads of the same wrong code only extend the message.
    _clearError?.cancel();
    _clearError = Timer(const Duration(seconds: 3), () {
      if (mounted) setState(() => _error = null);
    });
    if (_error == null) {
      HapticFeedback.lightImpact();
      _shake.forward(from: 0);
      setState(() => _error = widget.invalid);
    }
  }

  @override
  Widget build(BuildContext context) {
    final media = MediaQuery.of(context);
    final side = math.min(media.size.width * .72, 290.0);
    final window = Rect.fromCenter(
      center: Offset(
        media.size.width / 2,
        media.size.height * .42 - media.padding.bottom / 2,
      ),
      width: side,
      height: side,
    );
    return AnnotatedRegion<SystemUiOverlayStyle>(
      value: SystemUiOverlayStyle.light,
      child: Scaffold(
        backgroundColor: deepForest,
        body: Stack(
          fit: StackFit.expand,
          children: [
            MobileScanner(
              controller: _camera,
              scanWindow: window,
              onDetect: _detect,
              placeholderBuilder: (_) => const ColoredBox(
                color: deepForest,
                child: Center(
                  child: BioLoader(onDark: true, label: 'Starting camera'),
                ),
              ),
              errorBuilder: (context, error) => _CameraUnavailable(
                help: widget.cameraHelp,
                fallback: widget.fallback,
                denied:
                    error.errorCode == MobileScannerErrorCode.permissionDenied,
              ),
            ),
            // Drawn over the preview rather than in the plugin's overlay
            // slot, so the viewfinder shares the screen's coordinates with
            // the scan window. Hidden when the camera cannot start.
            ValueListenableBuilder(
              valueListenable: _camera,
              builder: (context, state, _) => state.error != null
                  ? const SizedBox.shrink()
                  : AnimatedBuilder(
                      animation: Listenable.merge([_sweep, _shake, _success]),
                      builder: (context, _) => CustomPaint(
                        size: Size.infinite,
                        painter: _ViewfinderPainter(
                          window: window,
                          sweep: Curves.easeInOutSine.transform(_sweep.value),
                          shake: _shake.isAnimating
                              ? math.sin(_shake.value * math.pi * 6) *
                                    (1 - _shake.value) *
                                    8
                              : 0,
                          success: Curves.easeOutCubic.transform(
                            _success.value,
                          ),
                          error: _error != null,
                        ),
                      ),
                    ),
            ),
            // A check settles into the viewfinder once a code is accepted.
            Positioned.fromRect(
              rect: window,
              child: Center(
                child: ScaleTransition(
                  scale: CurvedAnimation(
                    parent: _success,
                    curve: Curves.easeOutBack,
                  ),
                  child: Container(
                    width: 72,
                    height: 72,
                    decoration: const BoxDecoration(
                      color: lime,
                      shape: BoxShape.circle,
                    ),
                    child: const Icon(
                      Icons.check_rounded,
                      color: forest,
                      size: 40,
                    ),
                  ),
                ),
              ),
            ),
            Align(
              alignment: Alignment.topCenter,
              child: _TopBar(title: widget.title, camera: _camera),
            ),
            ValueListenableBuilder(
              valueListenable: _camera,
              builder: (context, state, _) => state.error != null
                  ? const SizedBox.shrink()
                  : Align(
                      alignment: Alignment.bottomCenter,
                      child: _InstructionCard(
                        eyebrow: widget.eyebrow,
                        instructions: widget.instructions,
                        error: _error,
                        fallback: widget.fallback,
                      ),
                    ),
            ),
          ],
        ),
      ),
    );
  }
}

class _TopBar extends StatelessWidget {
  const _TopBar({required this.title, required this.camera});
  final String title;
  final MobileScannerController camera;
  @override
  Widget build(BuildContext context) => SafeArea(
    child: Padding(
      padding: const EdgeInsets.fromLTRB(12, 8, 12, 0),
      child: Row(
        children: [
          _RoundButton(
            icon: Icons.arrow_back_rounded,
            tooltip: 'Back',
            onPressed: () => Navigator.maybePop(context),
          ),
          Expanded(
            child: Text(
              title,
              textAlign: TextAlign.center,
              style: const TextStyle(
                fontFamily: 'Manrope',
                color: Colors.white,
                fontSize: 18,
              ),
            ),
          ),
          ValueListenableBuilder(
            valueListenable: camera,
            builder: (context, state, _) => switch (state.torchState) {
              TorchState.unavailable => const SizedBox(width: 48),
              final torch => _RoundButton(
                icon: torch == TorchState.on
                    ? Icons.flashlight_on_rounded
                    : Icons.flashlight_off_rounded,
                tooltip: torch == TorchState.on
                    ? 'Turn off the torch'
                    : 'Turn on the torch',
                active: torch == TorchState.on,
                onPressed: () {
                  AppFeedback.selection();
                  camera.toggleTorch();
                },
              ),
            },
          ),
        ],
      ),
    ),
  );
}

class _RoundButton extends StatelessWidget {
  const _RoundButton({
    required this.icon,
    required this.tooltip,
    required this.onPressed,
    this.active = false,
  });
  final IconData icon;
  final String tooltip;
  final VoidCallback onPressed;
  final bool active;
  @override
  Widget build(BuildContext context) => Material(
    color: active ? lime : Colors.black.withValues(alpha: .32),
    shape: const CircleBorder(),
    child: IconButton(
      tooltip: tooltip,
      onPressed: onPressed,
      color: active ? forest : Colors.white,
      icon: Icon(icon),
    ),
  );
}

class _InstructionCard extends StatelessWidget {
  const _InstructionCard({
    required this.eyebrow,
    required this.instructions,
    required this.error,
    required this.fallback,
  });
  final String eyebrow, instructions, fallback;
  final String? error;
  @override
  Widget build(BuildContext context) => SafeArea(
    minimum: const EdgeInsets.all(16),
    child: Container(
      width: double.infinity,
      padding: const EdgeInsets.fromLTRB(20, 18, 20, 8),
      decoration: BoxDecoration(
        color: paper,
        borderRadius: BorderRadius.circular(26),
        boxShadow: [
          BoxShadow(
            color: Colors.black.withValues(alpha: .25),
            blurRadius: 24,
            offset: const Offset(0, 10),
          ),
        ],
      ),
      child: Column(
        mainAxisSize: MainAxisSize.min,
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            children: [
              Container(
                width: 40,
                height: 40,
                decoration: BoxDecoration(
                  color: cream,
                  borderRadius: BorderRadius.circular(13),
                ),
                child: const Icon(
                  Icons.qr_code_scanner_rounded,
                  color: forest,
                  size: 21,
                ),
              ),
              const SizedBox(width: 12),
              Expanded(child: Eyebrow(eyebrow)),
            ],
          ),
          const SizedBox(height: 12),
          Text(instructions, style: const TextStyle(height: 1.45)),
          AnimatedSize(
            duration: const Duration(milliseconds: 220),
            curve: Curves.easeOutCubic,
            child: error == null
                ? const SizedBox(width: double.infinity)
                : Padding(
                    padding: const EdgeInsets.only(top: 12),
                    child: Semantics(
                      liveRegion: true,
                      child: Container(
                        width: double.infinity,
                        padding: const EdgeInsets.symmetric(
                          horizontal: 12,
                          vertical: 10,
                        ),
                        decoration: BoxDecoration(
                          color: gold.withValues(alpha: .2),
                          borderRadius: BorderRadius.circular(12),
                        ),
                        child: Row(
                          children: [
                            const Icon(
                              Icons.info_outline_rounded,
                              size: 18,
                              color: ink,
                            ),
                            const SizedBox(width: 8),
                            Expanded(
                              child: Text(
                                error!,
                                style: const TextStyle(
                                  fontSize: 13,
                                  height: 1.35,
                                ),
                              ),
                            ),
                          ],
                        ),
                      ),
                    ),
                  ),
          ),
          const SizedBox(height: 4),
          Align(
            alignment: Alignment.centerLeft,
            child: TextButton(
              onPressed: () => Navigator.pop(context),
              child: Text(fallback),
            ),
          ),
        ],
      ),
    ),
  );
}

class _CameraUnavailable extends StatelessWidget {
  const _CameraUnavailable({
    required this.help,
    required this.fallback,
    required this.denied,
  });
  final String help, fallback;
  final bool denied;
  @override
  Widget build(BuildContext context) => ColoredBox(
    color: deepForest,
    child: SafeArea(
      child: Padding(
        padding: const EdgeInsets.fromLTRB(28, 72, 28, 28),
        child: Column(
          mainAxisAlignment: MainAxisAlignment.center,
          children: [
            Container(
              width: 76,
              height: 76,
              decoration: BoxDecoration(
                color: Colors.white.withValues(alpha: .08),
                shape: BoxShape.circle,
              ),
              child: const Icon(
                Icons.no_photography_outlined,
                color: lime,
                size: 34,
              ),
            ),
            const SizedBox(height: 20),
            const Text(
              'Camera unavailable',
              style: TextStyle(
                fontFamily: 'Manrope',
                color: Colors.white,
                fontSize: 22,
              ),
            ),
            const SizedBox(height: 10),
            Text(
              help,
              textAlign: TextAlign.center,
              style: const TextStyle(color: Colors.white70, height: 1.5),
            ),
            const SizedBox(height: 24),
            if (denied) ...[
              FilledButton.icon(
                onPressed: openAppSettings,
                style: FilledButton.styleFrom(
                  backgroundColor: lime,
                  foregroundColor: forest,
                ),
                icon: const Icon(Icons.settings_outlined),
                label: const Text('Open settings'),
              ),
              const SizedBox(height: 8),
            ],
            TextButton(
              onPressed: () => Navigator.pop(context),
              style: TextButton.styleFrom(foregroundColor: Colors.white),
              child: Text(fallback),
            ),
          ],
        ),
      ),
    ),
  );
}

/// Dims everything but the viewfinder, draws its lime corners, and sweeps a
/// scan line through it. The corners close into a full border on success
/// and turn gold, with a shake, for a code this screen does not accept.
class _ViewfinderPainter extends CustomPainter {
  _ViewfinderPainter({
    required this.window,
    required this.sweep,
    required this.shake,
    required this.success,
    required this.error,
  });
  final Rect window;
  final double sweep, shake, success;
  final bool error;

  @override
  void paint(Canvas canvas, Size size) {
    final frame = RRect.fromRectAndRadius(
      window.shift(Offset(shake, 0)),
      const Radius.circular(28),
    );
    canvas.drawPath(
      Path.combine(
        PathOperation.difference,
        Path()..addRect(Offset.zero & size),
        Path()..addRRect(frame),
      ),
      Paint()..color = deepForest.withValues(alpha: .72),
    );
    final colour = error ? gold : lime;
    final stroke = Paint()
      ..color = colour
      ..style = PaintingStyle.stroke
      ..strokeWidth = 5
      ..strokeCap = StrokeCap.round;
    // Each corner is a quarter of the rounded frame plus a straight run,
    // which grows until the corners meet on success.
    final r = frame.outerRect;
    final arm = 34 + (r.width / 2 - 34) * success;
    const radius = 28.0;
    for (final (cx, cy, sx, sy) in [
      (r.left, r.top, 1.0, 1.0),
      (r.right, r.top, -1.0, 1.0),
      (r.right, r.bottom, -1.0, -1.0),
      (r.left, r.bottom, 1.0, -1.0),
    ]) {
      final path = Path()
        ..moveTo(cx, cy + sy * arm)
        ..lineTo(cx, cy + sy * radius)
        ..arcToPoint(
          Offset(cx + sx * radius, cy),
          radius: const Radius.circular(radius),
          clockwise: sx * sy > 0,
        )
        ..lineTo(cx + sx * arm, cy);
      canvas.drawPath(path, stroke);
    }
    if (success > 0 || error) return;
    // The scan line, fading at its ends.
    final y = r.top + 18 + (r.height - 36) * sweep;
    final line = Rect.fromLTRB(r.left + 18, y - 1.5, r.right - 18, y + 1.5);
    canvas.drawRRect(
      RRect.fromRectAndRadius(line, const Radius.circular(2)),
      Paint()
        ..shader = LinearGradient(
          colors: [lime.withValues(alpha: 0), lime, lime.withValues(alpha: 0)],
        ).createShader(line),
    );
    // A soft glow trailing the line.
    final glow = Rect.fromLTRB(line.left, y - 26, line.right, y);
    canvas.drawRect(
      glow,
      Paint()
        ..shader = LinearGradient(
          begin: Alignment.bottomCenter,
          end: Alignment.topCenter,
          colors: [lime.withValues(alpha: .16), lime.withValues(alpha: 0)],
        ).createShader(glow),
    );
  }

  @override
  bool shouldRepaint(_ViewfinderPainter old) =>
      old.window != window ||
      old.sweep != sweep ||
      old.shake != shake ||
      old.success != success ||
      old.error != error;
}
