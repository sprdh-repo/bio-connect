import 'dart:convert';

import 'package:flutter/services.dart';
import 'package:bio_connect_app/models/event_content.dart';
import 'package:bio_connect_app/models/event_guide.dart';
import 'package:bio_connect_app/screens/event_guide_screens.dart';
import 'package:bio_connect_app/main.dart';
import 'package:bio_connect_app/providers/content_provider.dart';
import 'package:bio_connect_app/services/content_service.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:provider/provider.dart';

void main() {
  setUp(() => rootBundle.clear());
  test('event times are displayed in IST', () {
    expect(sessionTime(DateTime.parse('2026-10-08T20:00:00Z')), '01:30');
    expect(sessionDay(DateTime.parse('2026-10-08T20:00:00Z')), '09/10/2026');
  });

  testWidgets('published sessions filter by day and search, and open details', (
    tester,
  ) async {
    final json = jsonDecode(
      await rootBundle.loadString('assets/content/event.json'),
    ) as Map<String, dynamic>;
    json['event_guide'] = {
      'sessions': [
        {
          'id': 'one',
          'title': 'Opening discussion',
          'starts_at': '2026-10-08T09:00:00+05:30',
          'ends_at': '2026-10-08T10:00:00+05:30',
          'location': 'Hall A',
          'speakers': 'Dr Test',
        },
        {
          'id': 'two',
          'title': 'Innovation forum',
          'starts_at': '2026-10-09T11:00:00+05:30',
          'ends_at': '2026-10-09T12:00:00+05:30',
        },
      ],
      'faqs': [
        {
          'id': 'faq',
          'question': 'Where is the help desk?',
          'answer': 'In the foyer.',
        },
      ],
    };
    final provider = ContentProvider(
      CurrentContentService(remoteContent: false),
    )..content = EventContent.fromJson(json);
    await tester.pumpWidget(
      ChangeNotifierProvider.value(
        value: provider,
        child: const BioConnectApp(),
      ),
    );
    await tester.pumpAndSettle();
    await tester.tap(find.text('Sessions').last);
    await tester.pumpAndSettle();
    await tester.tap(find.text('09/10/2026'));
    await tester.pumpAndSettle();
    expect(find.text('Innovation forum'), findsOneWidget);
    expect(find.text('Opening discussion'), findsNothing);
    await tester.tap(find.text('All days'));
    await tester.enterText(find.byType(TextField), 'Hall A');
    await tester.pumpAndSettle();
    await tester.tap(find.text('Opening discussion'));
    await tester.pumpAndSettle();
    expect(find.text('08/10/2026\n09:00 - 10:00 IST'), findsOneWidget);
    expect(find.text('Dr Test'), findsOneWidget);
    await tester.pageBack();
    await tester.pumpAndSettle();
    await tester.enterText(find.byType(TextField), 'no such session');
    await tester.pumpAndSettle();
    expect(find.text('No matching sessions'), findsOneWidget);
    await tester.pumpWidget(
      ChangeNotifierProvider.value(
        value: provider,
        child: const MaterialApp(home: FaqScreen()),
      ),
    );
    await tester.pumpAndSettle();
    await tester.tap(find.text('Where is the help desk?'));
    await tester.pumpAndSettle();
    expect(find.text('In the foyer.'), findsOneWidget);
    expect(tester.takeException(), isNull);
    provider.dispose();
  });

  testWidgets('home adapts to narrow screens and large text', (tester) async {
    tester.view.physicalSize = const Size(320, 740);
    tester.view.devicePixelRatio = 1;
    tester.platformDispatcher.textScaleFactorTestValue = 1.7;
    addTearDown(tester.view.resetPhysicalSize);
    addTearDown(tester.view.resetDevicePixelRatio);
    addTearDown(tester.platformDispatcher.clearTextScaleFactorTestValue);
    await tester.pumpWidget(
      ChangeNotifierProvider(
        create: (_) =>
            ContentProvider(CurrentContentService(remoteContent: false))
              ..load(),
        child: const BioConnectApp(),
      ),
    );
    await tester.pumpAndSettle();
    expect(tester.takeException(), isNull);
    await tester.drag(find.byType(ListView).first, const Offset(0, -550));
    await tester.pumpAndSettle();
    expect(tester.takeException(), isNull);
  });

  testWidgets('attendee can reach unpublished sessions, venue and FAQs', (
    tester,
  ) async {
    await tester.pumpWidget(
      ChangeNotifierProvider(
        create: (_) =>
            ContentProvider(CurrentContentService(remoteContent: false))
              ..load(),
        child: const BioConnectApp(),
      ),
    );
    await tester.pumpAndSettle();
    await tester.tap(find.text('Sessions').last);
    await tester.pumpAndSettle();
    expect(find.text('Session timetable to be announced'), findsOneWidget);
    await tester.tap(find.text('Guide').last);
    await tester.pumpAndSettle();
    await tester.tap(find.text('Venue & directions'));
    await tester.pumpAndSettle();
    // No floor plan is published, so nothing about one is shown.
    expect(find.text('Arrival & check-in'), findsOneWidget);
    expect(find.text('Open venue floor plan'), findsNothing);
    expect(find.text('Floor plan to be announced'), findsNothing);
    await tester.pageBack();
    await tester.pumpAndSettle();
    await tester.tap(find.text('FAQs'));
    await tester.pumpAndSettle();
    expect(find.text('Event-day FAQs to be announced'), findsOneWidget);
    expect(tester.takeException(), isNull);
  });
}
