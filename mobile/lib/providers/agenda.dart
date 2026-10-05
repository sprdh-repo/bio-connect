import 'dart:convert';

import 'package:flutter/foundation.dart';

import '../models/event_content.dart';
import '../models/event_guide.dart';
import '../services/local_store.dart';
import '../services/reminders.dart';

/// What the attendee saved for their day: sessions, speakers and exhibitors,
/// with reminders scheduled before each saved session.
class Agenda extends ChangeNotifier {
  Agenda({LocalStore? store, Reminders? reminders, DateTime Function()? now})
    : _store = store ?? PreferencesStore(),
      reminders = reminders ?? LocalReminders(),
      _now = now ?? DateTime.now;

  static const _key = 'agenda_v1';
  static const reminderChoices = [5, 10, 15, 30];

  final LocalStore _store;
  final Reminders reminders;
  final DateTime Function() _now;

  final Set<String> _sessions = {}, _speakers = {};

  /// Saved exhibitors by [Exhibitor.key], kept whole so they show without
  /// waiting for the directory to load.
  final Map<String, Exhibitor> _exhibitors = {};
  bool remindersOn = true;
  int reminderMinutes = 10;
  bool loaded = false;
  String _scheduled = '';
  EventContent? _content;

  DateTime now() => _now();

  bool get isEmpty =>
      _sessions.isEmpty && _speakers.isEmpty && _exhibitors.isEmpty;
  bool hasSession(String id) => _sessions.contains(id);
  bool hasSpeaker(String id) => _speakers.contains(id);
  bool hasExhibitor(Exhibitor e) => _exhibitors.containsKey(e.key);
  List<Exhibitor> get exhibitors =>
      _exhibitors.values.toList()
        ..sort((a, b) => a.name.toLowerCase().compareTo(b.name.toLowerCase()));

  Future<void> load() async {
    try {
      final raw = await _store.read(_key);
      if (raw != null) {
        final json = jsonDecode(raw) as Map<String, dynamic>;
        _sessions.addAll(strings(json['sessions']));
        _speakers.addAll(strings(json['speakers']));
        for (final e in maps(json['exhibitors'])) {
          final exhibitor = Exhibitor.fromJson(e);
          _exhibitors[exhibitor.key] = exhibitor;
        }
        remindersOn = json['reminders_on'] != false;
        final minutes = json['reminder_minutes'];
        if (minutes is int && reminderChoices.contains(minutes)) {
          reminderMinutes = minutes;
        }
      }
    } catch (error) {
      // A damaged copy must not stop the app; the attendee starts afresh.
      debugPrint('Agenda could not be read: $error');
    }
    loaded = true;
    notifyListeners();
    if (_content case final content?) syncReminders(content);
  }

  Future<void> _save() async {
    notifyListeners();
    try {
      await _store.write(
        _key,
        jsonEncode({
          'sessions': _sessions.toList(),
          'speakers': _speakers.toList(),
          'exhibitors': _exhibitors.values.map((e) => e.toJson()).toList(),
          'reminders_on': remindersOn,
          'reminder_minutes': reminderMinutes,
        }),
      );
    } catch (error) {
      debugPrint('Agenda could not be saved: $error');
    }
  }

  /// Saves or removes [session]. When saving, returns the saved sessions it
  /// clashes with, so the caller can warn.
  List<GuideSession> toggleSession(GuideSession session, EventContent content) {
    if (_sessions.remove(session.id)) {
      _save();
      syncReminders(content);
      return const [];
    }
    _sessions.add(session.id);
    _save();
    if (remindersOn && session.startsAt != null) reminders.requestPermission();
    syncReminders(content);
    return clashesFor(session, content);
  }

  void toggleSpeaker(String id) {
    if (!_speakers.remove(id)) _speakers.add(id);
    _save();
  }

  void toggleExhibitor(Exhibitor e) {
    if (_exhibitors.remove(e.key) == null) _exhibitors[e.key] = e;
    _save();
  }

  /// Refreshes a saved exhibitor's details, such as a newly allocated stall.
  void refreshExhibitors(List<Exhibitor> directory) {
    var changed = false;
    for (final e in directory) {
      final saved = _exhibitors[e.key];
      if (saved != null &&
          jsonEncode(saved.toJson()) != jsonEncode(e.toJson())) {
        _exhibitors[e.key] = e;
        changed = true;
      }
    }
    if (changed) _save();
  }

  Future<void> setReminders(
    EventContent content, {
    bool? on,
    int? minutes,
  }) async {
    remindersOn = on ?? remindersOn;
    reminderMinutes = minutes ?? reminderMinutes;
    await _save();
    if (on == true) await reminders.requestPermission();
    syncReminders(content);
  }

  /// Saved sessions that are still published, in programme order.
  List<GuideSession> sessions(EventContent content) =>
      sortSessions(content.guide.sessions.where((s) => hasSession(s.id)));

  List<Speaker> speakers(EventContent content) =>
      content.speakers.where((s) => hasSpeaker(s.id)).toList();

  /// Saved sessions that overlap [session].
  List<GuideSession> clashesFor(GuideSession session, EventContent content) =>
      sessions(content).where(session.clashesWith).toList();

  /// Sessions of saved speakers the attendee has not saved yet.
  List<GuideSession> suggestions(EventContent content) {
    final ids = <String>{};
    for (final speaker in speakers(content)) {
      ids.addAll(content.sessionsOf(speaker).map((s) => s.id));
    }
    return sortSessions(
      content.guide.sessions.where(
        (s) => ids.contains(s.id) && !hasSession(s.id),
      ),
    );
  }

  /// The saved session under way and the next one to start.
  ({GuideSession? now, GuideSession? next}) nowAndNext(EventContent content) {
    final at = _now();
    GuideSession? current, next;
    for (final s in sessions(content)) {
      final start = s.startsAt, end = s.endsAt;
      if (start == null || end == null) continue;
      if (!start.isAfter(at) && end.isAfter(at)) {
        current ??= s;
      } else if (start.isAfter(at)) {
        next ??= s;
      }
    }
    return (now: current, next: next);
  }

  /// Schedules a reminder before each saved session that has yet to start.
  /// Cheap to call often: nothing is rescheduled unless the plan changed.
  void syncReminders(EventContent content) {
    _content = content;
    if (!loaded) return;
    final at = _now();
    final due = <Reminder>[
      if (remindersOn)
        for (final s in sessions(content))
          if (s.startsAt case final start?
              when start
                  .subtract(Duration(minutes: reminderMinutes))
                  .isAfter(at))
            Reminder(
              id: reminderId(s.id),
              at: start.subtract(Duration(minutes: reminderMinutes)),
              title: 'Starts in $reminderMinutes minutes',
              body: [
                s.title,
                if (s.location.isNotEmpty) s.location,
              ].join(' · '),
              payload: 'session:${s.id}',
            ),
    ];
    final signature = due
        .map((r) => '${r.id}|${r.at.toIso8601String()}|${r.body}')
        .join(',');
    if (signature == _scheduled) return;
    _scheduled = signature;
    reminders.replace(due);
  }
}

/// A stable positive notification ID for a session ID.
int reminderId(String sessionId) {
  var h = 0x811c9dc5;
  for (final unit in sessionId.codeUnits) {
    h = ((h ^ unit) * 0x01000193) & 0x7fffffff;
  }
  return h;
}

/// Timed sessions by start, then untimed ones by title.
List<GuideSession> sortSessions(Iterable<GuideSession> items) =>
    items.toList()..sort((a, b) {
      if (a.startsAt == null) {
        return b.startsAt == null ? a.title.compareTo(b.title) : 1;
      }
      if (b.startsAt == null) return -1;
      return a.startsAt!.compareTo(b.startsAt!);
    });
