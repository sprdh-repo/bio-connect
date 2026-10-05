import 'package:flutter/material.dart';

import '../main.dart';
import 'interaction.dart';

/// Pull-to-refresh for pages backed by live data. [onRefresh] completes with
/// whether the backend was reached; on failure the page keeps what it shows and
/// a short notice explains that it is saved information.
class LiveRefresh extends StatelessWidget {
  const LiveRefresh({super.key, required this.onRefresh, required this.child});
  final Future<bool> Function() onRefresh;
  final Widget child;

  @override
  Widget build(BuildContext context) => RefreshIndicator(
    color: forest,
    backgroundColor: paper,
    onRefresh: () async {
      AppFeedback.selection();
      final ok = await onRefresh();
      if (!ok && context.mounted) {
        ScaffoldMessenger.of(context)
          ..hideCurrentSnackBar()
          ..showSnackBar(
            const SnackBar(
              behavior: SnackBarBehavior.floating,
              content: Text(
                'Could not reach Bio Connect. Showing saved information.',
              ),
            ),
          );
      }
    },
    child: child,
  );
}

/// A logo on a light panel, falling back to the organisation's initials.
class LogoBox extends StatelessWidget {
  const LogoBox({
    super.key,
    required this.url,
    required this.name,
    this.size = 56,
    this.height,
    this.radius = 14,
    this.padding = 8,
    this.color = Colors.white,
  });
  final String url, name;
  final double size;
  final double? height;
  final double radius, padding;
  final Color color;

  @override
  Widget build(BuildContext context) {
    final h = height ?? size;
    final fallback = Center(
      child: Text(
        initials(name),
        style: TextStyle(
          color: forest,
          fontFamily: 'Manrope',
          fontSize: (h * .34).clamp(14, 40),
        ),
      ),
    );
    return Container(
      width: size,
      height: h,
      padding: EdgeInsets.all(padding),
      decoration: BoxDecoration(
        color: color,
        borderRadius: BorderRadius.circular(radius),
        border: Border.all(color: forest.withValues(alpha: .08)),
      ),
      child: url.isEmpty
          ? fallback
          : Image.network(
              url,
              fit: BoxFit.contain,
              semanticLabel: '$name logo',
              frameBuilder: (_, child, frame, sync) => sync || frame != null
                  ? child
                  : const SkeletonBlock(radius: 8),
              errorBuilder: (_, _, _) => fallback,
            ),
    );
  }
}

String initials(String name) => name
    .split(RegExp(r'[\s&-]+'))
    .where((part) => part.isNotEmpty && RegExp(r'[A-Za-z]').hasMatch(part[0]))
    .take(2)
    .map((part) => part[0].toUpperCase())
    .join();

/// A softly pulsing placeholder for content that is still loading.
class SkeletonBlock extends StatefulWidget {
  const SkeletonBlock({super.key, this.width, this.height, this.radius = 12});
  final double? width, height;
  final double radius;
  @override
  State<SkeletonBlock> createState() => _SkeletonBlockState();
}

class _SkeletonBlockState extends State<SkeletonBlock>
    with SingleTickerProviderStateMixin {
  late final _pulse = AnimationController(
    vsync: this,
    duration: const Duration(milliseconds: 900),
  )..repeat(reverse: true);

  @override
  void dispose() {
    _pulse.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) => FadeTransition(
    opacity: Tween(begin: .45, end: 1.0).animate(_pulse),
    child: Container(
      width: widget.width,
      height: widget.height,
      decoration: BoxDecoration(
        color: cream,
        borderRadius: BorderRadius.circular(widget.radius),
      ),
    ),
  );
}

/// A small rounded label, e.g. a stall type or committee role.
class Pill extends StatelessWidget {
  const Pill(
    this.text, {
    super.key,
    this.background = cream,
    this.foreground = forest,
  });
  final String text;
  final Color background, foreground;
  @override
  Widget build(BuildContext context) => Container(
    padding: const EdgeInsets.symmetric(horizontal: 9, vertical: 4),
    decoration: BoxDecoration(
      color: background,
      borderRadius: BorderRadius.circular(99),
    ),
    child: Text(
      text,
      style: TextStyle(
        color: foreground,
        fontSize: 10,
        fontWeight: FontWeight.w700,
        letterSpacing: .3,
      ),
    ),
  );
}

/// Page intro used across guide pages: eyebrow, headline and optional lede.
class PageIntro extends StatelessWidget {
  const PageIntro({
    super.key,
    required this.eyebrow,
    required this.title,
    this.lede = '',
  });
  final String eyebrow, title, lede;
  @override
  Widget build(BuildContext context) => Column(
    crossAxisAlignment: CrossAxisAlignment.start,
    children: [
      Eyebrow(eyebrow),
      const SizedBox(height: 8),
      TitleText(title),
      if (lede.isNotEmpty) ...[
        const SizedBox(height: 10),
        Text(lede, style: const TextStyle(color: muted, height: 1.5)),
      ],
    ],
  );
}

/// Section heading with an optional trailing count, e.g. "Sponsors · 9".
class SectionLabel extends StatelessWidget {
  const SectionLabel(this.title, {super.key, this.trailing = ''});
  final String title, trailing;
  @override
  Widget build(BuildContext context) => Row(
    crossAxisAlignment: CrossAxisAlignment.end,
    children: [
      Expanded(
        child: Text(
          title,
          style: const TextStyle(fontFamily: 'Manrope', fontSize: 20),
        ),
      ),
      if (trailing.isNotEmpty)
        Text(trailing, style: const TextStyle(color: muted, fontSize: 12)),
    ],
  );
}
