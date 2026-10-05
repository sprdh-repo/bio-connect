import 'package:bio_connect_app/main.dart';
import 'package:bio_connect_app/models/event_content.dart';
import 'package:bio_connect_app/services/registration_service.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

class _RegistrationService extends RegistrationService {
  @override
  Future<RegistrationOptions> options() async => RegistrationOptions(
    enabled: true,
    cutoff: DateTime.utc(2026, 9, 30),
    categories: const [
      RegistrationCategory(
        id: 'student',
        kind: 'delegate',
        label: 'Students',
        earlyPaise: 100000,
        regularPaise: 150000,
        payablePaise: 150000,
        rosterCount: 1,
        open: true,
        freeOnly: false,
      ),
      RegistrationCategory(
        id: 'premium',
        kind: 'exhibitor',
        label: 'Premium stall',
        earlyPaise: 1000000,
        regularPaise: 1200000,
        payablePaise: 1200000,
        rosterCount: 5,
        open: false,
        freeOnly: false,
      ),
    ],
  );
}

EventDetails event({required bool showExhibitorRegistration}) =>
    EventDetails.fromJson({
      'title': 'Bio Connect 4.0',
      'start_date': '2026-10-08',
      'end_date': '2026-10-09',
      'show_exhibitor_registration': showExhibitorRegistration,
    });

void main() {
  testWidgets('admin toggle hides only exhibitor registration', (tester) async {
    await tester.pumpWidget(
      MaterialApp(
        home: RegistrationScreen(
          event(showExhibitorRegistration: false),
          service: _RegistrationService(),
        ),
      ),
    );
    await tester.pumpAndSettle();

    expect(find.text('DELEGATE PASSES'), findsOneWidget);
    expect(find.text('Students'), findsOneWidget);
    expect(find.text('EXHIBITION SPACE'), findsNothing);
    expect(find.text('Premium stall'), findsNothing);
    expect(find.text('Book exhibition space'), findsNothing);
    expect(find.textContaining('Exhibition'), findsNothing);
    expect(find.text('Register for\nBio Connect.'), findsOneWidget);
  });

  testWidgets('admin toggle can restore exhibitor registration', (
    tester,
  ) async {
    await tester.pumpWidget(
      MaterialApp(
        home: RegistrationScreen(
          event(showExhibitorRegistration: true),
          service: _RegistrationService(),
        ),
      ),
    );
    await tester.pumpAndSettle();

    expect(find.text('EXHIBITION SPACE'), findsOneWidget);
    expect(find.text('Premium stall'), findsOneWidget);
    expect(find.text('Book exhibition space'), findsOneWidget);
  });
}
