import 'dart:math' as math;

import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:provider/provider.dart';

import '../main.dart';
import '../providers/content_provider.dart';
import 'motion.dart';

/// Shows the animated splash over [child] until the event guide is ready.
///
/// The first frame repeats the native launch screen exactly (the mark,
/// 240 logical pixels square, centred on cream), so the hand-over from the
/// platform is seamless. The mark then settles upwards while rings pulse
/// out, the helix from the app's loader draws itself in, and the wordmark
/// and event details rise. Once content is ready and the intro has played,
/// the splash lifts away. [child] builds underneath from the start, so the
/// splash never adds to loading time beyond its short intro.
class SplashGate extends StatefulWidget {
  const SplashGate({super.key, required this.child});
  final Widget child;
  @override
  State<SplashGate> createState() => _SplashGateState();
}

class _SplashGateState extends State<SplashGate> with TickerProviderStateMixin {
  late final _intro = AnimationController(
    vsync: this,
    // The build-up takes 1.8 seconds; the remaining 0.7 seconds hold the
    // finished splash so the dates and venue can be read before it leaves.
    duration: const Duration(milliseconds: 2500),
  );
  late final _spin = AnimationController(
    vsync: this,
    duration: const Duration(milliseconds: 1600),
  )..repeat();
  late final _drift = AnimationController(
    vsync: this,
    duration: const Duration(seconds: 9),
  )..repeat();
  late final _exit = AnimationController(
    vsync: this,
    duration: const Duration(milliseconds: 520),
  );
  bool _done = false, _started = false, _queued = false;

  @override
  void didChangeDependencies() {
    super.didChangeDependencies();
    if (_started) return;
    _started = true;
    // Reduced motion shows the finished composition and simply fades.
    if (MediaQuery.disableAnimationsOf(context)) {
      _intro.value = 1;
      _exit.duration = const Duration(milliseconds: 200);
    }
    _intro.addStatusListener((_) => _advance());
    _exit.addStatusListener((status) {
      if (status == AnimationStatus.completed && mounted) {
        _spin.stop();
        _drift.stop();
        setState(() => _done = true);
      }
    });
  }

  /// Plays the build-up once content is ready, then leaves after it.
  /// Loading parses the event guide on the UI thread, so an intro playing
  /// meanwhile would skip frames; until then the still mark simply
  /// continues the native launch screen.
  void _advance() {
    if (!mounted || _exit.isAnimating || _exit.isCompleted) return;
    final state = context.read<ContentProvider>();
    if (state.content == null && state.error == null) return;
    if (_intro.isCompleted) {
      _exit.forward();
    } else if (!_queued) {
      // Android 12+ fades its own splash icon out after Flutter's first
      // frame; moving the mark only once that has finished avoids a
      // momentary double image.
      _queued = true;
      Future<void>.delayed(const Duration(milliseconds: 300), () {
        if (mounted) _intro.forward();
      });
    }
  }

  @override
  void dispose() {
    _intro.dispose();
    _spin.dispose();
    _drift.dispose();
    _exit.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    if (_done) return widget.child;
    // Re-check whenever content arrives.
    context.watch<ContentProvider>();
    WidgetsBinding.instance.addPostFrameCallback((_) => _advance());
    final event = context.read<ContentProvider>().content?.event;
    return Stack(
      fit: StackFit.expand,
      children: [
        widget.child,
        AnnotatedRegion<SystemUiOverlayStyle>(
          value: SystemUiOverlayStyle.dark,
          child: AnimatedBuilder(
            animation: _exit,
            builder: (context, child) {
              final t = Curves.easeInCubic.transform(_exit.value);
              return IgnorePointer(
                ignoring: _exit.value > 0,
                child: Opacity(
                  opacity: 1 - t,
                  child: Transform.scale(scale: 1 + .06 * t, child: child),
                ),
              );
            },
            child: Semantics(
              label: 'Bio Connect 4.0, loading',
              liveRegion: true,
              // Material gives the splash the app's text style.
              child: Material(
                color: cream,
                child: _SplashArt(
                  intro: _intro,
                  spin: _spin,
                  drift: _drift,
                  dates: event == null
                      ? ''
                      : eventDateRange(event, uppercase: true),
                  venue: event?.venue.toUpperCase() ?? '',
                ),
              ),
            ),
          ),
        ),
      ],
    );
  }
}

class _SplashArt extends StatelessWidget {
  const _SplashArt({
    required this.intro,
    required this.spin,
    required this.drift,
    required this.dates,
    required this.venue,
  });
  final AnimationController intro, spin, drift;
  final String dates, venue;

  /// The native launch screen draws the mark in a 240dp square.
  static const markBox = 240.0;

  double _at(double begin, double end, [Curve curve = Curves.easeOutCubic]) =>
      curve.transform(((intro.value - begin) / (end - begin)).clamp(0.0, 1.0));

  @override
  Widget build(BuildContext context) => AnimatedBuilder(
    animation: Listenable.merge([intro, spin, drift]),
    builder: (context, _) {
      final settle = _at(0, .314, Curves.easeInOutCubic);
      final helix = _at(.224, .571);
      final word = _at(.37, .616);
      final details = _at(.448, .683);
      final place = _at(.515, .717);
      Widget rise(double t, Widget child, {double by = 14}) => Opacity(
        opacity: t,
        child: Transform.translate(
          offset: Offset(0, by * (1 - t)),
          child: child,
        ),
      );
      Widget at(double dy, Widget child) => Align(
        child: Transform.translate(offset: Offset(0, dy), child: child),
      );
      return Stack(
        fit: StackFit.expand,
        children: [
          // A soft lime glow behind the composition.
          DecoratedBox(
            decoration: BoxDecoration(
              gradient: RadialGradient(
                center: const Alignment(0, -.12),
                radius: .9,
                colors: [
                  lime.withValues(alpha: .28 * settle),
                  cream,
                ],
              ),
            ),
          ),
          CustomPaint(
            painter: _ParticlePainter(drift.value, fade: _at(.146, .526)),
          ),
          // Two rings pulse out from the mark as it settles.
          at(
            -86 * settle,
            CustomPaint(
              size: const Size.square(markBox * 2),
              painter: _RingPainter([_at(.045, .526), _at(.168, .672)]),
            ),
          ),
          at(
            -86 * settle,
            Transform.scale(
              scale: 1 - .3 * settle,
              child: Image.asset(
                'assets/images/splash-mark.png',
                width: markBox,
                height: markBox,
                filterQuality: FilterQuality.medium,
              ),
            ),
          ),
          at(
            10,
            Opacity(
              opacity: helix,
              child: SizedBox(
                width: 176,
                height: 56,
                child: CustomPaint(
                  painter: HelixPainter(
                    phase: spin.value * 2 * math.pi,
                    progress: helix,
                    pairs: 10,
                  ),
                ),
              ),
            ),
          ),
          at(
            84,
            rise(
              word,
              Image.asset(
                'assets/images/bio-connect-logo.png',
                width: 210,
                filterQuality: FilterQuality.medium,
              ),
            ),
          ),
          if (dates.isNotEmpty)
            at(
              142,
              rise(
                details,
                Text(
                  dates,
                  style: const TextStyle(
                    color: forest,
                    fontSize: 12,
                    fontWeight: FontWeight.w700,
                    letterSpacing: 1.6,
                  ),
                ),
              ),
            ),
          if (venue.isNotEmpty)
            at(
              166,
              rise(
                place,
                Padding(
                  padding: const EdgeInsets.symmetric(horizontal: 32),
                  child: Text(
                    venue,
                    textAlign: TextAlign.center,
                    style: const TextStyle(
                      color: muted,
                      fontSize: 11,
                      fontWeight: FontWeight.w600,
                      letterSpacing: 1.4,
                    ),
                  ),
                ),
              ),
            ),
        ],
      );
    },
  );
}

/// Expanding rings, one per entry in [progress], each fading as it grows.
class _RingPainter extends CustomPainter {
  const _RingPainter(this.progress);
  final List<double> progress;
  @override
  void paint(Canvas canvas, Size size) {
    final center = size.center(Offset.zero);
    for (final (i, p) in progress.indexed) {
      if (p <= 0 || p >= 1) continue;
      canvas.drawCircle(
        center,
        56 + (size.width / 2 - 56) * p,
        Paint()
          ..style = PaintingStyle.stroke
          ..strokeWidth = 2.2 * (1 - p) + .4
          ..color = (i.isEven ? lime : forest).withValues(alpha: .45 * (1 - p)),
      );
    }
  }

  @override
  bool shouldRepaint(_RingPainter old) => true;
}

/// Small base pairs drifting upwards: two dots joined by a rung, in the
/// helix's colours, faint enough to stay behind the content.
class _ParticlePainter extends CustomPainter {
  _ParticlePainter(this.t, {required this.fade});
  final double t, fade;

  static final _seeds = List.generate(16, (i) {
    final r = math.Random(i * 7919 + 3);
    return (x: r.nextDouble(), y: r.nextDouble(), s: .6 + r.nextDouble() * .8);
  });

  @override
  void paint(Canvas canvas, Size size) {
    if (fade <= 0) return;
    for (final (i, p) in _seeds.indexed) {
      final y = ((p.y - t * (.35 + .25 * p.s)) % 1) * size.height;
      final x = p.x * size.width + math.sin((t + p.y) * 2 * math.pi) * 10;
      // Fade in from the bottom and out towards the top.
      final edge = math.min(y / size.height, 1 - y / size.height) * 5;
      final alpha = .2 * fade * edge.clamp(0.0, 1.0);
      final gap = 5 * p.s, dot = 2.1 * p.s;
      final angle = (t * 2 + p.x) * math.pi;
      final a = Offset(x + math.cos(angle) * gap, y + math.sin(angle) * gap);
      final b = Offset(x - math.cos(angle) * gap, y - math.sin(angle) * gap);
      canvas.drawLine(
        a,
        b,
        Paint()
          ..color = muted.withValues(alpha: alpha * .6)
          ..strokeWidth = 1,
      );
      canvas.drawCircle(
        a,
        dot,
        Paint()
          ..color = (i.isEven ? forest : gold).withValues(alpha: alpha * 2),
      );
      canvas.drawCircle(
        b,
        dot,
        Paint()..color = (i.isEven ? gold : lime).withValues(alpha: alpha * 2),
      );
    }
  }

  @override
  bool shouldRepaint(_ParticlePainter old) => old.t != t || old.fade != fade;
}
