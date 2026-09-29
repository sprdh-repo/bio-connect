import 'package:bio_connect_app/screens/delegate_registration_screen.dart';
import 'package:bio_connect_app/services/registration_service.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

class _FakeRegistrationService extends RegistrationService {
  @override
  Future<List<RegistrationCategory>> delegateCategories() async => const [
    RegistrationCategory(
      id: 'student',
      kind: 'delegate',
      label: 'Students',
      earlyPaise: 100000,
      regularPaise: 150000,
      payablePaise: 100000,
      rosterCount: 1,
      open: true,
      freeOnly: false,
    ),
    RegistrationCategory(
      id: 'industry',
      kind: 'delegate',
      label: 'Industry',
      earlyPaise: 600000,
      regularPaise: 700000,
      payablePaise: 600000,
      rosterCount: 1,
      open: true,
      freeOnly: false,
    ),
  ];
}

void main() {
  testWidgets('delegate registration loads live passes and validates details', (
    tester,
  ) async {
    await tester.pumpWidget(
      MaterialApp(
        home: DelegateRegistrationScreen(service: _FakeRegistrationService()),
      ),
    );
    await tester.pumpAndSettle();

    expect(find.text('Students'), findsOneWidget);
    expect(find.text('Industry'), findsOneWidget);
    expect(find.text('₹6,000'), findsOneWidget);

    await tester.scrollUntilVisible(
      find.text('Save registration'),
      400,
      scrollable: find.byType(Scrollable).first,
    );
    await tester.tap(find.text('Save registration'));
    await tester.pump();

    expect(find.text('This field is required'), findsNWidgets(4));
    expect(
      find.text('Use international format, e.g. +919876543210'),
      findsOneWidget,
    );
  });
}
