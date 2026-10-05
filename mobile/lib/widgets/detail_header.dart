import 'package:flutter/material.dart';
import 'package:flutter/services.dart';

import '../main.dart';

/// A collapsing forest header for detail pages, with [child] (a portrait or
/// logo) centred on the leaf texture.
class DetailHeader extends StatelessWidget {
  const DetailHeader({
    super.key,
    required this.title,
    required this.childHeight,
    required this.child,
  });
  final String title;

  /// The height [child] needs; the header grows to fit it under the toolbar.
  final double childHeight;
  final Widget child;

  static const radius = Radius.circular(28);

  @override
  Widget build(BuildContext context) => SliverAppBar(
    pinned: true,
    stretch: true,
    // The clipped flexible space paints the header, so the bar itself stays
    // transparent and flat while content scrolls under it.
    backgroundColor: Colors.transparent,
    foregroundColor: Colors.white,
    surfaceTintColor: Colors.transparent,
    elevation: 0,
    scrolledUnderElevation: 0,
    systemOverlayStyle: SystemUiOverlayStyle.light,
    expandedHeight: childHeight + kToolbarHeight + 36,
    title: Text(title),
    // The flexible space is sized to the bar's current height, so the rounded
    // clip follows the bottom edge as the header collapses.
    flexibleSpace: ClipRRect(
      borderRadius: const BorderRadius.vertical(bottom: radius),
      // FlexibleSpaceBar fades its background out once collapsed; this keeps
      // a solid bar behind the toolbar.
      child: ColoredBox(
        color: forest,
        child: FlexibleSpaceBar(
          collapseMode: CollapseMode.parallax,
          background: DecoratedBox(
            decoration: const BoxDecoration(
              color: forest,
              image: DecorationImage(
                image: AssetImage('assets/images/pass-texture.webp'),
                fit: BoxFit.cover,
                colorFilter: ColorFilter.mode(
                  Color(0x59051C17),
                  BlendMode.srcOver,
                ),
              ),
            ),
            child: SafeArea(
              bottom: false,
              child: Padding(
                padding: const EdgeInsets.only(top: kToolbarHeight, bottom: 28),
                child: Center(child: _CollapseFade(child: child)),
              ),
            ),
          ),
        ),
      ),
    ),
  );
}

/// A soft frame that lifts a portrait or logo off the header.
class HeaderFrame extends StatelessWidget {
  const HeaderFrame({super.key, required this.child, this.width});
  final Widget child;
  final double? width;

  @override
  Widget build(BuildContext context) => Container(
    width: width,
    padding: const EdgeInsets.all(4),
    decoration: BoxDecoration(
      color: Colors.white.withValues(alpha: .14),
      borderRadius: BorderRadius.circular(26),
      boxShadow: const [
        BoxShadow(
          color: Color(0x40000000),
          blurRadius: 24,
          offset: Offset(0, 12),
        ),
      ],
    ),
    child: ClipRRect(borderRadius: BorderRadius.circular(22), child: child),
  );
}

/// A labelled fact with an icon tile, for detail-page panels.
class DetailFact extends StatelessWidget {
  const DetailFact(this.icon, this.label, this.value, {super.key});
  final IconData icon;
  final String label, value;

  @override
  Widget build(BuildContext context) => Padding(
    padding: const EdgeInsets.all(10),
    child: Row(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Container(
          width: 40,
          height: 40,
          decoration: BoxDecoration(
            color: cream,
            borderRadius: BorderRadius.circular(13),
          ),
          child: Icon(icon, color: forest, size: 20),
        ),
        const SizedBox(width: 12),
        Expanded(
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Text(
                label.toUpperCase(),
                style: const TextStyle(
                  color: muted,
                  fontSize: 10,
                  fontWeight: FontWeight.w700,
                  letterSpacing: 1.2,
                ),
              ),
              const SizedBox(height: 3),
              Text(
                value,
                style: const TextStyle(
                  fontSize: 15,
                  fontWeight: FontWeight.w600,
                  height: 1.35,
                ),
              ),
            ],
          ),
        ),
      ],
    ),
  );
}

/// A white rounded panel grouping facts or copy on a detail page.
class DetailPanel extends StatelessWidget {
  const DetailPanel({super.key, required this.children, this.padding = 6});
  final List<Widget> children;
  final double padding;

  @override
  Widget build(BuildContext context) => Container(
    width: double.infinity,
    padding: EdgeInsets.all(padding),
    decoration: BoxDecoration(
      color: Colors.white,
      borderRadius: BorderRadius.circular(20),
    ),
    child: Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: children,
    ),
  );
}

/// Fades and shrinks the header's picture as the bar collapses, so it is gone
/// before it reaches the toolbar title.
class _CollapseFade extends StatelessWidget {
  const _CollapseFade({required this.child});
  final Widget child;

  @override
  Widget build(BuildContext context) {
    final settings = context
        .dependOnInheritedWidgetOfExactType<FlexibleSpaceBarSettings>();
    if (settings == null) return child;
    final range = settings.maxExtent - settings.minExtent;
    final open = range <= 0
        ? 1.0
        : ((settings.currentExtent - settings.minExtent) / range).clamp(
            0.0,
            1.0,
          );
    // Fully visible above 60% open, gone by 25%.
    final t = ((open - .25) / .35).clamp(0.0, 1.0);
    return Opacity(
      opacity: Curves.easeOut.transform(t),
      child: Transform.scale(scale: .85 + .15 * t, child: child),
    );
  }
}
