import 'package:flutter/material.dart';
import 'package:provider/provider.dart';

import '../main.dart';
import '../models/event_content.dart';
import '../providers/content_provider.dart';
import '../widgets/directory.dart';
import '../widgets/interaction.dart';
import 'event_guide_screens.dart';

enum ExhibitorSort { stall, name }

/// Stall-first ordering: allocated stalls in floor-plan order, then the rest
/// alphabetically.
List<Exhibitor> sortExhibitors(List<Exhibitor> items, ExhibitorSort sort) {
  int byName(Exhibitor a, Exhibitor b) =>
      a.name.toLowerCase().compareTo(b.name.toLowerCase());
  return [...items]..sort((a, b) {
    if (sort == ExhibitorSort.name) return byName(a, b);
    if (a.hasStall != b.hasStall) return a.hasStall ? -1 : 1;
    if (!a.hasStall) return byName(a, b);
    final order = compareStallNumbers(a.stallNumber, b.stallNumber);
    return order != 0 ? order : byName(a, b);
  });
}

bool exhibitorMatches(Exhibitor e, String query) {
  final q = query.trim().toLowerCase();
  if (q.isEmpty) return true;
  final stall = e.stallNumber.toLowerCase();
  // "B12" finds stall "B-12" too.
  String compact(String v) => v.replaceAll(RegExp(r'[\s/-]'), '');
  return '${e.name} ${e.description} ${e.stallType}'.toLowerCase().contains(
        q,
      ) ||
      stall.contains(q) ||
      (stall.isNotEmpty && compact(stall).contains(compact(q)));
}

class ExhibitorsScreen extends StatefulWidget {
  const ExhibitorsScreen({super.key});
  @override
  State<ExhibitorsScreen> createState() => _ExhibitorsScreenState();
}

class _ExhibitorsScreenState extends State<ExhibitorsScreen> {
  String query = '';
  String type = '';
  ExhibitorSort sort = ExhibitorSort.stall;

  @override
  void initState() {
    super.initState();
    // Always refresh on open; a previously loaded list shows meanwhile.
    WidgetsBinding.instance.addPostFrameCallback(
      (_) => context.read<ContentProvider>().loadExhibitors(),
    );
  }

  @override
  Widget build(BuildContext context) {
    final state = context.watch<ContentProvider>();
    final all = state.exhibitors;
    final types = {
      for (final e in all)
        if (e.stallType.isNotEmpty) e.stallType,
    }.toList()..sort();
    final activeType = types.contains(type) ? type : '';
    final visible = sortExhibitors(
      all
          .where(
            (e) =>
                (activeType.isEmpty || e.stallType == activeType) &&
                exhibitorMatches(e, query),
          )
          .toList(),
      sort,
    );
    final allocated = all.where((e) => e.hasStall).length;
    // Stall features appear only once the organisers publish allocations.
    final stalls = allocated > 0;
    return Scaffold(
      appBar: AppBar(
        title: const Text('Exhibitors'),
        actions: [
          if (stalls)
            PopupMenuButton<ExhibitorSort>(
              tooltip: 'Sort exhibitors',
              icon: const Icon(Icons.sort),
              initialValue: sort,
              onSelected: (value) {
                AppFeedback.selection();
                setState(() => sort = value);
              },
              itemBuilder: (_) => [
                CheckedPopupMenuItem(
                  value: ExhibitorSort.stall,
                  checked: sort == ExhibitorSort.stall,
                  child: const Text('By stall number'),
                ),
                CheckedPopupMenuItem(
                  value: ExhibitorSort.name,
                  checked: sort == ExhibitorSort.name,
                  child: const Text('By name (A-Z)'),
                ),
              ],
            ),
          const SizedBox(width: 4),
        ],
      ),
      body: LiveRefresh(
        onRefresh: state.loadExhibitors,
        child: ListView(
          keyboardDismissBehavior: ScrollViewKeyboardDismissBehavior.onDrag,
          physics: const AlwaysScrollableScrollPhysics(),
          padding: const EdgeInsets.fromLTRB(20, 12, 20, 32),
          children: [
            PageIntro(
              eyebrow: 'THE EXPO',
              title: 'Meet your next\ncollaborator.',
              lede: stalls
                  ? 'Find exhibitors by name, expertise or stall number.'
                  : 'Find exhibitors by name or area of expertise.',
            ),
            const SizedBox(height: 20),
            if (!state.exhibitorsLoaded && state.exhibitorsError == null)
              ...List.generate(6, (_) => const _ExhibitorSkeleton())
            else if (!state.exhibitorsLoaded)
              StateMessage(
                state.exhibitorsError!,
                action: 'Try again',
                onTap: state.loadExhibitors,
              )
            else if (all.isEmpty)
              const GuideNotice(
                title: 'The line-up is being confirmed',
                message: 'Approved exhibitors will appear here. Pull down to check for updates.',
                icon: Icons.storefront_outlined,
              )
            else ...[
              if (state.exhibitorsError != null) ...[
                _StaleBanner(onRetry: state.loadExhibitors),
                const SizedBox(height: 12),
              ],
              GuideSearchField(
                label: stalls
                    ? 'Search name, expertise or stall'
                    : 'Search name or expertise',
                onChanged: (v) => setState(() => query = v),
              ),
              if (types.length > 1) ...[
                const SizedBox(height: 12),
                SingleChildScrollView(
                  scrollDirection: Axis.horizontal,
                  clipBehavior: Clip.none,
                  child: Row(
                    children: [
                      for (final t in ['', ...types])
                        Padding(
                          padding: const EdgeInsets.only(right: 8),
                          child: ChoiceChip(
                            label: Text(t.isEmpty ? 'All' : t),
                            selected: activeType == t,
                            onSelected: (_) {
                              if (activeType == t) return;
                              AppFeedback.selection();
                              FocusScope.of(context).unfocus();
                              setState(() => type = t);
                            },
                          ),
                        ),
                    ],
                  ),
                ),
              ],
              const SizedBox(height: 14),
              Text(
                visible.length != all.length
                    ? '${visible.length} of ${all.length} exhibitors'
                    : stalls
                    ? '${all.length} exhibitors · $allocated '
                          '${allocated == 1 ? 'stall' : 'stalls'} allocated'
                    : '${all.length} exhibitors',
                style: const TextStyle(color: muted, fontSize: 12),
              ),
              const SizedBox(height: 12),
              if (visible.isEmpty)
                GuideNotice(
                  title: 'No matching exhibitors',
                  message: stalls
                      ? 'Try another name, area of expertise or stall.'
                      : 'Try another name or area of expertise.',
                  icon: Icons.search_off,
                ),
              for (final e in visible) ExhibitorTile(e),
            ],
          ],
        ),
      ),
    );
  }
}

class _StaleBanner extends StatelessWidget {
  const _StaleBanner({required this.onRetry});
  final VoidCallback onRetry;
  @override
  Widget build(BuildContext context) => Container(
    padding: const EdgeInsets.fromLTRB(14, 6, 6, 6),
    decoration: BoxDecoration(
      color: gold.withValues(alpha: .18),
      borderRadius: BorderRadius.circular(14),
    ),
    child: Row(
      children: [
        const Icon(Icons.cloud_off_outlined, size: 18, color: ink),
        const SizedBox(width: 10),
        const Expanded(
          child: Text(
            'Showing the last loaded directory.',
            style: TextStyle(fontSize: 12),
          ),
        ),
        TextButton(onPressed: onRetry, child: const Text('Retry')),
      ],
    ),
  );
}

String _heroTag(Exhibitor e) => 'exhibitor:${e.name}:${e.stallNumber}';

class ExhibitorTile extends StatelessWidget {
  const ExhibitorTile(this.exhibitor, {super.key});
  final Exhibitor exhibitor;

  @override
  Widget build(BuildContext context) {
    final e = exhibitor;
    return Padding(
      padding: const EdgeInsets.only(bottom: 10),
      child: Material(
        color: Colors.white,
        borderRadius: BorderRadius.circular(18),
        child: InkWell(
          borderRadius: BorderRadius.circular(18),
          onTap: () => Navigator.push(
            context,
            MaterialPageRoute<void>(builder: (_) => ExhibitorDetailScreen(e)),
          ),
          child: Semantics(
            button: true,
            label: e.hasStall ? '${e.name}, stall ${e.stallNumber}' : e.name,
            excludeSemantics: true,
            child: Padding(
              padding: const EdgeInsets.all(12),
              child: Row(
                children: [
                  Hero(
                    tag: _heroTag(e),
                    child: LogoBox(
                      url: e.logoUrl,
                      name: e.name,
                      size: 60,
                      radius: 13,
                      padding: 6,
                      color: paper,
                    ),
                  ),
                  const SizedBox(width: 14),
                  Expanded(
                    child: Column(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: [
                        Text(
                          e.name,
                          maxLines: 2,
                          overflow: TextOverflow.ellipsis,
                          style: const TextStyle(
                            fontSize: 14,
                            fontWeight: FontWeight.w700,
                            height: 1.25,
                          ),
                        ),
                        if (e.stallType.isNotEmpty) ...[
                          const SizedBox(height: 3),
                          Text(
                            e.stallType,
                            style: const TextStyle(
                              color: forest,
                              fontSize: 11,
                              fontWeight: FontWeight.w500,
                            ),
                          ),
                        ],
                        if (e.description.isNotEmpty) ...[
                          const SizedBox(height: 5),
                          Text(
                            e.description,
                            maxLines: 2,
                            overflow: TextOverflow.ellipsis,
                            style: const TextStyle(
                              color: muted,
                              fontSize: 12,
                              height: 1.35,
                            ),
                          ),
                        ],
                      ],
                    ),
                  ),
                  if (e.hasStall) ...[const SizedBox(width: 10), StallBadge(e)],
                ],
              ),
            ),
          ),
        ),
      ),
    );
  }
}

/// An allocated stall number as a compact floor-plan tile.
class StallBadge extends StatelessWidget {
  const StallBadge(this.exhibitor, {super.key});
  final Exhibitor exhibitor;
  @override
  Widget build(BuildContext context) => Container(
    width: 56,
    height: 60,
    padding: const EdgeInsets.symmetric(horizontal: 5),
    decoration: BoxDecoration(
      color: forest,
      borderRadius: BorderRadius.circular(13),
    ),
    child: Column(
      mainAxisAlignment: MainAxisAlignment.center,
      children: [
        const Text(
          'STALL',
          style: TextStyle(
            color: lime,
            fontSize: 8.5,
            fontWeight: FontWeight.w700,
            letterSpacing: 1.2,
          ),
        ),
        const SizedBox(height: 3),
        FittedBox(
          fit: BoxFit.scaleDown,
          child: Text(
            exhibitor.stallNumber,
            maxLines: 1,
            style: const TextStyle(
              color: Colors.white,
              fontFamily: 'Manrope',
              fontSize: 18,
            ),
          ),
        ),
      ],
    ),
  );
}

class _ExhibitorSkeleton extends StatelessWidget {
  const _ExhibitorSkeleton();
  @override
  Widget build(BuildContext context) => Container(
    margin: const EdgeInsets.only(bottom: 10),
    padding: const EdgeInsets.all(12),
    decoration: BoxDecoration(
      color: Colors.white,
      borderRadius: BorderRadius.circular(18),
    ),
    child: const Row(
      children: [
        SkeletonBlock(width: 60, height: 60, radius: 13),
        SizedBox(width: 14),
        Expanded(
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              SkeletonBlock(width: 150, height: 14, radius: 6),
              SizedBox(height: 8),
              SkeletonBlock(width: 80, height: 10, radius: 5),
              SizedBox(height: 8),
              SkeletonBlock(height: 10, radius: 5),
            ],
          ),
        ),
      ],
    ),
  );
}

class ExhibitorDetailScreen extends StatelessWidget {
  const ExhibitorDetailScreen(this.exhibitor, {super.key});
  final Exhibitor exhibitor;

  @override
  Widget build(BuildContext context) {
    final e = exhibitor;
    final floorPlan =
        context.watch<ContentProvider>().content?.guide.venue.floorPlanUrl ??
        '';
    return Scaffold(
      appBar: AppBar(title: const Text('Exhibitor')),
      body: ListView(
        padding: const EdgeInsets.fromLTRB(20, 8, 20, 32),
        children: [
          Hero(
            tag: _heroTag(e),
            child: LogoBox(
              url: e.logoUrl,
              name: e.name,
              size: double.infinity,
              height: 160,
              radius: 22,
              padding: 28,
            ),
          ),
          const SizedBox(height: 22),
          const Eyebrow('EXHIBITOR'),
          const SizedBox(height: 8),
          SelectableText(
            e.name,
            style: const TextStyle(
              fontFamily: 'Manrope',
              fontSize: 28,
              height: 1.15,
              color: ink,
            ),
          ),
          if (e.stallType.isNotEmpty) ...[
            const SizedBox(height: 10),
            Align(alignment: Alignment.centerLeft, child: Pill(e.stallType)),
          ],
          // Stall and floor plan details appear only once published.
          if (e.hasStall) ...[
            const SizedBox(height: 20),
            Container(
              padding: const EdgeInsets.all(20),
              decoration: BoxDecoration(
                color: forest,
                borderRadius: BorderRadius.circular(20),
              ),
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Row(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Expanded(
                        child: Column(
                          crossAxisAlignment: CrossAxisAlignment.start,
                          children: [
                            const Text(
                              'FIND THEM AT STALL',
                              style: TextStyle(
                                color: lime,
                                fontSize: 10,
                                fontWeight: FontWeight.w700,
                                letterSpacing: 1.3,
                              ),
                            ),
                            const SizedBox(height: 6),
                            Text(
                              e.stallNumber,
                              style: const TextStyle(
                                color: Colors.white,
                                fontFamily: 'Manrope',
                                fontSize: 40,
                                height: 1.1,
                              ),
                            ),
                          ],
                        ),
                      ),
                      const Icon(
                        Icons.storefront_outlined,
                        color: lime,
                        size: 30,
                      ),
                    ],
                  ),
                  if (floorPlan.isNotEmpty) ...[
                    const SizedBox(height: 16),
                    FilledButton.icon(
                      onPressed: () => openLink(context, floorPlan),
                      style: FilledButton.styleFrom(
                        backgroundColor: lime,
                        foregroundColor: forest,
                      ),
                      icon: const Icon(Icons.map_outlined),
                      label: const Text('Open venue floor plan'),
                    ),
                  ],
                ],
              ),
            ),
          ] else if (floorPlan.isNotEmpty) ...[
            const SizedBox(height: 20),
            OutlinedButton.icon(
              onPressed: () => openLink(context, floorPlan),
              icon: const Icon(Icons.map_outlined),
              label: const Text('Open venue floor plan'),
            ),
          ],
          if (e.description.isNotEmpty) ...[
            const SizedBox(height: 26),
            const SectionLabel('About'),
            const SizedBox(height: 8),
            SelectableText(
              e.description,
              style: const TextStyle(color: ink, height: 1.55, fontSize: 15),
            ),
          ],
          const SizedBox(height: 26),
          OutlinedButton.icon(
            onPressed: () => showGuidePage(context, const VenueScreen()),
            icon: const Icon(Icons.place_outlined),
            label: const Text('Venue & directions'),
          ),
        ],
      ),
    );
  }
}
