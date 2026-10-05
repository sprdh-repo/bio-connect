import 'dart:async';

import 'package:add_2_calendar/add_2_calendar.dart';
import 'package:flutter/foundation.dart';
import 'package:flutter/widgets.dart';

import '../models/event_guide.dart';

/// Adds sessions to the phone's calendar through its own "new event" screen,
/// so the attendee confirms each one and no calendar permission is needed.
abstract interface class SessionCalendar {
  /// Adds [sessions] one after another. Returns how many were handed over.
  Future<int> add(List<GuideSession> sessions, {String venue = ''});
}

class DeviceCalendar implements SessionCalendar {
  const DeviceCalendar();

  @override
  Future<int> add(List<GuideSession> sessions, {String venue = ''}) async {
    var added = 0;
    for (final s in sessions) {
      if (s.startsAt == null || s.endsAt == null) continue;
      final event = Event(
        title: s.title,
        description: [
          if (s.speakers.isNotEmpty) s.speakers,
          if (s.description.isNotEmpty) s.description,
        ].join('\n\n'),
        location: [s.location, venue].where((v) => v.isNotEmpty).join(', '),
        startDate: s.startsAt!,
        endDate: s.endsAt!,
        timeZone: 'Asia/Kolkata',
      );
      // Android opens the calendar and returns at once; wait until the
      // attendee comes back before opening the next session.
      final back = defaultTargetPlatform == TargetPlatform.android
          ? _nextResume()
          : null;
      final ok = await Add2Calendar.addEvent2Cal(event);
      if (!ok) break;
      added++;
      if (back != null && s != sessions.last) await back;
    }
    return added;
  }

  Future<void> _nextResume() {
    final done = Completer<void>();
    late final AppLifecycleListener listener;
    var left = false;
    listener = AppLifecycleListener(
      onStateChange: (state) {
        if (state != AppLifecycleState.resumed) left = true;
        if (state == AppLifecycleState.resumed && left && !done.isCompleted) {
          listener.dispose();
          done.complete();
        }
      },
    );
    // Carry on if the calendar never took over the screen.
    Timer(const Duration(seconds: 3), () {
      if (!left && !done.isCompleted) {
        listener.dispose();
        done.complete();
      }
    });
    return done.future;
  }
}
