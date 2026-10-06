import 'dart:math' as math;

import 'package:flutter/material.dart';
import 'package:provider/provider.dart';

import '../main.dart';
import '../widgets/destinations.dart';
import '../widgets/detail_header.dart';
import '../widgets/directory.dart';
import '../widgets/interaction.dart';
import '../widgets/motion.dart';
import '../models/event_content.dart';
import '../models/event_guide.dart';
import '../providers/agenda.dart';
import '../providers/content_provider.dart';
import '../widgets/saving.dart';
import 'agenda_screen.dart';
import 'delegate_registration_screen.dart';
import 'selfie_frame_screen.dart';

void showGuidePage(BuildContext context, Widget page) =>
    Navigator.of(context).push(MaterialPageRoute<void>(builder: (_) => page));

class AttendeeHomeScreen extends StatelessWidget {
  const AttendeeHomeScreen(this.content, {super.key});
  final EventContent content;
  @override
  Widget build(BuildContext context) {
    final sessions = MenuEntry(
      'sessions',
      content.text('home.button', 'View sessions'),
    );
    final shortcuts = visibleMenu(content, 'home_shortcuts');
    final allLinks = visibleMenu(content, 'home_links');
    // A published selfie frame link becomes the banner under the shortcuts.
    final frame = allLinks.where((e) => e.key == 'selfie_frame').firstOrNull;
    final links = [
      for (final entry in allLinks)
        if (entry.key != 'selfie_frame') entry,
    ];
    final noticeTitle = content.override('home.notice_title');
    final noticeMessage = content.override('home.notice_message');
    return LiveRefresh(
      onRefresh: context.read<ContentProvider>().load,
      child: ListView(
        keyboardDismissBehavior: ScrollViewKeyboardDismissBehavior.onDrag,
        physics: const AlwaysScrollableScrollPhysics(),
        padding: const EdgeInsets.fromLTRB(20, 12, 20, 32),
        children: [
          Reveal(
            child: _HomeHero(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text(
                    content.text('home.eyebrow', 'YOUR CONCLAVE COMPANION'),
                    style: const TextStyle(
                      color: lime,
                      fontSize: 10,
                      letterSpacing: 1.5,
                      fontWeight: FontWeight.bold,
                    ),
                  ),
                  const SizedBox(height: 8),
                  Text(
                    content.text(
                      'home.title',
                      'Make the most\nof Bio Connect.',
                    ),
                    style: const TextStyle(
                      fontFamily: 'Manrope',
                      color: Colors.white,
                      fontSize: 24,
                      height: 1.15,
                    ),
                  ),
                  const SizedBox(height: 12),
                  IconText(
                    Icons.calendar_today_outlined,
                    eventDateRange(content.event),
                  ),
                  if (content.event.venue.isNotEmpty) ...[
                    const SizedBox(height: 6),
                    IconText(Icons.place_outlined, content.event.venue),
                  ],
                  if (visibleTabs(content).any((t) => t.key == 'sessions') ||
                      content.guide.sessions.isNotEmpty) ...[
                    const SizedBox(height: 14),
                    FilledButton.icon(
                      onPressed: () =>
                          openDestination(context, content, sessions),
                      style: FilledButton.styleFrom(
                        backgroundColor: lime,
                        foregroundColor: forest,
                        minimumSize: const Size(0, 42),
                        padding: const EdgeInsets.symmetric(horizontal: 18),
                      ),
                      icon: const Icon(Icons.view_agenda_outlined, size: 20),
                      label: Text(sessions.title),
                    ),
                  ],
                ],
              ),
            ),
          ),
          const AgendaGlance(padding: EdgeInsets.only(top: 14)),
          if (shortcuts.isNotEmpty) ...[
            const SizedBox(height: 22),
            Reveal(
              order: 1,
              child: Text(
                content.text('home.glance', 'Your event, at a glance.'),
                style: const TextStyle(fontFamily: 'Manrope', fontSize: 19),
              ),
            ),
            const SizedBox(height: 12),
            LayoutBuilder(
              builder: (context, constraints) {
                // Three tiles a row on phones keeps every shortcut above the fold.
                final columns = constraints.maxWidth > 600 ? 6 : 3;
                const gap = 10.0;
                final width =
                    (constraints.maxWidth - gap * (columns - 1)) / columns;
                // Every tile has the same size: an icon, a one-line title and
                // up to two lines of subtitle at the reader's text scale.
                final text = MediaQuery.textScalerOf(context);
                final height =
                    24.0 +
                    36 +
                    10 +
                    text.scale(_QuickLink.titleSize) * _QuickLink.lineHeight +
                    2 +
                    text.scale(_QuickLink.subtitleSize) *
                        _QuickLink.lineHeight *
                        2 +
                    1;
                return Wrap(
                  spacing: gap,
                  runSpacing: gap,
                  children: [
                    for (final (i, entry) in shortcuts.indexed)
                      Reveal(
                        order: 2 + i,
                        child: _QuickLink(
                          entryIcon(entry),
                          entryTitle(entry),
                          entrySubtitle(entry, content),
                          () => openDestination(context, content, entry),
                          Size(width, height),
                        ),
                      ),
                  ],
                );
              },
            ),
          ],
          if (frame != null) ...[
            const SizedBox(height: 16),
            Reveal(
              order: 2 + shortcuts.length,
              child: SelfieFramePromo(content, frame),
            ),
          ],
          // Shown only while staff have an announcement up.
          if (noticeTitle != null || noticeMessage != null) ...[
            const SizedBox(height: 24),
            GuideNotice(
              title: noticeTitle ?? '',
              message: noticeMessage ?? '',
              icon: Icons.campaign_outlined,
            ),
          ],
          if (links.isNotEmpty) const SizedBox(height: 16),
          for (final entry in links)
            GuideCard(
              entryIcon(entry),
              entryTitle(entry),
              entrySubtitle(entry, content),
              () => openDestination(context, content, entry),
            ),
        ],
      ),
    );
  }
}

class _QuickLink extends StatelessWidget {
  const _QuickLink(this.icon, this.title, this.subtitle, this.onTap, this.size);
  final IconData icon;
  final String title, subtitle;
  final VoidCallback onTap;
  final Size size;

  /// Fixed line heights, so a tile's height is known before it is laid out.
  static const titleSize = 14.0, subtitleSize = 11.0, lineHeight = 1.3;
  @override
  Widget build(BuildContext context) => SizedBox.fromSize(
    size: size,
    child: Pressable(
      child: Material(
        color: cream,
        borderRadius: BorderRadius.circular(18),
        child: InkWell(
          onTap: onTap,
          borderRadius: BorderRadius.circular(18),
          child: Padding(
            padding: const EdgeInsets.all(12),
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Container(
                  width: 36,
                  height: 36,
                  decoration: BoxDecoration(
                    color: Colors.white,
                    borderRadius: BorderRadius.circular(12),
                  ),
                  child: Icon(icon, color: forest, size: 20),
                ),
                const SizedBox(height: 10),
                Text(
                  title,
                  maxLines: 1,
                  overflow: TextOverflow.ellipsis,
                  style: const TextStyle(
                    fontFamily: 'Manrope',
                    fontSize: titleSize,
                    height: lineHeight,
                  ),
                ),
                if (subtitle.isNotEmpty) ...[
                  const SizedBox(height: 2),
                  Text(
                    subtitle,
                    maxLines: 2,
                    overflow: TextOverflow.ellipsis,
                    style: const TextStyle(
                      color: muted,
                      fontSize: subtitleSize,
                      height: lineHeight,
                    ),
                  ),
                ],
              ],
            ),
          ),
        ),
      ),
    ),
  );
}

/// Invites attendees to make and share a selfie frame, worded for the days
/// before, during or after the event.
class SelfieFramePromo extends StatelessWidget {
  const SelfieFramePromo(this.content, this.entry, {super.key, this.now});
  final EventContent content;
  final MenuEntry entry;

  /// Tests fix the date; the app uses the phone's clock.
  final DateTime? now;

  @override
  Widget build(BuildContext context) {
    final (eyebrow, title) = switch (eventPhase(
      content.event,
      now ?? DateTime.now(),
    )) {
      EventPhase.before => (
        'SHARE THE NEWS',
        "Tell your network\nyou're coming.",
      ),
      EventPhase.during => ('YOU ARE HERE', "Tell your network\nyou're here."),
      EventPhase.after => ('THANKS FOR COMING', 'Share that you\nwere there.'),
    };
    void open() => openSelfieFrame(
      context,
      content,
      title: entryTitle(entry),
      caption: frameCaption(content.event, now ?? DateTime.now()),
    );
    return Pressable(
      child: Material(
        color: forest,
        borderRadius: BorderRadius.circular(24),
        clipBehavior: Clip.antiAlias,
        child: InkWell(
          onTap: open,
          child: Ink(
            decoration: const BoxDecoration(
              image: DecorationImage(
                image: AssetImage('assets/images/frame-forest.webp'),
                fit: BoxFit.cover,
                alignment: Alignment(0, -.55),
              ),
            ),
            child: DecoratedBox(
              decoration: const BoxDecoration(
                gradient: LinearGradient(
                  begin: Alignment.centerLeft,
                  end: Alignment.centerRight,
                  colors: [
                    Color(0xF2051C17),
                    Color(0xB30B3329),
                    Color(0x330B3329),
                  ],
                  stops: [0, .6, 1],
                ),
              ),
              child: Padding(
                padding: const EdgeInsets.fromLTRB(20, 18, 18, 18),
                child: Row(
                  children: [
                    Expanded(
                      child: Column(
                        crossAxisAlignment: CrossAxisAlignment.start,
                        children: [
                          Text(
                            content.text('frame.promo_eyebrow', eyebrow),
                            style: const TextStyle(
                              color: lime,
                              fontSize: 10,
                              letterSpacing: 1.5,
                              fontWeight: FontWeight.bold,
                            ),
                          ),
                          const SizedBox(height: 7),
                          Text(
                            content.text('frame.promo_title', title),
                            style: const TextStyle(
                              fontFamily: 'Manrope',
                              color: Colors.white,
                              fontSize: 20,
                              height: 1.2,
                            ),
                          ),
                          const SizedBox(height: 14),
                          FilledButton.icon(
                            onPressed: open,
                            style: FilledButton.styleFrom(
                              backgroundColor: lime,
                              foregroundColor: forest,
                              minimumSize: const Size(0, 42),
                              padding: const EdgeInsets.symmetric(
                                horizontal: 16,
                              ),
                            ),
                            icon: const Icon(
                              Icons.add_a_photo_outlined,
                              size: 19,
                            ),
                            label: Text(
                              content.text(
                                'frame.promo_button',
                                'Create your frame',
                              ),
                            ),
                          ),
                        ],
                      ),
                    ),
                    const SizedBox(width: 14),
                    const _ArchPreview(),
                  ],
                ),
              ),
            ),
          ),
        ),
      ),
    );
  }
}

/// A small gold arch, echoing the Arch frame design.
class _ArchPreview extends StatelessWidget {
  const _ArchPreview();

  @override
  Widget build(BuildContext context) => Container(
    width: 74,
    height: 96,
    padding: const EdgeInsets.all(4),
    decoration: const BoxDecoration(
      border: Border.fromBorderSide(BorderSide(color: gold, width: 1.4)),
      borderRadius: BorderRadius.vertical(
        top: Radius.circular(37),
        bottom: Radius.circular(10),
      ),
    ),
    child: const DecoratedBox(
      decoration: BoxDecoration(
        color: Color(0x33B9DC72),
        borderRadius: BorderRadius.vertical(
          top: Radius.circular(33),
          bottom: Radius.circular(7),
        ),
      ),
      child: Center(
        child: Icon(
          Icons.face_retouching_natural_outlined,
          color: lime,
          size: 30,
        ),
      ),
    ),
  );
}

/// The home banner: the event's artwork behind a forest wash that keeps the
/// copy readable.
class _HomeHero extends StatelessWidget {
  const _HomeHero({required this.child});
  final Widget child;

  @override
  Widget build(BuildContext context) => ClipRRect(
    borderRadius: BorderRadius.circular(24),
    child: DecoratedBox(
      decoration: const BoxDecoration(
        color: forest,
        image: DecorationImage(
          image: AssetImage('assets/images/home-hero.webp'),
          fit: BoxFit.cover,
          alignment: Alignment(.7, 0),
        ),
      ),
      child: DecoratedBox(
        decoration: const BoxDecoration(
          gradient: LinearGradient(
            begin: Alignment.centerLeft,
            end: Alignment.centerRight,
            colors: [Color(0xF2051C17), Color(0xB30B3329), Color(0x1A0B3329)],
            stops: [0, .55, 1],
          ),
        ),
        child: Padding(padding: const EdgeInsets.all(20), child: child),
      ),
    ),
  );
}

class GuideNotice extends StatelessWidget {
  const GuideNotice({
    super.key,
    required this.title,
    required this.message,
    this.icon = Icons.schedule,
  });
  final String title, message;
  final IconData icon;
  @override
  Widget build(BuildContext context) => Container(
    padding: const EdgeInsets.all(20),
    decoration: BoxDecoration(
      color: cream,
      borderRadius: BorderRadius.circular(18),
    ),
    child: Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Icon(icon, color: forest),
        if (title.isNotEmpty) ...[
          const SizedBox(height: 12),
          Text(
            title,
            style: const TextStyle(fontFamily: 'Manrope', fontSize: 18),
          ),
        ],
        if (message.isNotEmpty) ...[
          const SizedBox(height: 8),
          Text(message, style: const TextStyle(color: muted, height: 1.5)),
        ],
      ],
    ),
  );
}

class SessionsScreen extends StatefulWidget {
  const SessionsScreen({super.key});
  @override
  State<SessionsScreen> createState() => _SessionsScreenState();
}

class _SessionsScreenState extends State<SessionsScreen> {
  static const allDays = 'All days';
  String query = '';

  /// Null until the attendee picks a day: then today during the event,
  /// otherwise every day.
  String? selectedDay;

  @override
  Widget build(BuildContext context) {
    final state = context.watch<ContentProvider>();
    final content = state.content!;
    final agenda = context.watch<Agenda>();
    final now = agenda.now();
    final sessions = sortSessions(content.guide.sessions);
    final days = sessions
        .where((s) => s.startsAt != null)
        .map((s) => sessionDayLabel(s.startsAt!))
        .toSet()
        .toList();
    final today = sessionDayLabel(now);
    final wanted = selectedDay ?? (days.contains(today) ? today : allDays);
    final activeDay = days.contains(wanted) ? wanted : allDays;
    final terms = query.toLowerCase().split(RegExp(r'\s+'))
      ..removeWhere((t) => t.isEmpty);
    final filtered = sessions
        .where(
          (s) =>
              (activeDay == allDays ||
                  (s.startsAt != null &&
                      sessionDayLabel(s.startsAt!) == activeDay)) &&
              terms.every(s.searchText.contains),
        )
        .toList();
    // A search shows what matched; breaks only frame the full timetable.
    final shown = terms.isEmpty
        ? filtered
        : filtered.where((s) => s.plannable).toList();
    final live = sessions.where((s) => s.plannable && s.liveAt(now)).toList();
    return LiveRefresh(
      onRefresh: state.load,
      child: ListView(
        keyboardDismissBehavior: ScrollViewKeyboardDismissBehavior.onDrag,
        physics: const AlwaysScrollableScrollPhysics(),
        padding: const EdgeInsets.fromLTRB(20, 24, 20, 30),
        children: [
          Eyebrow(content.text('sessions.eyebrow', 'THE PROGRAMME')),
          const SizedBox(height: 8),
          TitleText(
            content.text('sessions.title', 'Find your next\nconversation.'),
          ),
          const SizedBox(height: 10),
          Text(
            content.text(
              'sessions.intro',
              'Sessions, speakers and places to be.',
            ),
            style: const TextStyle(color: muted),
          ),
          const SizedBox(height: 24),
          if (sessions.isEmpty) ...[
            const GuideNotice(
              title: 'Session timetable to be announced',
              message: 'The confirmed programme, session times and halls will appear here. Pull down to check for updates.',
            ),
            if (content.event.brochureUrl.isNotEmpty) ...[
              const SizedBox(height: 20),
              OutlinedButton.icon(
                onPressed: () => openLink(context, content.event.brochureUrl),
                icon: const Icon(Icons.article_outlined),
                label: const Text('View event brochure'),
              ),
            ],
          ] else ...[
            if (live.isNotEmpty) ...[
              _HappeningNow(live),
              const SizedBox(height: 18),
            ],
            GuideSearchField(
              label: 'Search sessions, speakers or topics',
              onChanged: (v) => setState(() => query = v),
            ),
            const SizedBox(height: 12),
            Wrap(
              spacing: 8,
              runSpacing: 8,
              children: [
                for (final day in [allDays, ...days])
                  ChoiceChip(
                    label: Text(
                      day == today && day != allDays ? '$day · Today' : day,
                    ),
                    selected: activeDay == day,
                    onSelected: (_) {
                      if (activeDay != day) {
                        AppFeedback.selection();
                        FocusScope.of(context).unfocus();
                        setState(() => selectedDay = day);
                      }
                    },
                  ),
              ],
            ),
            const SizedBox(height: 8),
            if (shown.isEmpty) ...[
              const SizedBox(height: 8),
              const GuideNotice(
                title: 'No matching sessions',
                message: 'Try a different search or choose another day.',
                icon: Icons.search_off,
              ),
            ],
            for (final (i, session) in shown.indexed) ...[
              if (session.startsAt != null &&
                  (i == 0 ||
                      shown[i - 1].startsAt == null ||
                      sessionDayLabel(shown[i - 1].startsAt!) !=
                          sessionDayLabel(session.startsAt!)))
                _DayHeading(session.startsAt!, days),
              if (session.startsAt == null &&
                  (i == 0 || shown[i - 1].startsAt != null))
                const _DayHeadingText('Time to be confirmed'),
              if (session.isBreak)
                _BreakRow(session)
              else
                _SessionCard(
                  session,
                  live: session.liveAt(now),
                  saved: agenda.hasSession(session.id),
                ),
            ],
          ],
        ],
      ),
    );
  }
}

/// "DAY 1 · THU 8 OCT", numbered across the whole programme.
class _DayHeading extends StatelessWidget {
  const _DayHeading(this.day, this.days);
  final DateTime day;
  final List<String> days;
  @override
  Widget build(BuildContext context) {
    final label = sessionDayLabel(day);
    final n = days.indexOf(label) + 1;
    return _DayHeadingText(days.length > 1 ? 'Day $n · $label' : label);
  }
}

class _DayHeadingText extends StatelessWidget {
  const _DayHeadingText(this.text);
  final String text;
  @override
  Widget build(BuildContext context) => Padding(
    padding: const EdgeInsets.fromLTRB(2, 22, 2, 10),
    child: Semantics(
      header: true,
      child: Text(
        text.toUpperCase(),
        style: const TextStyle(
          color: forest,
          fontSize: 11,
          fontWeight: FontWeight.w800,
          letterSpacing: 1.4,
        ),
      ),
    ),
  );
}

/// What kind of entry a session is, when staff gave it no label.
String sessionKindLabel(GuideSession s) => switch (s.kind) {
  'talk' => 'Talk',
  'panel' => 'Panel discussion',
  'ceremony' => 'Ceremony',
  'social' => 'Evening programme',
  'break' => 'Break',
  _ => 'Session',
};

/// One line about who is on stage, for the timetable.
String sessionPeopleSummary(GuideSession s) {
  if (s.segments.isNotEmpty) {
    final n = s.segments.where((g) => g.startsAt != null).length;
    return 'Running order · $n ${n == 1 ? 'item' : 'items'}';
  }
  final moderator = s.people.where((p) => p.role == 'moderator').firstOrNull;
  final panel = s.people.where((p) => p.role == 'panelist').length;
  final others = s.people.where(
    (p) => p.role != 'moderator' && p.role != 'panelist',
  );
  if (moderator != null || panel > 0) {
    return [
      if (moderator != null) 'Moderated by ${moderator.name}',
      if (panel > 0) '$panel ${panel == 1 ? 'panellist' : 'panellists'}',
    ].join(' · ');
  }
  if (others.length == 1) {
    final p = others.first;
    // A keynote titled with its speaker's name needs only the designation.
    return p.name == s.title ? p.designation : p.name;
  }
  if (others.isNotEmpty) return others.map((p) => p.name).join(', ');
  return s.structured ? '' : s.speakers.split('\n').first;
}

class _SessionCard extends StatelessWidget {
  const _SessionCard(this.session, {required this.live, required this.saved});
  final GuideSession session;
  final bool live, saved;
  @override
  Widget build(BuildContext context) {
    final s = session;
    final summary = sessionPeopleSummary(s);
    final meta = [
      if (s.location.isNotEmpty) s.location,
      if (s.track.isNotEmpty) s.track,
    ];
    return Pressable(
      child: Card(
        color: Colors.white,
        margin: const EdgeInsets.only(bottom: 10),
        shape: RoundedRectangleBorder(
          borderRadius: BorderRadius.circular(18),
          side: live
              ? const BorderSide(color: forest, width: 1.5)
              : BorderSide.none,
        ),
        clipBehavior: Clip.antiAlias,
        child: InkWell(
          onTap: () => showGuidePage(context, SessionDetailScreen(s.id)),
          child: Padding(
            padding: const EdgeInsets.fromLTRB(16, 16, 4, 16),
            child: Row(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                SizedBox(
                  width: 50,
                  child: s.startsAt == null
                      ? const Text('TBC', style: TextStyle(color: muted))
                      : Column(
                          crossAxisAlignment: CrossAxisAlignment.start,
                          children: [
                            Text(
                              sessionTime(s.startsAt!),
                              style: const TextStyle(
                                fontFamily: 'Manrope',
                                fontWeight: FontWeight.w700,
                                fontSize: 15,
                                fontFeatures: [FontFeature.tabularFigures()],
                              ),
                            ),
                            if (s.endsAt != null)
                              Text(
                                sessionTime(s.endsAt!),
                                style: const TextStyle(
                                  color: muted,
                                  fontSize: 12,
                                  fontFeatures: [FontFeature.tabularFigures()],
                                ),
                              ),
                          ],
                        ),
                ),
                const SizedBox(width: 10),
                Expanded(
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Row(
                        children: [
                          if (live) ...[
                            const _LiveBadge(),
                            const SizedBox(width: 8),
                          ],
                          Flexible(
                            child: Text(
                              (s.label.isEmpty ? sessionKindLabel(s) : s.label)
                                  .toUpperCase(),
                              style: const TextStyle(
                                color: forest,
                                fontSize: 10,
                                fontWeight: FontWeight.w800,
                                letterSpacing: 1.2,
                              ),
                            ),
                          ),
                        ],
                      ),
                      const SizedBox(height: 6),
                      Text(
                        s.title,
                        style: const TextStyle(
                          fontFamily: 'Manrope',
                          fontSize: 16,
                          height: 1.25,
                          fontWeight: FontWeight.w600,
                        ),
                      ),
                      if (summary.isNotEmpty) ...[
                        const SizedBox(height: 6),
                        Text(
                          summary,
                          maxLines: 2,
                          overflow: TextOverflow.ellipsis,
                          style: const TextStyle(
                            color: muted,
                            fontSize: 12,
                            height: 1.4,
                          ),
                        ),
                      ],
                      if (meta.isNotEmpty) ...[
                        const SizedBox(height: 8),
                        Text(
                          meta.join(' · '),
                          style: const TextStyle(
                            color: forest,
                            fontSize: 11,
                            fontWeight: FontWeight.w600,
                          ),
                        ),
                      ],
                    ],
                  ),
                ),
                SaveIcon(
                  saved: saved,
                  label: s.title,
                  onPressed: () => toggleSavedSession(context, s),
                ),
              ],
            ),
          ),
        ),
      ),
    );
  }
}

class _LiveBadge extends StatelessWidget {
  const _LiveBadge();
  @override
  Widget build(BuildContext context) => Container(
    padding: const EdgeInsets.symmetric(horizontal: 7, vertical: 2),
    decoration: BoxDecoration(
      color: lime,
      borderRadius: BorderRadius.circular(20),
    ),
    child: const Text(
      'NOW',
      style: TextStyle(
        color: deepForest,
        fontSize: 9,
        fontWeight: FontWeight.w800,
        letterSpacing: 1,
      ),
    ),
  );
}

/// Tea, lunch and registration: part of the day's shape, not something to plan.
class _BreakRow extends StatelessWidget {
  const _BreakRow(this.session);
  final GuideSession session;
  @override
  Widget build(BuildContext context) {
    final s = session;
    final title = s.title.toLowerCase();
    final icon = title.contains('lunch') || title.contains('dinner')
        ? Icons.restaurant_outlined
        : title.contains('registration')
        ? Icons.badge_outlined
        : Icons.local_cafe_outlined;
    return Semantics(
      container: true,
      label: [s.title, if (s.startsAt != null) sessionSpan(s)].join(', '),
      excludeSemantics: true,
      child: Container(
        margin: const EdgeInsets.only(bottom: 10),
        padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 11),
        decoration: BoxDecoration(
          color: cream,
          borderRadius: BorderRadius.circular(14),
        ),
        child: Row(
          children: [
            SizedBox(
              width: 50,
              child: Text(
                s.startsAt == null ? '' : sessionTime(s.startsAt!),
                style: const TextStyle(
                  color: muted,
                  fontSize: 13,
                  fontFeatures: [FontFeature.tabularFigures()],
                ),
              ),
            ),
            const SizedBox(width: 10),
            Icon(icon, size: 18, color: muted),
            const SizedBox(width: 10),
            Expanded(
              child: Text(
                s.title,
                style: const TextStyle(fontWeight: FontWeight.w600),
              ),
            ),
            if (s.endsAt != null)
              Text(
                'until ${sessionTime(s.endsAt!)}',
                style: const TextStyle(color: muted, fontSize: 12),
              ),
          ],
        ),
      ),
    );
  }
}

/// The sessions under way, above the timetable while the event runs.
class _HappeningNow extends StatelessWidget {
  const _HappeningNow(this.sessions);
  final List<GuideSession> sessions;
  @override
  Widget build(BuildContext context) => Container(
    padding: const EdgeInsets.all(16),
    decoration: BoxDecoration(
      color: forest,
      borderRadius: BorderRadius.circular(18),
    ),
    child: Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        const Row(
          children: [
            _LiveBadge(),
            SizedBox(width: 8),
            Text(
              'HAPPENING NOW',
              style: TextStyle(
                color: lime,
                fontSize: 10,
                fontWeight: FontWeight.w800,
                letterSpacing: 1.2,
              ),
            ),
          ],
        ),
        for (final s in sessions)
          InkWell(
            onTap: () => showGuidePage(context, SessionDetailScreen(s.id)),
            child: Padding(
              padding: const EdgeInsets.only(top: 10),
              child: Row(
                children: [
                  Expanded(
                    child: Column(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: [
                        Text(
                          s.title,
                          style: const TextStyle(
                            color: Colors.white,
                            fontFamily: 'Manrope',
                            fontSize: 16,
                            fontWeight: FontWeight.w600,
                          ),
                        ),
                        Text(
                          [
                            'Until ${sessionTime(s.endsAt!)}',
                            if (s.location.isNotEmpty) s.location,
                          ].join(' · '),
                          style: const TextStyle(
                            color: Color(0xCCFFFFFF),
                            fontSize: 12,
                          ),
                        ),
                      ],
                    ),
                  ),
                  const Icon(Icons.chevron_right, color: Colors.white),
                ],
              ),
            ),
          ),
      ],
    ),
  );
}

class SessionDetailScreen extends StatelessWidget {
  const SessionDetailScreen(this.id, {super.key});
  final String id;
  @override
  Widget build(BuildContext context) {
    final state = context.watch<ContentProvider>();
    final content = state.content!;
    final agenda = context.watch<Agenda>();
    final items = content.guide.sessions.where((s) => s.id == id);
    final plannable = items.isNotEmpty && items.first.plannable;
    return Scaffold(
      appBar: AppBar(
        title: const Text('Session details'),
        actions: [
          if (plannable)
            SaveIcon(
              saved: agenda.hasSession(id),
              label: 'this session',
              onPressed: () => toggleSavedSession(context, items.first),
            ),
          const SizedBox(width: 8),
        ],
      ),
      body: LiveRefresh(
        onRefresh: state.load,
        child: items.isEmpty
            ? ListView(
                physics: const AlwaysScrollableScrollPhysics(),
                padding: const EdgeInsets.all(20),
                children: const [
                  GuideNotice(
                    title: 'This session is no longer published',
                    message: 'It may have been moved or withdrawn. Check the programme for the latest timetable.',
                    icon: Icons.event_busy_outlined,
                  ),
                ],
              )
            : Builder(
                builder: (context) {
                  final s = items.first;
                  final saved = agenda.hasSession(s.id);
                  final clashes = plannable
                      ? agenda.clashesFor(s, content)
                      : const <GuideSession>[];
                  final linked = [
                    for (final id in s.speakerIds) ?content.speaker(id),
                  ];
                  return ListView(
                    keyboardDismissBehavior:
                        ScrollViewKeyboardDismissBehavior.onDrag,
                    physics: const AlwaysScrollableScrollPhysics(),
                    padding: const EdgeInsets.fromLTRB(20, 20, 20, 36),
                    children: [
                      Row(
                        children: [
                          if (s.liveAt(agenda.now())) ...[
                            const _LiveBadge(),
                            const SizedBox(width: 8),
                          ],
                          Flexible(
                            child: Eyebrow(
                              (s.label.isEmpty ? sessionKindLabel(s) : s.label)
                                  .toUpperCase(),
                            ),
                          ),
                        ],
                      ),
                      const SizedBox(height: 8),
                      TitleText(s.title),
                      if (s.track.isNotEmpty) ...[
                        const SizedBox(height: 12),
                        Align(
                          alignment: Alignment.centerLeft,
                          child: Chip(
                            label: Text(s.track),
                            backgroundColor: cream,
                            visualDensity: VisualDensity.compact,
                          ),
                        ),
                      ],
                      const SizedBox(height: 18),
                      if (plannable)
                        SaveButton(
                          saved: saved,
                          onPressed: () => toggleSavedSession(context, s),
                        ),
                      if (plannable &&
                          s.startsAt != null &&
                          s.endsAt != null) ...[
                        const SizedBox(height: 10),
                        OutlinedButton.icon(
                          onPressed: () => addSessionsToCalendar(context, [s]),
                          icon: const Icon(Icons.event_available_outlined),
                          label: const Text('Add to calendar'),
                        ),
                      ],
                      if (clashes.isNotEmpty) ...[
                        const SizedBox(height: 14),
                        GuideNotice(
                          icon: Icons.warning_amber_rounded,
                          title: saved
                              ? 'Overlaps your agenda'
                              : 'Would overlap your agenda',
                          message: [
                            for (final c in clashes)
                              '${c.title} (${sessionSpan(c)})',
                          ].join('\n'),
                        ),
                      ],
                      const SizedBox(height: 6),
                      if (s.startsAt case final start?)
                        _DetailBlock(
                          'When',
                          '${sessionDayLabel(start)}\n${sessionTime(start)}${s.endsAt == null ? '' : ' - ${sessionTime(s.endsAt!)}'}',
                          icon: Icons.schedule_rounded,
                        ),
                      if (s.location.isNotEmpty)
                        _DetailBlock(
                          'Where',
                          s.location,
                          icon: Icons.place_outlined,
                        ),
                      if (s.description.isNotEmpty)
                        _DetailBlock(
                          'About the session',
                          s.description,
                          icon: Icons.notes_rounded,
                        ),
                      if (s.people.isNotEmpty) _PeoplePanel(s),
                      if (s.segments.isNotEmpty) _RunningOrder(s.segments),
                      // Content saved before people were structured.
                      if (!s.structured && s.speakers.isNotEmpty)
                        _DetailBlock(
                          'Speakers',
                          s.speakers,
                          icon: Icons.people_outline_rounded,
                        ),
                      if (!s.structured && linked.isNotEmpty) ...[
                        const SizedBox(height: 14),
                        for (final speaker in linked)
                          GuideCard(
                            Icons.person_outline,
                            speaker.name,
                            [
                              speaker.role,
                              speaker.organization,
                            ].where((v) => v.isNotEmpty).join(' · '),
                            () => showGuidePage(
                              context,
                              SpeakerDetailScreen(speaker),
                            ),
                          ),
                      ],
                    ],
                  );
                },
              ),
      ),
    );
  }
}

/// The people on a session, grouped by their part in it.
class _PeoplePanel extends StatelessWidget {
  const _PeoplePanel(this.session);
  final GuideSession session;
  @override
  Widget build(BuildContext context) {
    final people = session.people;
    final speaking = people
        .where((p) => !const {'moderator', 'panelist', 'host'}.contains(p.role))
        .toList();
    final groups = [
      (session.kind == 'panel' ? 'In conversation' : 'Speaker', speaking),
      ('Moderator', people.where((p) => p.role == 'moderator').toList()),
      ('Panellists', people.where((p) => p.role == 'panelist').toList()),
      ('Host', people.where((p) => p.role == 'host').toList()),
    ].where((g) => g.$2.isNotEmpty);
    return Padding(
      padding: const EdgeInsets.only(top: 14),
      child: DetailPanel(
        padding: 8,
        children: [
          for (final (i, (title, list)) in groups.indexed) ...[
            if (i > 0) const Divider(height: 18, color: cream),
            Padding(
              padding: const EdgeInsets.fromLTRB(10, 10, 10, 2),
              child: Text(
                (list.length > 1 && title == 'Speaker' ? 'Speakers' : title)
                    .toUpperCase(),
                style: const TextStyle(
                  color: muted,
                  fontSize: 10,
                  fontWeight: FontWeight.w700,
                  letterSpacing: 1.2,
                ),
              ),
            ),
            for (final p in list) _PersonTile(p),
          ],
        ],
      ),
    );
  }
}

/// A person with a portrait when they are in the speaker directory, whose
/// profile it then opens; initials otherwise.
class _PersonTile extends StatelessWidget {
  const _PersonTile(this.person, {this.compact = false});
  final SessionPerson person;
  final bool compact;
  @override
  Widget build(BuildContext context) {
    final speaker = person.speakerId.isEmpty
        ? null
        : context.read<ContentProvider>().content?.speaker(person.speakerId);
    final size = compact ? 36.0 : 48.0;
    final avatar = ClipOval(
      child: SizedBox.square(
        dimension: size,
        child: speaker != null && speaker.image.isNotEmpty
            ? ColoredBox(
                color: cream,
                child: SpeakerImage(speaker, width: size),
              )
            : ColoredBox(
                color: cream,
                child: Center(
                  child: Text(
                    personInitials(person.name),
                    style: TextStyle(
                      color: forest,
                      fontWeight: FontWeight.w700,
                      fontSize: compact ? 12 : 15,
                    ),
                  ),
                ),
              ),
      ),
    );
    final text = Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Text(
          person.name,
          style: TextStyle(
            fontWeight: FontWeight.w700,
            fontSize: compact ? 13 : 14,
          ),
        ),
        if (person.designation.isNotEmpty) ...[
          const SizedBox(height: 2),
          Text(
            person.designation,
            style: TextStyle(
              color: muted,
              fontSize: compact ? 11.5 : 12.5,
              height: 1.4,
            ),
          ),
        ],
      ],
    );
    final row = Padding(
      padding: EdgeInsets.symmetric(horizontal: 10, vertical: compact ? 6 : 8),
      child: Row(
        children: [
          avatar,
          const SizedBox(width: 12),
          Expanded(child: text),
          if (speaker != null)
            const Icon(Icons.chevron_right, color: forest, size: 20),
        ],
      ),
    );
    if (speaker == null) return row;
    return InkWell(
      borderRadius: BorderRadius.circular(14),
      onTap: () => showGuidePage(context, SpeakerDetailScreen(speaker)),
      child: Semantics(
        button: true,
        hint: 'Opens their speaker profile',
        child: row,
      ),
    );
  }
}

/// Up to two initials, ignoring honorifics such as "Dr." or "Shri.".
String personInitials(String name) {
  final words = name
      .replaceAll(
        RegExp(
          r'\b(dr|prof|mr|mrs|ms|shri|smt|padma bhushan|padma shri|ias)\b\.?',
          caseSensitive: false,
        ),
        ' ',
      )
      .split(RegExp(r'[\s.]+'))
      .where((w) => w.isNotEmpty && RegExp(r'[A-Za-z]').hasMatch(w[0]))
      .toList();
  if (words.isEmpty) return '';
  final last = words.length > 1 ? words.last[0] : '';
  return (words.first[0] + last).toUpperCase();
}

/// A ceremony's running order as a timeline.
class _RunningOrder extends StatelessWidget {
  const _RunningOrder(this.segments);
  final List<SessionSegment> segments;
  @override
  Widget build(BuildContext context) => Padding(
    padding: const EdgeInsets.only(top: 14),
    child: DetailPanel(
      padding: 18,
      children: [
        const Row(
          children: [
            Icon(Icons.format_list_numbered_rounded, color: forest, size: 20),
            SizedBox(width: 10),
            Text(
              'Running order',
              style: TextStyle(fontFamily: 'Manrope', fontSize: 17),
            ),
          ],
        ),
        const SizedBox(height: 14),
        for (final (i, g) in segments.indexed)
          IntrinsicHeight(
            child: Row(
              crossAxisAlignment: CrossAxisAlignment.stretch,
              children: [
                SizedBox(
                  width: 46,
                  child: Text(
                    g.startsAt == null ? '' : sessionTime(g.startsAt!),
                    style: const TextStyle(
                      fontWeight: FontWeight.w700,
                      fontSize: 13,
                      fontFeatures: [FontFeature.tabularFigures()],
                    ),
                  ),
                ),
                SizedBox(
                  width: 18,
                  child: Column(
                    children: [
                      Container(
                        margin: const EdgeInsets.only(top: 4),
                        width: 9,
                        height: 9,
                        decoration: BoxDecoration(
                          color: g.startsAt == null ? cream : lime,
                          border: Border.all(color: forest, width: 1.5),
                          shape: BoxShape.circle,
                        ),
                      ),
                      if (i < segments.length - 1)
                        Expanded(child: Container(width: 1.5, color: cream)),
                    ],
                  ),
                ),
                const SizedBox(width: 8),
                Expanded(
                  child: Padding(
                    padding: const EdgeInsets.only(bottom: 18),
                    child: Column(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: [
                        Text(
                          g.title,
                          style: TextStyle(
                            fontWeight: FontWeight.w600,
                            color: g.startsAt == null ? muted : ink,
                          ),
                        ),
                        if (g.description.isNotEmpty) ...[
                          const SizedBox(height: 3),
                          Text(
                            g.description,
                            style: const TextStyle(
                              color: muted,
                              fontSize: 12.5,
                              height: 1.4,
                            ),
                          ),
                        ],
                        for (final p in g.people)
                          Padding(
                            padding: const EdgeInsets.only(top: 4),
                            child: Transform.translate(
                              offset: const Offset(-10, 0),
                              child: _PersonTile(p, compact: true),
                            ),
                          ),
                      ],
                    ),
                  ),
                ),
              ],
            ),
          ),
      ],
    ),
  );
}

class VenueScreen extends StatelessWidget {
  const VenueScreen({super.key});
  @override
  Widget build(BuildContext context) {
    final state = context.watch<ContentProvider>();
    final content = state.content!;
    final event = content.event, venue = content.guide.venue;
    final address = venue.address.isNotEmpty ? venue.address : event.city;
    // Wide enough to show the venue, but never most of a short screen.
    final screen = MediaQuery.sizeOf(context);
    final photoHeight = math.min(
      (screen.width * .62).clamp(200.0, 340.0),
      screen.height * .34,
    );
    return Scaffold(
      body: LiveRefresh(
        onRefresh: state.load,
        child: CustomScrollView(
          keyboardDismissBehavior: ScrollViewKeyboardDismissBehavior.onDrag,
          physics: const AlwaysScrollableScrollPhysics(),
          slivers: [
            DetailHeader(
              title: 'Venue & directions',
              childHeight: photoHeight,
              photo: const AssetImage('assets/images/venue.webp'),
              // Keep the hotel building, not just the pool, in view.
              photoAlignment: const Alignment(-.75, -.3),
              child: Column(
                mainAxisSize: MainAxisSize.min,
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text(
                    content.text('venue.eyebrow', 'GETTING HERE'),
                    style: const TextStyle(
                      color: lime,
                      fontSize: 11,
                      fontWeight: FontWeight.w700,
                      letterSpacing: 1.5,
                    ),
                  ),
                  const SizedBox(height: 6),
                  Text(
                    event.venue.isEmpty ? 'The venue' : event.venue,
                    style: const TextStyle(
                      color: Colors.white,
                      fontFamily: 'Manrope',
                      fontSize: 28,
                      height: 1.15,
                    ),
                  ),
                ],
              ),
            ),
            SliverPadding(
              padding: const EdgeInsets.fromLTRB(20, 22, 20, 36),
              sliver: SliverList.list(
                children: [
                  Reveal(
                    child: DetailPanel(
                      children: [
                        DetailFact(
                          Icons.calendar_today_outlined,
                          'Dates',
                          eventDateRange(event),
                        ),
                        if (address.isNotEmpty) ...[
                          const Divider(height: 1, indent: 62, color: cream),
                          DetailFact(Icons.place_outlined, 'Address', address),
                        ],
                      ],
                    ),
                  ),
                  if (event.venue.isNotEmpty || event.city.isNotEmpty) ...[
                    const SizedBox(height: 14),
                    Reveal(
                      order: 1,
                      child: FilledButton.icon(
                        onPressed: () =>
                            openLink(context, directionsUrl(event)),
                        icon: const Icon(Icons.directions_outlined),
                        label: Text(
                          content.text('venue.directions', 'Open directions'),
                        ),
                      ),
                    ),
                  ],
                  // Shown only once the organisers publish a floor plan.
                  if (venue.floorPlanUrl.isNotEmpty) ...[
                    const SizedBox(height: 10),
                    OutlinedButton.icon(
                      onPressed: () => openLink(context, venue.floorPlanUrl),
                      icon: const Icon(Icons.map_outlined),
                      label: const Text('Open venue floor plan'),
                    ),
                  ],
                  if (venue.arrival.isNotEmpty)
                    _DetailBlock(
                      'Arrival & check-in',
                      venue.arrival,
                      icon: Icons.login_rounded,
                    ),
                  if (venue.accessibility.isNotEmpty)
                    _DetailBlock(
                      'Accessibility',
                      venue.accessibility,
                      icon: Icons.accessible_rounded,
                    ),
                  _HelpDesk(venue, content.text('help.title', 'Need a hand?')),
                ],
              ),
            ),
          ],
        ),
      ),
    );
  }
}

class ActivitiesScreen extends StatelessWidget {
  const ActivitiesScreen({super.key});
  @override
  Widget build(BuildContext context) {
    final state = context.watch<ContentProvider>();
    final content = state.content!;
    return Scaffold(
      appBar: AppBar(title: const Text('Activities')),
      body: LiveRefresh(
        onRefresh: state.load,
        child: ListView(
          keyboardDismissBehavior: ScrollViewKeyboardDismissBehavior.onDrag,
          physics: const AlwaysScrollableScrollPhysics(),
          padding: const EdgeInsets.all(20),
          children: [
            Eyebrow(content.text('activities.eyebrow', 'BEYOND THE SESSIONS')),
            const SizedBox(height: 8),
            TitleText(
              content.text('activities.title', 'Discover. Meet. Connect.'),
            ),
            const SizedBox(height: 20),
            if (content.guide.activities.isEmpty) ...[
              const GuideNotice(
                title: 'Activity details to be announced',
                message: 'Times, locations and participation details will appear here as they are confirmed.',
                icon: Icons.local_activity_outlined,
              ),
              const SizedBox(height: 24),
              const Text(
                'Programme highlights',
                style: TextStyle(fontFamily: 'Manrope', fontSize: 18),
              ),
              const SizedBox(height: 8),
              for (final title in content.programmeHighlights)
                ListTile(
                  contentPadding: EdgeInsets.zero,
                  leading: const Icon(Icons.arrow_outward, color: forest),
                  title: Text(title),
                ),
            ] else
              for (final a in content.guide.activities)
                Card(
                  color: cream,
                  child: Padding(
                    padding: const EdgeInsets.all(18),
                    child: Column(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: [
                        Text(
                          a.title,
                          style: const TextStyle(
                            fontFamily: 'Manrope',
                            fontSize: 18,
                          ),
                        ),
                        if (a.description.isNotEmpty) ...[
                          const SizedBox(height: 10),
                          Text(a.description),
                        ],
                        for (final (i, line) in [
                          if (a.schedule.isNotEmpty) a.schedule,
                          if (a.location.isNotEmpty) a.location,
                        ].indexed) ...[
                          SizedBox(height: i == 0 ? 14 : 6),
                          Text(line, style: const TextStyle(color: muted)),
                        ],
                      ],
                    ),
                  ),
                ),
          ],
        ),
      ),
    );
  }
}

class FaqScreen extends StatefulWidget {
  const FaqScreen({super.key});
  @override
  State<FaqScreen> createState() => _FaqScreenState();
}

class _FaqScreenState extends State<FaqScreen> {
  String query = '';
  @override
  Widget build(BuildContext context) {
    final state = context.watch<ContentProvider>();
    final content = state.content!;
    final guide = content.guide;
    final items = guide.faqs.where(
      (f) => '${f.question} ${f.answer}'.toLowerCase().contains(
        query.toLowerCase(),
      ),
    );
    return Scaffold(
      appBar: AppBar(title: const Text('FAQs')),
      body: LiveRefresh(
        onRefresh: state.load,
        child: ListView(
          keyboardDismissBehavior: ScrollViewKeyboardDismissBehavior.onDrag,
          physics: const AlwaysScrollableScrollPhysics(),
          padding: const EdgeInsets.all(20),
          children: [
            Eyebrow(content.text('faqs.eyebrow', 'A LITTLE HELP')),
            const SizedBox(height: 8),
            TitleText(content.text('faqs.title', 'Good to know.')),
            const SizedBox(height: 20),
            if (guide.faqs.isEmpty)
              const GuideNotice(
                title: 'Event-day FAQs to be announced',
                message: 'Answers about entry, facilities and attending the conclave will appear here once confirmed.',
                icon: Icons.help_outline,
              )
            else ...[
              GuideSearchField(
                label: 'Search FAQs',
                onChanged: (v) => setState(() => query = v),
              ),
              const SizedBox(height: 16),
              if (items.isEmpty)
                const GuideNotice(
                  title: 'No matching answers',
                  message: 'Try another search term.',
                  icon: Icons.search_off,
                ),
              for (final f in items)
                Card(
                  child: ExpansionTile(
                    onExpansionChanged: (_) => AppFeedback.selection(),
                    key: ValueKey(f.id),
                    title: Text(f.question),
                    childrenPadding: const EdgeInsets.fromLTRB(16, 0, 16, 20),
                    children: [
                      Align(
                        alignment: Alignment.centerLeft,
                        child: Text(f.answer),
                      ),
                    ],
                  ),
                ),
            ],
            const SizedBox(height: 24),
            _HelpDesk(guide.venue, content.text('help.title', 'Need a hand?')),
            const SizedBox(height: 16),
            OutlinedButton.icon(
              onPressed: () =>
                  showGuidePage(context, const DelegateRegistrationScreen()),
              icon: const Icon(Icons.how_to_reg_outlined),
              label: const Text('Register as a delegate'),
            ),
          ],
        ),
      ),
    );
  }
}

class _DetailBlock extends StatelessWidget {
  const _DetailBlock(this.title, this.body, {this.icon});
  final IconData? icon;
  final String title, body;
  @override
  Widget build(BuildContext context) => Padding(
    padding: const EdgeInsets.only(top: 14),
    child: DetailPanel(
      padding: 18,
      children: [
        Row(
          children: [
            if (icon != null) ...[
              Icon(icon, color: forest, size: 20),
              const SizedBox(width: 10),
            ],
            Expanded(
              child: Text(
                title,
                style: const TextStyle(fontFamily: 'Manrope', fontSize: 17),
              ),
            ),
          ],
        ),
        const SizedBox(height: 10),
        Text(body, style: const TextStyle(color: muted, height: 1.55)),
      ],
    ),
  );
}

/// Event-day contacts. Absent entirely until staff publish one.
class _HelpDesk extends StatelessWidget {
  const _HelpDesk(this.venue, this.title);
  final VenueGuide venue;
  final String title;
  @override
  Widget build(BuildContext context) {
    if (!venue.hasHelp) return const SizedBox.shrink();
    final whatsapp = venue.helpWhatsApp.replaceAll(RegExp(r'[^0-9]'), '');
    return Padding(
      padding: const EdgeInsets.only(top: 14),
      child: DetailPanel(
        padding: 8,
        children: [
          Padding(
            padding: const EdgeInsets.fromLTRB(10, 10, 10, 4),
            child: Text(
              title,
              style: const TextStyle(fontFamily: 'Manrope', fontSize: 17),
            ),
          ),
          if (venue.helpPhone.isNotEmpty)
            _HelpRow(
              Icons.phone_outlined,
              venue.helpPhone,
              Uri(scheme: 'tel', path: venue.helpPhone).toString(),
            ),
          if (whatsapp.isNotEmpty)
            _HelpRow(
              Icons.chat_outlined,
              'WhatsApp ${venue.helpWhatsApp}',
              'https://wa.me/$whatsapp',
            ),
          if (venue.helpEmail.isNotEmpty)
            _HelpRow(
              Icons.mail_outline,
              venue.helpEmail,
              Uri(scheme: 'mailto', path: venue.helpEmail).toString(),
            ),
        ],
      ),
    );
  }
}

class _HelpRow extends StatelessWidget {
  const _HelpRow(this.icon, this.label, this.url);
  final IconData icon;
  final String label, url;
  @override
  Widget build(BuildContext context) => InkWell(
    borderRadius: BorderRadius.circular(14),
    onTap: () => openLink(context, url),
    child: Padding(
      padding: const EdgeInsets.all(10),
      child: Row(
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
            child: Text(
              label,
              style: const TextStyle(fontWeight: FontWeight.w600),
            ),
          ),
          const Icon(Icons.chevron_right, color: forest),
        ],
      ),
    ),
  );
}
