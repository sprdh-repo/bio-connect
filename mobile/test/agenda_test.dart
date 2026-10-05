import 'dart:convert';

import 'package:bio_connect_app/main.dart';
import 'package:bio_connect_app/models/event_content.dart';
import 'package:bio_connect_app/providers/agenda.dart';
import 'package:bio_connect_app/providers/content_provider.dart';
import 'package:bio_connect_app/screens/agenda_screen.dart';
import 'package:bio_connect_app/screens/event_guide_screens.dart';
import 'package:bio_connect_app/services/content_service.dart';
import 'package:bio_connect_app/services/local_store.dart';
import 'package:bio_connect_app/services/reminders.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:provider/provider.dart';

class FakeReminders implements Reminders {
  List<Reminder> scheduled = [];
  int replaced = 0, permissionRequests = 0;
  void Function(String)? onTap;
  @override
  Future<void> init(void Function(String payload) onTap) async =>
      this.onTap = onTap;
  @override
  Future<String?> launchPayload() async => null;
  @override
  Future<bool> requestPermission() async {
    permissionRequests++;
    return true;
  }

  @override
  Future<void> replace(List<Reminder> reminders) async {
    replaced++;
    scheduled = reminders;
  }
}

Map<String, dynamic> session(
  String id,
  String start,
  String end, {
  List<String> speakers = const [],
  String names = '',
}) => {
  'id': id,
  'title': 'Session $id',
  'starts_at': start.isEmpty ? '' : '2026-10-08T$start:00+05:30',
  'ends_at': end.isEmpty ? '' : '2026-10-08T$end:00+05:30',
  'location': 'Hall $id',
  'speakers': names,
  'speaker_ids': speakers,
};

EventContent content() => EventContent.fromJson({
  'event': {
    'title': 'Bio Connect 4.0',
    'start_date': '2026-10-08',
    'end_date': '2026-10-09',
  },
  'speakers': [
    {'id': 'asha', 'name': 'Dr. Asha Nair'},
    {'id': 'ravi', 'name': 'Ravi Menon'},
  ],
  'event_guide': {
    'sessions': [
      session('a', '09:00', '10:00', speakers: ['asha']),
      session('b', '09:30', '10:30'),
      session('c', '11:00', '12:00', names: 'Ravi Menon, Others'),
      session('d', '', ''),
    ],
  },
});

/// 08 Oct 2026, 09:45 IST.
final morning = DateTime.parse('2026-10-08T09:45:00+05:30');

void main() {
  test(
    'saving reports clashes, and Now and Up next follow the clock',
    () async {
      final reminders = FakeReminders();
      var now = morning;
      final agenda = Agenda(
        store: MemoryStore(),
        reminders: reminders,
        now: () => now,
      );
      await agenda.load();
      final c = content();
      final [a, b, cc, _] = c.guide.sessions;
      expect(agenda.toggleSession(a, c), isEmpty);
      expect(agenda.toggleSession(b, c).map((s) => s.id), ['a']);
      expect(agenda.toggleSession(cc, c), isEmpty);
      expect(reminders.permissionRequests, 3);
      expect(agenda.clashesFor(a, c).map((s) => s.id), ['b']);
      expect(agenda.clashesFor(cc, c), isEmpty);

      var plan = agenda.nowAndNext(c);
      expect(plan.now?.id, 'a');
      expect(plan.next?.id, 'c');
      now = DateTime.parse('2026-10-08T12:30:00+05:30');
      plan = agenda.nowAndNext(c);
      expect((plan.now, plan.next), (null, null));

      // Removing a session clears it, and it no longer clashes with anything.
      agenda.toggleSession(b, c);
      expect(agenda.hasSession('b'), isFalse);
      expect(agenda.clashesFor(a, c), isEmpty);
    },
  );

  test('saved speakers suggest their sessions, by link or by name', () async {
    final agenda = Agenda(store: MemoryStore(), reminders: FakeReminders());
    await agenda.load();
    final c = content();
    agenda.toggleSpeaker('asha');
    agenda.toggleSpeaker('ravi');
    expect(agenda.suggestions(c).map((s) => s.id), ['a', 'c']);
    agenda.toggleSession(c.guide.sessions.first, c);
    expect(agenda.suggestions(c).map((s) => s.id), ['c']);
    // A linked session lists only its linked speakers, not name matches.
    expect(c.sessionsOf(c.speakers.first).map((s) => s.id), ['a']);
  });

  test(
    'reminders cover upcoming saved sessions only, and follow settings',
    () async {
      final reminders = FakeReminders();
      final agenda = Agenda(
        store: MemoryStore(),
        reminders: reminders,
        now: () => morning,
      );
      await agenda.load();
      final c = content();
      for (final s in c.guide.sessions) {
        agenda.toggleSession(s, c);
      }
      // a and b have begun and d has no time: only c is reminded.
      expect(reminders.scheduled.single.payload, 'session:c');
      expect(
        reminders.scheduled.single.at,
        DateTime.parse('2026-10-08T10:50:00+05:30'),
      );
      expect(reminders.scheduled.single.body, 'Session c · Hall c');

      final calls = reminders.replaced;
      agenda.syncReminders(c);
      expect(
        reminders.replaced,
        calls,
        reason: 'unchanged plans are not rescheduled',
      );

      await agenda.setReminders(c, minutes: 30);
      expect(
        reminders.scheduled.single.at,
        DateTime.parse('2026-10-08T10:30:00+05:30'),
      );
      await agenda.setReminders(c, on: false);
      expect(reminders.scheduled, isEmpty);
    },
  );

  test('the agenda survives a restart; a damaged copy starts afresh', () async {
    final store = MemoryStore();
    final c = content();
    final first = Agenda(store: store, reminders: FakeReminders());
    await first.load();
    first
      ..toggleSession(c.guide.sessions.first, c)
      ..toggleSpeaker('ravi')
      ..toggleExhibitor(
        const Exhibitor(
          id: 'BC4-EX-0001',
          name: 'Acme',
          description: '',
          logoUrl: '',
        ),
      );
    await first.setReminders(c, minutes: 15);

    final second = Agenda(store: store, reminders: FakeReminders());
    await second.load();
    expect(second.hasSession('a'), isTrue);
    expect(second.hasSpeaker('ravi'), isTrue);
    expect(second.exhibitors.single.name, 'Acme');
    expect(second.reminderMinutes, 15);

    // A refreshed directory updates the saved copy, e.g. a new stall.
    second.refreshExhibitors(const [
      Exhibitor(
        id: 'BC4-EX-0001',
        name: 'Acme',
        description: '',
        logoUrl: '',
        stallNumber: 'A-4',
      ),
    ]);
    expect(second.exhibitors.single.stallNumber, 'A-4');

    final broken = Agenda(
      store: MemoryStore({scopedKey('agenda_v1'): 'not json'}),
      reminders: FakeReminders(),
    );
    await broken.load();
    expect(broken.loaded, isTrue);
    expect(broken.isEmpty, isTrue);
  });

  testWidgets(
    'a session saved from the programme appears in My agenda with its clash',
    (tester) async {
      final json = jsonDecode(
        await rootBundle.loadString('assets/content/event.json'),
      ) as Map<String, dynamic>;
      json['event_guide'] = {
        'sessions': [
          session('a', '09:00', '10:00'),
          session('b', '09:30', '10:30'),
        ],
      };
      final provider = ContentProvider(
        CurrentContentService(remoteContent: false),
      )..content = EventContent.fromJson(json);
      final reminders = FakeReminders();
      await tester.pumpWidget(
        ChangeNotifierProvider.value(
          value: provider,
          child: BioConnectApp(
            agenda: Agenda(
              store: MemoryStore(),
              reminders: reminders,
              now: () => DateTime.parse('2026-10-08T08:00:00+05:30'),
            ),
          ),
        ),
      );
      await tester.pumpAndSettle();

      await tester.tap(find.text('Sessions').last);
      await tester.pumpAndSettle();
      await tester.tap(find.byTooltip('Save Session a to my agenda'));
      await tester.pump();
      expect(find.text('Saved to your agenda'), findsOneWidget);
      final saveB = find.byTooltip('Save Session b to my agenda');
      // Several tabs keep scrollables alive; scroll the Sessions list.
      await tester.scrollUntilVisible(
        saveB,
        120,
        scrollable: find
            .descendant(
              of: find.byType(SessionsScreen),
              matching: find.byType(Scrollable),
            )
            .first,
      );
      await tester.ensureVisible(saveB);
      await tester.pumpAndSettle();
      await tester.tap(saveB);
      // The earlier message makes way for this one.
      await tester.pumpAndSettle();
      expect(find.text('Saved. It overlaps “Session a”.'), findsOneWidget);
      await tester.pumpAndSettle(const Duration(seconds: 5));

      await tester.tap(find.text('My agenda').last);
      await tester.pumpAndSettle();
      expect(find.text('UP NEXT'), findsOneWidget);
      expect(find.text('Overlaps “Session b”'), findsOneWidget);
      await tester.scrollUntilVisible(
        find.text('Overlaps “Session a”'),
        120,
        scrollable: find
            .descendant(
              of: find.byType(AgendaScreen),
              matching: find.byType(Scrollable),
            )
            .first,
      );
      expect(reminders.scheduled.map((r) => r.payload), [
        'session:a',
        'session:b',
      ]);

      // A tapped reminder opens its session.
      reminders.onTap!('session:b');
      await tester.pumpAndSettle();
      expect(find.text('Session details'), findsOneWidget);
      expect(find.text('Session b'), findsWidgets);
      expect(find.text('Overlaps your agenda'), findsOneWidget);
    },
  );
}
