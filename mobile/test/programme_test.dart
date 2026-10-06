import 'package:bio_connect_app/main.dart';
import 'package:bio_connect_app/models/event_content.dart';
import 'package:bio_connect_app/models/event_guide.dart';
import 'package:bio_connect_app/providers/agenda.dart';
import 'package:bio_connect_app/providers/content_provider.dart';
import 'package:bio_connect_app/screens/event_guide_screens.dart';
import 'package:bio_connect_app/services/content_service.dart';
import 'package:bio_connect_app/services/local_store.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:provider/provider.dart';

import 'agenda_test.dart' show FakeReminders, morning;

Map<String, dynamic> person(
  String name, {
  String designation = '',
  String role = '',
  String speaker = '',
}) => {
  'name': name,
  'designation': designation,
  'role': role,
  'speaker_id': speaker,
};

String at(String hm) => '2026-10-08T$hm:00+05:30';

EventContent programme() => EventContent.fromJson({
  'event': {
    'title': 'Bio Connect 4.0',
    'start_date': '2026-10-08',
    'end_date': '2026-10-09',
  },
  'speakers': [
    {'id': 'iyer', 'name': 'Dr. Mahesh Iyer', 'role': 'Head of Data'},
  ],
  'event_guide': {
    'sessions': [
      {
        'id': 'inaugural',
        'kind': 'ceremony',
        'title': 'Inaugural Session',
        'starts_at': at('09:00'),
        'ends_at': at('10:00'),
        'location': 'Grand Ball Room',
        'segments': [
          {
            'title': 'Welcome Address',
            'starts_at': at('09:03'),
            'ends_at': at('09:10'),
            'people': [
              person('Shri. Snehil Kumar Singh IAS', designation: 'MD, KSIDC'),
            ],
          },
          {'title': 'Media interaction, if any'},
        ],
      },
      {
        'id': 'tea',
        'kind': 'break',
        'title': 'Tea Break',
        'starts_at': at('10:00'),
        'ends_at': at('10:10'),
      },
      {
        'id': 'panel',
        'kind': 'panel',
        'label': 'Panel Discussion 1',
        'title': 'Digital Innovation',
        'track': 'AI',
        'starts_at': at('10:10'),
        'ends_at': at('11:00'),
        'speaker_ids': ['iyer'],
        'people': [
          person(
            'Dr. Mahesh Iyer',
            designation: 'Head, BMS',
            role: 'moderator',
            speaker: 'iyer',
          ),
          person(
            'Dr. Naveen Sivadasan',
            designation: 'Principal Scientist, TCS',
            role: 'panelist',
          ),
          person('Dr. Ajitesh Lunge', role: 'panelist'),
        ],
      },
      {
        'id': 'talk',
        'kind': 'talk',
        'title': 'Presentation',
        'starts_at': at('10:10'),
        'ends_at': at('10:20'),
      },
    ],
  },
});

void main() {
  test('programme entries parse people, running orders and kinds', () {
    final sessions = programme().guide.sessions;
    final ceremony = sessions.first, tea = sessions[1], panel = sessions[2];
    expect(ceremony.segments.map((s) => s.title), [
      'Welcome Address',
      'Media interaction, if any',
    ]);
    expect(ceremony.segments.last.startsAt, isNull);
    expect(panel.people.first.role, 'moderator');
    expect(panel.searchText, contains('principal scientist, tcs'));
    expect(ceremony.searchText, contains('snehil'));
    expect(tea.plannable, isFalse);
    expect(
      sessionPeopleSummary(panel),
      'Moderated by Dr. Mahesh Iyer · 2 panellists',
    );
    expect(sessionPeopleSummary(ceremony), 'Running order · 1 item');
  });

  test('breaks never clash, and equal starts list the shorter first', () {
    final sessions = programme().guide.sessions;
    final tea = sessions[1], panel = sessions[2], talk = sessions[3];
    final early = GuideSession(
      id: 'x',
      title: 'Overlaps tea',
      startsAt: DateTime.parse(at('10:05')),
      endsAt: DateTime.parse(at('10:15')),
    );
    expect(early.clashesWith(tea), isFalse);
    expect(early.clashesWith(panel), isTrue);
    expect(sortSessions([panel, talk]).map((s) => s.id), ['talk', 'panel']);
    expect(sortSessions([talk, panel]).map((s) => s.id), ['talk', 'panel']);
  });

  test('a saved session that became a break leaves the agenda', () async {
    final agenda = Agenda(
      store: MemoryStore(),
      reminders: FakeReminders(),
      now: () => morning,
    );
    await agenda.load();
    final c = programme();
    agenda.toggleSession(c.guide.sessions[1], c);
    expect(agenda.sessions(c), isEmpty);
  });

  test('initials skip honorifics', () {
    expect(personInitials('Shri. A. P. M. Mohammed Hanish IAS'), 'AH');
    expect(personInitials('Padma Bhushan Prof. T. Ramasami'), 'TR');
    expect(personInitials('Master of Ceremonies'), 'MC');
    expect(personInitials('Venkatachalam'), 'V');
  });

  testWidgets(
    'the timetable opens on today, frames breaks and shows who is on stage',
    (tester) async {
      final provider = ContentProvider(
        CurrentContentService(remoteContent: false),
      )..content = programme();
      final agenda = Agenda(
        store: MemoryStore(),
        reminders: FakeReminders(),
        now: () => morning,
      );
      await tester.pumpWidget(
        ChangeNotifierProvider.value(
          value: provider,
          child: BioConnectApp(agenda: agenda),
        ),
      );
      await tester.pumpAndSettle();
      await tester.tap(find.text('Sessions').last);
      await tester.pumpAndSettle();

      // 09:45 on the first day: today is selected and the ceremony is live.
      final today = tester.widget<ChoiceChip>(
        find.widgetWithText(ChoiceChip, 'Thu 8 Oct · Today'),
      );
      expect(today.selected, isTrue);
      expect(find.text('HAPPENING NOW'), findsOneWidget);
      await tester.scrollUntilVisible(
        find.byTooltip('Save Digital Innovation to my agenda'),
        200,
        scrollable: find
            .ancestor(
              of: find.text('HAPPENING NOW'),
              matching: find.byType(Scrollable),
            )
            .first,
      );
      expect(find.text('Tea Break'), findsOneWidget);
      expect(find.byTooltip('Save Tea Break to my agenda'), findsNothing);
      expect(
        find.byTooltip('Save Digital Innovation to my agenda'),
        findsOneWidget,
      );

      // A panellist's designation finds their panel; breaks drop out of a search.
      await tester.scrollUntilVisible(
        find.byType(TextField),
        -200,
        scrollable: find.byType(Scrollable).hitTestable().first,
      );
      await tester.enterText(find.byType(TextField), 'tcs');
      await tester.pumpAndSettle();
      expect(find.text('Digital Innovation'), findsOneWidget);
      expect(find.text('Tea Break'), findsNothing);

      await tester.tap(find.text('Digital Innovation'));
      await tester.pumpAndSettle();
      expect(find.text('PANEL DISCUSSION 1'), findsOneWidget);
      expect(find.text('MODERATOR'), findsOneWidget);
      expect(find.text('PANELLISTS'), findsOneWidget);
      expect(find.text('Principal Scientist, TCS'), findsOneWidget);
      expect(find.text('AI'), findsOneWidget);
      // Linked people open their speaker profile.
      await tester.tap(find.text('Dr. Mahesh Iyer'));
      await tester.pumpAndSettle();
      expect(find.text('CONCLAVE SPEAKER'), findsOneWidget);
      await tester.pageBack();
      await tester.pumpAndSettle();
      await tester.pageBack();
      await tester.pumpAndSettle();

      await tester.enterText(find.byType(TextField), '');
      await tester.pumpAndSettle();
      await tester.tap(find.text('Inaugural Session').last);
      await tester.pumpAndSettle();
      expect(find.text('Running order'), findsOneWidget);
      expect(find.text('Welcome Address'), findsOneWidget);
      expect(find.text('Shri. Snehil Kumar Singh IAS'), findsOneWidget);
      expect(find.text('Media interaction, if any'), findsOneWidget);
      expect(tester.takeException(), isNull);
      provider.dispose();
    },
  );
}
