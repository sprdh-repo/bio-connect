import 'dart:async';

import 'package:flutter/material.dart';
import 'package:provider/provider.dart';

import '../main.dart';
import '../models/event_content.dart';
import '../models/event_guide.dart';
import '../providers/agenda.dart';
import '../providers/content_provider.dart';
import '../widgets/destinations.dart';
import '../widgets/directory.dart';
import '../widgets/interaction.dart';
import '../widgets/motion.dart';
import '../widgets/saving.dart';
import 'event_guide_screens.dart';
import 'exhibitors_screen.dart';

/// My agenda as a pushed page, for when it is not a bottom tab.
class AgendaPage extends StatelessWidget {
  const AgendaPage({super.key, this.title = 'My agenda'});
  final String title;
  @override
  Widget build(BuildContext context) => Scaffold(
    appBar: AppBar(title: Text(title)),
    body: const AgendaScreen(),
  );
}

/// The attendee's day: what is on now and next, saved sessions by day with
/// clashes marked, suggestions from saved speakers, and saved exhibitors.
class AgendaScreen extends StatefulWidget {
  const AgendaScreen({super.key});
  @override
  State<AgendaScreen> createState() => _AgendaScreenState();
}

class _AgendaScreenState extends State<AgendaScreen> {
  String? _day;

  /// Sessions picked for the calendar; null when not picking.
  Set<String>? _picking;
  Timer? _tick;

  @override
  void initState() {
    super.initState();
    // Keeps "Now" and "Up next" current while the page is open.
    _tick = Timer.periodic(const Duration(minutes: 1), (_) {
      if (mounted) setState(() {});
    });
  }

  @override
  void dispose() {
    _tick?.cancel();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final state = context.watch<ContentProvider>();
    final agenda = context.watch<Agenda>();
    final content = state.content!;
    final saved = agenda.sessions(content);
    final days = {
      for (final s in saved)
        if (s.startsAt != null) sessionDay(s.startsAt!),
    }.toList();
    final today = sessionDay(agenda.now());
    final day = days.contains(_day)
        ? _day!
        : days.contains(today)
        ? today
        : days.isEmpty
        ? ''
        : days.first;
    final shown = saved
        .where((s) => s.startsAt != null && sessionDay(s.startsAt!) == day)
        .toList();
    final untimed = saved.where((s) => s.startsAt == null).toList();
    final suggestions = agenda.suggestions(content);
    final speakers = agenda.speakers(content);
    final exhibitors = agenda.exhibitors;

    return LiveRefresh(
      onRefresh: state.load,
      child: ListView(
        keyboardDismissBehavior: ScrollViewKeyboardDismissBehavior.onDrag,
        physics: const AlwaysScrollableScrollPhysics(),
        padding: const EdgeInsets.fromLTRB(20, 24, 20, 30),
        children: [
          PageIntro(
            eyebrow: content.text('agenda.eyebrow', 'YOUR DAY PLAN'),
            title: content.text('agenda.title', 'Your Bio Connect,\nyour way.'),
          ),
          const SizedBox(height: 20),
          if (!agenda.loaded)
            const Center(child: BioLoader(width: 60))
          else if (agenda.isEmpty)
            _EmptyAgenda(content)
          else ...[
            const AgendaGlance(showEmpty: true),
            if (saved.isNotEmpty) ...[
              const SizedBox(height: 26),
              Row(
                children: [
                  Expanded(
                    child: SectionLabel(
                      'Saved sessions',
                      trailing: _picking == null ? '${saved.length}' : '',
                    ),
                  ),
                  if (_picking == null && shown.isNotEmpty)
                    TextButton.icon(
                      onPressed: () {
                        AppFeedback.selection();
                        setState(
                          () => _picking = {for (final s in shown) s.id},
                        );
                      },
                      icon: const Icon(
                        Icons.event_available_outlined,
                        size: 18,
                      ),
                      label: const Text('Add to calendar'),
                    )
                  else if (_picking != null)
                    TextButton(
                      onPressed: () => setState(() => _picking = null),
                      child: const Text('Cancel'),
                    ),
                ],
              ),
              if (days.length > 1) ...[
                const SizedBox(height: 8),
                Wrap(
                  spacing: 8,
                  children: [
                    for (final d in days)
                      ChoiceChip(
                        label: Text(_dayLabel(d, today)),
                        selected: d == day,
                        onSelected: (_) {
                          AppFeedback.selection();
                          setState(() => _day = d);
                        },
                      ),
                  ],
                ),
              ],
              const SizedBox(height: 12),
              for (final s in shown)
                _AgendaSessionTile(
                  s,
                  clashes: agenda.clashesFor(s, content),
                  now: agenda.now(),
                  picked: _picking?.contains(s.id),
                  onPick: (v) => setState(
                    () => v ? _picking!.add(s.id) : _picking!.remove(s.id),
                  ),
                ),
              if (untimed.isNotEmpty && _picking == null) ...[
                const SizedBox(height: 10),
                const Eyebrow('TIME TO BE CONFIRMED'),
                const SizedBox(height: 8),
                for (final s in untimed)
                  _AgendaSessionTile(s, clashes: const [], now: agenda.now()),
              ],
              if (_picking != null) ...[
                const SizedBox(height: 8),
                FilledButton.icon(
                  onPressed: _picking!.isEmpty
                      ? null
                      : () async {
                          final picked = saved
                              .where((s) => _picking!.contains(s.id))
                              .toList();
                          setState(() => _picking = null);
                          await addSessionsToCalendar(context, picked);
                        },
                  icon: const Icon(Icons.event_available_outlined),
                  label: Text(switch (_picking!.length) {
                    0 => 'Pick sessions to add',
                    1 => 'Add 1 session to calendar',
                    final n => 'Add $n sessions to calendar',
                  }),
                ),
                const SizedBox(height: 6),
                const Text(
                  'Your calendar opens for each session so you can confirm it.',
                  style: TextStyle(color: muted, fontSize: 12),
                ),
              ],
            ],
            if (suggestions.isNotEmpty) ...[
              const SizedBox(height: 26),
              const SectionLabel('From speakers you saved'),
              const SizedBox(height: 10),
              for (final s in suggestions)
                _AgendaSessionTile(
                  s,
                  clashes: agenda.clashesFor(s, content),
                  now: agenda.now(),
                  suggested: true,
                ),
            ],
            if (speakers.isNotEmpty) ...[
              const SizedBox(height: 26),
              SectionLabel('Speakers', trailing: '${speakers.length}'),
              const SizedBox(height: 10),
              SizedBox(
                height: 168,
                child: ListView.separated(
                  scrollDirection: Axis.horizontal,
                  itemCount: speakers.length,
                  separatorBuilder: (_, _) => const SizedBox(width: 10),
                  itemBuilder: (context, i) => _SpeakerChip(speakers[i]),
                ),
              ),
            ],
            if (exhibitors.isNotEmpty) ...[
              const SizedBox(height: 26),
              SectionLabel('Exhibitors', trailing: '${exhibitors.length}'),
              const SizedBox(height: 10),
              for (final e in exhibitors) ExhibitorTile(e),
            ],
            const SizedBox(height: 26),
            _ReminderSettings(content),
          ],
        ],
      ),
    );
  }
}

String _dayLabel(String day, String today) {
  if (day == today) return 'Today';
  final [d, m, _] = day.split('/');
  const months = [
    'Jan',
    'Feb',
    'Mar',
    'Apr',
    'May',
    'Jun',
    'Jul',
    'Aug',
    'Sep',
    'Oct',
    'Nov',
    'Dec',
  ];
  return '${int.parse(d)} ${months[int.parse(m) - 1]}';
}

class _EmptyAgenda extends StatelessWidget {
  const _EmptyAgenda(this.content);
  final EventContent content;
  @override
  Widget build(BuildContext context) => Column(
    crossAxisAlignment: CrossAxisAlignment.stretch,
    children: [
      const GuideNotice(
        icon: Icons.bookmark_add_outlined,
        title: 'Plan your Bio Connect',
        message: 'Tap the bookmark on any session, speaker or exhibitor to save it here. You get a reminder before each saved session, and a warning when two overlap.',
      ),
      const SizedBox(height: 16),
      if (content.guide.sessions.isNotEmpty)
        FilledButton.icon(
          onPressed: () => openDestination(
            context,
            content,
            const MenuEntry('sessions', 'Sessions'),
          ),
          icon: const Icon(Icons.calendar_month_outlined),
          label: const Text('Browse sessions'),
        ),
      if (content.speakers.isNotEmpty) ...[
        const SizedBox(height: 10),
        OutlinedButton.icon(
          onPressed: () => openDestination(
            context,
            content,
            const MenuEntry('speakers', 'Speakers'),
          ),
          icon: const Icon(Icons.people_outline),
          label: const Text('Meet the speakers'),
        ),
      ],
      const SizedBox(height: 10),
      OutlinedButton.icon(
        onPressed: () => openDestination(
          context,
          content,
          const MenuEntry('exhibitors', 'Exhibitors'),
        ),
        icon: const Icon(Icons.storefront_outlined),
        label: const Text('Explore exhibitors'),
      ),
    ],
  );
}

/// "Now" and "Up next" from the saved sessions. Shown on Home and the agenda.
class AgendaGlance extends StatelessWidget {
  const AgendaGlance({
    super.key,
    this.showEmpty = false,
    this.padding = EdgeInsets.zero,
  });

  /// Whether to say so when nothing saved is still to come.
  final bool showEmpty;

  /// Space around the card, left out when there is nothing to show.
  final EdgeInsets padding;

  @override
  Widget build(BuildContext context) {
    final content = context.watch<ContentProvider>().content;
    final agenda = context.watch<Agenda>();
    if (content == null) return const SizedBox.shrink();
    final (:now, :next) = agenda.nowAndNext(content);
    if (now == null && next == null) {
      if (!showEmpty || agenda.sessions(content).isEmpty) {
        return const SizedBox.shrink();
      }
      return Padding(
        padding: padding,
        child: const GuideNotice(
          icon: Icons.event_available_outlined,
          title: 'Nothing else saved for later',
          message: 'Your saved sessions have finished. Browse the programme for what is still to come.',
        ),
      );
    }
    final at = agenda.now();
    return Padding(
      padding: padding,
      child: Container(
        padding: const EdgeInsets.all(20),
        decoration: BoxDecoration(
          gradient: const LinearGradient(
            begin: Alignment.topLeft,
            end: Alignment.bottomRight,
            colors: [forest, deepForest],
          ),
          borderRadius: BorderRadius.circular(24),
        ),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            if (now != null)
              _GlanceRow(
                label: 'NOW',
                live: true,
                session: now,
                detail: [
                  if (now.endsAt case final end?) 'until ${sessionTime(end)}',
                  if (now.location.isNotEmpty) now.location,
                ].join(' · '),
              ),
            if (now != null && next != null)
              Divider(height: 28, color: Colors.white.withValues(alpha: .14)),
            if (next != null)
              _GlanceRow(
                label: 'UP NEXT',
                session: next,
                detail: [
                  '${sessionTime(next.startsAt!)}, ${startsIn(next.startsAt!, at)}',
                  if (next.location.isNotEmpty) next.location,
                ].join(' · '),
              ),
          ],
        ),
      ),
    );
  }
}

class _GlanceRow extends StatelessWidget {
  const _GlanceRow({
    required this.label,
    required this.session,
    required this.detail,
    this.live = false,
  });
  final String label, detail;
  final GuideSession session;
  final bool live;
  @override
  Widget build(BuildContext context) => InkWell(
    onTap: () => showGuidePage(context, SessionDetailScreen(session.id)),
    borderRadius: BorderRadius.circular(12),
    child: Row(
      children: [
        Expanded(
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Row(
                children: [
                  if (live) ...[const _LiveDot(), const SizedBox(width: 7)],
                  Text(
                    label,
                    style: const TextStyle(
                      color: lime,
                      fontSize: 11,
                      letterSpacing: 1.5,
                      fontWeight: FontWeight.w700,
                    ),
                  ),
                ],
              ),
              const SizedBox(height: 8),
              Text(
                session.title,
                style: const TextStyle(
                  fontFamily: 'Manrope',
                  color: Colors.white,
                  fontSize: 18,
                  height: 1.25,
                ),
              ),
              if (detail.isNotEmpty) ...[
                const SizedBox(height: 6),
                Text(
                  detail,
                  style: TextStyle(
                    color: Colors.white.withValues(alpha: .75),
                    fontSize: 13,
                  ),
                ),
              ],
            ],
          ),
        ),
        const SizedBox(width: 8),
        const Icon(Icons.chevron_right, color: lime),
      ],
    ),
  );
}

/// A softly pulsing dot for the session under way.
class _LiveDot extends StatefulWidget {
  const _LiveDot();
  @override
  State<_LiveDot> createState() => _LiveDotState();
}

class _LiveDotState extends State<_LiveDot>
    with SingleTickerProviderStateMixin {
  late final _pulse = AnimationController(
    vsync: this,
    duration: const Duration(milliseconds: 1400),
  )..repeat(reverse: true);
  @override
  void dispose() {
    _pulse.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) => FadeTransition(
    opacity: Tween(begin: .35, end: 1.0).animate(_pulse),
    child: Container(
      width: 8,
      height: 8,
      decoration: const BoxDecoration(color: lime, shape: BoxShape.circle),
    ),
  );
}

class _AgendaSessionTile extends StatelessWidget {
  const _AgendaSessionTile(
    this.session, {
    required this.clashes,
    required this.now,
    this.suggested = false,
    this.picked,
    this.onPick,
  });
  final GuideSession session;
  final List<GuideSession> clashes;
  final DateTime now;
  final bool suggested;

  /// Whether the session is picked for the calendar; null when not picking.
  final bool? picked;
  final ValueChanged<bool>? onPick;

  @override
  Widget build(BuildContext context) {
    final s = session;
    final over = s.endsAt != null && !s.endsAt!.isAfter(now);
    final live =
        s.startsAt != null &&
        !s.startsAt!.isAfter(now) &&
        s.endsAt != null &&
        s.endsAt!.isAfter(now);
    return Opacity(
      opacity: over && picked == null ? .55 : 1,
      child: Card(
        color: suggested ? Colors.white : cream,
        margin: const EdgeInsets.only(bottom: 10),
        child: InkWell(
          onTap: picked != null
              ? () => onPick!(!picked!)
              : () => showGuidePage(context, SessionDetailScreen(s.id)),
          child: Padding(
            padding: const EdgeInsets.fromLTRB(16, 14, 6, 14),
            child: Row(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                if (s.startsAt != null)
                  SizedBox(
                    width: 52,
                    child: Column(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: [
                        Text(
                          sessionTime(s.startsAt!),
                          style: const TextStyle(
                            fontFamily: 'Manrope',
                            fontSize: 16,
                            color: forest,
                          ),
                        ),
                        if (s.endsAt != null)
                          Text(
                            sessionTime(s.endsAt!),
                            style: const TextStyle(color: muted, fontSize: 12),
                          ),
                      ],
                    ),
                  ),
                Expanded(
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      if (live) ...[
                        const Pill(
                          'ON NOW',
                          background: forest,
                          foreground: lime,
                        ),
                        const SizedBox(height: 6),
                      ],
                      Text(
                        s.title,
                        style: const TextStyle(
                          fontFamily: 'Manrope',
                          fontSize: 15,
                          height: 1.3,
                        ),
                      ),
                      if (suggested && s.startsAt != null) ...[
                        const SizedBox(height: 4),
                        Text(
                          sessionDay(s.startsAt!),
                          style: const TextStyle(color: muted, fontSize: 12),
                        ),
                      ],
                      if (s.location.isNotEmpty) ...[
                        const SizedBox(height: 4),
                        Text(
                          s.location,
                          style: const TextStyle(color: muted, fontSize: 12),
                        ),
                      ],
                      if (clashes.isNotEmpty) ...[
                        const SizedBox(height: 8),
                        _ClashNote(clashes, suggested: suggested),
                      ],
                    ],
                  ),
                ),
                if (picked != null)
                  Checkbox(value: picked, onChanged: (v) => onPick!(v ?? false))
                else
                  SaveIcon(
                    saved: !suggested,
                    label: 'this session',
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

class _ClashNote extends StatelessWidget {
  const _ClashNote(this.clashes, {this.suggested = false});
  final List<GuideSession> clashes;
  final bool suggested;
  @override
  Widget build(BuildContext context) => Container(
    padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 6),
    decoration: BoxDecoration(
      color: gold.withValues(alpha: .2),
      borderRadius: BorderRadius.circular(10),
    ),
    child: Row(
      mainAxisSize: MainAxisSize.min,
      children: [
        const Icon(Icons.warning_amber_rounded, size: 16, color: ink),
        const SizedBox(width: 6),
        Flexible(
          child: Text(
            clashes.length == 1
                ? '${suggested ? 'Would overlap' : 'Overlaps'} “${clashes.first.title}”'
                : '${suggested ? 'Would overlap' : 'Overlaps'} ${clashes.length} saved sessions',
            style: const TextStyle(fontSize: 12, height: 1.3),
          ),
        ),
      ],
    ),
  );
}

class _SpeakerChip extends StatelessWidget {
  const _SpeakerChip(this.speaker);
  final Speaker speaker;
  @override
  Widget build(BuildContext context) => SizedBox(
    width: 112,
    child: InkWell(
      borderRadius: BorderRadius.circular(18),
      onTap: () => showGuidePage(context, SpeakerDetailScreen(speaker)),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          ClipRRect(
            borderRadius: BorderRadius.circular(18),
            child: SizedBox(
              width: 112,
              height: 112,
              child: ColoredBox(color: cream, child: SpeakerImage(speaker)),
            ),
          ),
          const SizedBox(height: 7),
          Text(
            speaker.name,
            maxLines: 2,
            overflow: TextOverflow.ellipsis,
            style: const TextStyle(fontSize: 12, fontWeight: FontWeight.w700),
          ),
        ],
      ),
    ),
  );
}

class _ReminderSettings extends StatelessWidget {
  const _ReminderSettings(this.content);
  final EventContent content;
  @override
  Widget build(BuildContext context) {
    final agenda = context.watch<Agenda>();
    // Material, not a decorated box, so the switch's ink shows.
    return Material(
      color: Colors.white,
      borderRadius: BorderRadius.circular(20),
      child: Padding(
        padding: const EdgeInsets.fromLTRB(16, 6, 8, 14),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            SwitchListTile(
              contentPadding: EdgeInsets.zero,
              value: agenda.remindersOn,
              onChanged: (on) {
                AppFeedback.selection();
                agenda.setReminders(content, on: on);
              },
              title: const Text(
                'Session reminders',
                style: TextStyle(fontWeight: FontWeight.w700),
              ),
              subtitle: const Text(
                'A notification before each saved session starts, even offline.',
                style: TextStyle(color: muted, fontSize: 12),
              ),
            ),
            if (agenda.remindersOn)
              Wrap(
                spacing: 8,
                children: [
                  for (final m in Agenda.reminderChoices)
                    ChoiceChip(
                      label: Text('$m min before'),
                      selected: agenda.reminderMinutes == m,
                      onSelected: (_) {
                        AppFeedback.selection();
                        agenda.setReminders(content, minutes: m);
                      },
                    ),
                ],
              ),
          ],
        ),
      ),
    );
  }
}
