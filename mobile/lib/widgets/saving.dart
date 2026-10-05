import 'package:flutter/material.dart';
import 'package:provider/provider.dart';

import '../main.dart';
import '../models/event_content.dart';
import '../models/event_guide.dart';
import '../providers/agenda.dart';
import '../providers/content_provider.dart';
import '../services/calendar.dart';
import 'interaction.dart';

/// Saves or removes a session, and warns when it overlaps one already saved.
void toggleSavedSession(BuildContext context, GuideSession session) {
  final content = context.read<ContentProvider>().content;
  if (content == null) return;
  final agenda = context.read<Agenda>();
  final saving = !agenda.hasSession(session.id);
  final clashes = agenda.toggleSession(session, content);
  AppFeedback.selection();
  final messenger = ScaffoldMessenger.of(context)..hideCurrentSnackBar();
  messenger.showSnackBar(
    SnackBar(
      content: Text(switch ((saving, clashes)) {
        (false, _) => 'Removed from your agenda',
        (true, []) => 'Saved to your agenda',
        (true, [final other]) => 'Saved. It overlaps “${other.title}”.',
        (true, final others) =>
          'Saved. It overlaps ${others.length} sessions in your agenda.',
      }),
      action: SnackBarAction(
        label: 'Undo',
        onPressed: () => agenda.toggleSession(session, content),
      ),
    ),
  );
}

/// Adds sessions to the phone's calendar and reports the result.
Future<void> addSessionsToCalendar(
  BuildContext context,
  List<GuideSession> sessions,
) async {
  final content = context.read<ContentProvider>().content;
  final messenger = ScaffoldMessenger.of(context);
  final timed = sessions.where((s) => s.startsAt != null && s.endsAt != null);
  if (timed.isEmpty) {
    messenger.showSnackBar(
      const SnackBar(
        content: Text('Timings for this session are not confirmed yet.'),
      ),
    );
    return;
  }
  AppFeedback.action();
  try {
    final added = await calendar.add(
      timed.toList(),
      venue: content?.event.venue ?? '',
    );
    if (added == 0 && context.mounted) {
      messenger.showSnackBar(
        const SnackBar(
          content: Text('No calendar app could open this session.'),
        ),
      );
    }
  } catch (_) {
    if (context.mounted) {
      messenger.showSnackBar(
        const SnackBar(
          content: Text('Could not open your calendar. Please try again.'),
        ),
      );
    }
  }
}

/// The calendar used by the app; tests replace it.
SessionCalendar calendar = const DeviceCalendar();

/// A save toggle for lists: a bookmark that fills when saved.
class SaveIcon extends StatelessWidget {
  const SaveIcon({
    super.key,
    required this.saved,
    required this.onPressed,
    required this.label,
    this.color = forest,
  });
  final bool saved;
  final VoidCallback onPressed;
  final String label;
  final Color color;
  @override
  Widget build(BuildContext context) => IconButton(
    tooltip: saved
        ? 'Remove $label from my agenda'
        : 'Save $label to my agenda',
    onPressed: onPressed,
    color: color,
    icon: AnimatedSwitcher(
      duration: const Duration(milliseconds: 220),
      transitionBuilder: (child, animation) =>
          ScaleTransition(scale: animation, child: child),
      child: Icon(
        saved ? Icons.bookmark_rounded : Icons.bookmark_add_outlined,
        key: ValueKey(saved),
      ),
    ),
  );
}

/// A full-width save toggle for detail pages.
class SaveButton extends StatelessWidget {
  const SaveButton({
    super.key,
    required this.saved,
    required this.onPressed,
    this.saveLabel = 'Save to my agenda',
  });
  final bool saved;
  final VoidCallback onPressed;
  final String saveLabel;
  @override
  Widget build(BuildContext context) => saved
      ? OutlinedButton.icon(
          onPressed: onPressed,
          icon: const Icon(Icons.bookmark_rounded),
          label: const Text('Saved to my agenda'),
        )
      : FilledButton.icon(
          onPressed: onPressed,
          icon: const Icon(Icons.bookmark_add_outlined),
          label: Text(saveLabel),
        );
}

/// "09:30 - 10:15 IST", or the start alone when the end is unknown.
String sessionSpan(GuideSession s) => s.startsAt == null
    ? ''
    : '${sessionTime(s.startsAt!)}${s.endsAt == null ? '' : ' - ${sessionTime(s.endsAt!)}'} IST';

/// "in 25 min", "in 2 h 5 min" or "tomorrow" until [at].
String startsIn(DateTime at, DateTime now) {
  final d = at.difference(now);
  if (d.inMinutes < 1) return 'starting now';
  if (indiaTime(at).day != indiaTime(now).day && d.inHours >= 12) {
    return d.inHours < 36 ? 'tomorrow' : 'on ${sessionDay(at)}';
  }
  if (d.inMinutes < 60) return 'in ${d.inMinutes} min';
  final m = d.inMinutes % 60;
  return 'in ${d.inHours} h${m == 0 ? '' : ' $m min'}';
}

extension AgendaContent on EventContent {
  GuideSession? session(String id) {
    for (final s in guide.sessions) {
      if (s.id == id) return s;
    }
    return null;
  }

  Speaker? speaker(String id) {
    for (final s in speakers) {
      if (s.id == id) return s;
    }
    return null;
  }
}
