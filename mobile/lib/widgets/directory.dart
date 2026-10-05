import 'package:cached_network_image/cached_network_image.dart';
import 'package:flutter/material.dart';
import 'package:flutter_cache_manager/flutter_cache_manager.dart';

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
          : CachedPicture(
              url,
              fit: BoxFit.contain,
              semanticLabel: '$name logo',
              // Logos are contained by height; decode no larger than shown.
              decodeHeight: h - padding * 2,
              placeholderRadius: 8,
              fallback: fallback,
            ),
    );
  }
}

/// The app's image cache: logos, portraits and speaker photos stay on disk for
/// 30 days after last use, capped at 500 files. Tests swap in an in-memory one.
BaseCacheManager imageCacheManager = CacheManager(
  Config(
    'bioConnectImages',
    stalePeriod: const Duration(days: 30),
    maxNrOfCacheObjects: 500,
  ),
);

/// A remote image kept in the on-disk cache, so it shows instantly on later
/// launches and offline at the venue. [fallback] covers missing or failed
/// images; a pulsing placeholder covers the first download.
class CachedPicture extends StatelessWidget {
  const CachedPicture(
    this.url, {
    super.key,
    required this.fallback,
    this.fit = BoxFit.cover,
    this.alignment = Alignment.center,
    this.width,
    this.decodeWidth,
    this.decodeHeight,
    this.semanticLabel,
    this.placeholderRadius = 0,
  });
  final String url;
  final Widget fallback;
  final BoxFit fit;
  final Alignment alignment;
  final double? width;

  /// Logical size to decode at. Set one side only, so the aspect ratio holds.
  final double? decodeWidth, decodeHeight;
  final String? semanticLabel;
  final double placeholderRadius;

  @override
  Widget build(BuildContext context) {
    final ratio = MediaQuery.devicePixelRatioOf(context);
    int? px(double? v) =>
        v == null || !v.isFinite || v <= 0 ? null : (v * ratio).ceil();
    final image = CachedNetworkImage(
      imageUrl: url,
      cacheManager: imageCacheManager,
      width: width,
      fit: fit,
      alignment: alignment,
      memCacheWidth: px(decodeWidth),
      memCacheHeight: px(decodeHeight),
      fadeInDuration: const Duration(milliseconds: 180),
      fadeOutDuration: Duration.zero,
      placeholder: (_, _) => SkeletonBlock(radius: placeholderRadius),
      errorWidget: (_, _, _) => fallback,
    );
    return semanticLabel == null
        ? image
        : Semantics(label: semanticLabel, image: true, child: image);
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
