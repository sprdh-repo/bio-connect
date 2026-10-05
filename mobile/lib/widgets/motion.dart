import 'dart:math' as math;

import 'package:custom_refresh_indicator/custom_refresh_indicator.dart';
import 'package:flutter/material.dart';

import '../main.dart';

/// A double helix of base pairs. [phase] turns the helix; [progress] reveals
/// base pairs from left to right, so the same drawing grows during a pull and
/// spins while loading.
class HelixPainter extends CustomPainter {
  const HelixPainter({
    required this.phase,
    this.progress = 1,
    this.front = forest,
    this.back = gold,
    this.rung = const Color(0x33616F69),
    this.pairs = 8,
  });
  final double phase, progress;
  final Color front, back, rung;
  final int pairs;

  @override
  void paint(Canvas canvas, Size size) {
    final cy = size.height / 2;
    final amp = size.height * .3;
    final step = size.width / pairs;
    final dot = math.min(step * .34, size.height * .11);
    final rungPaint = Paint()
      ..strokeWidth = math.max(1, dot * .38)
      ..strokeCap = StrokeCap.round;
    for (var i = 0; i < pairs; i++) {
      final reveal = (progress * pairs - i).clamp(0.0, 1.0);
      if (reveal <= 0) continue;
      final x = step * (i + .5);
      final t = i * (2 * math.pi / pairs) + phase;
      final s = math.sin(t), c = math.cos(t);
      final a = Offset(x, cy + amp * s * reveal);
      final b = Offset(x, cy - amp * s * reveal);
      rungPaint.color = rung.withValues(alpha: rung.a * reveal);
      canvas.drawLine(a, b, rungPaint);
      // The strand nearer the viewer is larger and drawn last.
      final ra = dot * (.72 + .28 * c) * reveal;
      final rb = dot * (.72 - .28 * c) * reveal;
      final pa = Paint()
        ..color = front.withValues(alpha: .55 + .45 * (c + 1) / 2);
      final pb = Paint()
        ..color = back.withValues(alpha: .55 + .45 * (1 - c) / 2);
      if (c >= 0) {
        canvas.drawCircle(b, rb, pb);
        canvas.drawCircle(a, ra, pa);
      } else {
        canvas.drawCircle(a, ra, pa);
        canvas.drawCircle(b, rb, pb);
      }
    }
  }

  @override
  bool shouldRepaint(HelixPainter old) =>
      old.phase != phase ||
      old.progress != progress ||
      old.front != front ||
      old.back != back ||
      old.pairs != pairs;
}

/// The app's loading indicator: a turning double helix.
class BioLoader extends StatefulWidget {
  const BioLoader({
    super.key,
    this.width = 72,
    this.label,
    this.onDark = false,
  });
  final double width;
  final String? label;
  final bool onDark;
  @override
  State<BioLoader> createState() => _BioLoaderState();
}

class _BioLoaderState extends State<BioLoader>
    with SingleTickerProviderStateMixin {
  late final _spin = AnimationController(
    vsync: this,
    duration: const Duration(milliseconds: 1600),
  )..repeat();

  @override
  void dispose() {
    _spin.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final helix = SizedBox(
      width: widget.width,
      height: widget.width * .55,
      child: AnimatedBuilder(
        animation: _spin,
        builder: (_, _) => CustomPaint(
          painter: HelixPainter(
            phase: _spin.value * 2 * math.pi,
            front: widget.onDark ? lime : forest,
            back: gold,
            rung: widget.onDark
                ? const Color(0x40FFFFFF)
                : const Color(0x33616F69),
          ),
        ),
      ),
    );
    return Semantics(
      label: widget.label ?? 'Loading',
      liveRegion: true,
      child: Column(
        mainAxisSize: MainAxisSize.min,
        children: [
          helix,
          if (widget.label != null) ...[
            const SizedBox(height: 12),
            Text(
              widget.label!,
              textAlign: TextAlign.center,
              style: TextStyle(
                color: widget.onDark ? Colors.white70 : muted,
                fontSize: 12,
                letterSpacing: .2,
              ),
            ),
          ],
        ],
      ),
    );
  }
}

/// Pull to refresh: the page eases down and a helix grows as it is pulled,
/// then turns while the refresh runs.
class BioRefresh extends StatelessWidget {
  const BioRefresh({super.key, required this.onRefresh, required this.child});
  final Future<void> Function() onRefresh;
  final Widget child;

  static const _extent = 88.0;

  @override
  Widget build(BuildContext context) => CustomRefreshIndicator(
    onRefresh: onRefresh,
    offsetToArmed: _extent,
    durations: const RefreshIndicatorDurations(
      cancelDuration: Duration(milliseconds: 260),
      settleDuration: Duration(milliseconds: 220),
      finalizeDuration: Duration(milliseconds: 260),
    ),
    builder: (context, child, controller) =>
        _RefreshFrame(controller: controller, child: child),
    child: child,
  );
}

class _RefreshFrame extends StatefulWidget {
  const _RefreshFrame({required this.controller, required this.child});
  final IndicatorController controller;
  final Widget child;
  @override
  State<_RefreshFrame> createState() => _RefreshFrameState();
}

class _RefreshFrameState extends State<_RefreshFrame>
    with SingleTickerProviderStateMixin {
  late final _spin = AnimationController(
    vsync: this,
    duration: const Duration(milliseconds: 1400),
  );

  @override
  void initState() {
    super.initState();
    widget.controller.addListener(_sync);
  }

  @override
  void didUpdateWidget(_RefreshFrame old) {
    super.didUpdateWidget(old);
    if (old.controller != widget.controller) {
      old.controller.removeListener(_sync);
      widget.controller.addListener(_sync);
    }
  }

  void _sync() {
    final busy = widget.controller.isLoading || widget.controller.isSettling;
    if (busy && !_spin.isAnimating) _spin.repeat();
    if (widget.controller.isIdle && _spin.isAnimating) _spin.stop();
  }

  @override
  void dispose() {
    widget.controller.removeListener(_sync);
    _spin.dispose();
    super.dispose();
  }

  String _caption(IndicatorController c) {
    if (c.isLoading || c.isSettling) return 'Refreshing';
    if (c.isArmed) return 'Release to refresh';
    return 'Pull to refresh';
  }

  @override
  Widget build(BuildContext context) {
    final c = widget.controller;
    return AnimatedBuilder(
      animation: Listenable.merge([c, _spin]),
      builder: (context, child) {
        final pull = c.value.clamp(0.0, 1.25);
        final shown = pull.clamp(0.0, 1.0);
        final growing = c.isDragging || c.isArmed || c.isCanceling;
        return Stack(
          clipBehavior: Clip.hardEdge,
          children: [
            if (!c.isIdle)
              Positioned(
                top: BioRefresh._extent * (pull - 1),
                left: 0,
                right: 0,
                height: BioRefresh._extent,
                child: Opacity(
                  opacity: Curves.easeOut.transform(shown),
                  child: Center(
                    child: Column(
                      mainAxisSize: MainAxisSize.min,
                      children: [
                        SizedBox(
                          width: 64,
                          height: 30,
                          child: CustomPaint(
                            painter: HelixPainter(
                              progress: growing ? shown : 1,
                              phase:
                                  _spin.value * 2 * math.pi +
                                  (growing ? pull * math.pi : 0),
                            ),
                          ),
                        ),
                        if (shown > .55) ...[
                          const SizedBox(height: 6),
                          Text(
                            _caption(c),
                            style: const TextStyle(
                              color: muted,
                              fontSize: 10,
                              fontWeight: FontWeight.w700,
                              letterSpacing: 1.1,
                            ),
                          ),
                        ],
                      ],
                    ),
                  ),
                ),
              ),
            Transform.translate(
              offset: Offset(0, BioRefresh._extent * pull),
              child: child,
            ),
          ],
        );
      },
      child: widget.child,
    );
  }
}

/// Fades and lifts its child into place once, staggered by [order].
class Reveal extends StatefulWidget {
  const Reveal({
    super.key,
    required this.child,
    this.order = 0,
    this.lift = 18,
  });
  final Widget child;
  final int order;
  final double lift;
  @override
  State<Reveal> createState() => _RevealState();
}

class _RevealState extends State<Reveal> with SingleTickerProviderStateMixin {
  static const _base = 460, _stagger = 70;
  late final int _delay = math.min(widget.order, 8) * _stagger;
  late final _controller = AnimationController(
    vsync: this,
    duration: Duration(milliseconds: _base + _delay),
  );
  late final _curve = CurvedAnimation(
    parent: _controller,
    curve: Interval(_delay / (_base + _delay), 1, curve: Curves.easeOutCubic),
  );

  @override
  void didChangeDependencies() {
    super.didChangeDependencies();
    if (_controller.isDismissed) {
      if (MediaQuery.maybeDisableAnimationsOf(context) ?? false) {
        _controller.value = 1;
      } else {
        _controller.forward();
      }
    }
  }

  @override
  void dispose() {
    _curve.dispose();
    _controller.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) => AnimatedBuilder(
    animation: _curve,
    builder: (_, child) => Opacity(
      opacity: _curve.value,
      child: Transform.translate(
        offset: Offset(0, widget.lift * (1 - _curve.value)),
        child: child,
      ),
    ),
    child: widget.child,
  );
}

/// Gently presses its child down while a finger is on it. Taps still reach
/// the child's own [InkWell] or button.
class Pressable extends StatefulWidget {
  const Pressable({super.key, required this.child, this.scale = .97});
  final Widget child;
  final double scale;
  @override
  State<Pressable> createState() => _PressableState();
}

class _PressableState extends State<Pressable> {
  bool _down = false;
  void _set(bool down) {
    if (_down != down) setState(() => _down = down);
  }

  @override
  Widget build(BuildContext context) => Listener(
    onPointerDown: (_) => _set(true),
    onPointerUp: (_) => _set(false),
    onPointerCancel: (_) => _set(false),
    child: AnimatedScale(
      scale: _down ? widget.scale : 1,
      duration: const Duration(milliseconds: 140),
      curve: Curves.easeOut,
      child: widget.child,
    ),
  );
}
