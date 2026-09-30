import 'dart:async';

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

class _PendingRegistrationService extends _FakeRegistrationService {
  final result = Completer<RegistrationResult>();
  int submissions = 0;
  @override
  Future<RegistrationResult> registerDelegate({
    required String categoryID,
    required String institution,
    required String name,
    required String designation,
    required String email,
    required String phone,
    required bool whatsappConsent,
  }) {
    submissions++;
    return result.future;
  }
}

Future<void> openRegistration(
  WidgetTester tester,
  RegistrationService service,
) async {
  await tester.pumpWidget(
    MaterialApp(
      home: Builder(
        builder: (context) => Scaffold(
          body: TextButton(
            onPressed: () => Navigator.push(
              context,
              MaterialPageRoute<void>(
                builder: (_) => DelegateRegistrationScreen(service: service),
              ),
            ),
            child: const Text('Open registration'),
          ),
        ),
      ),
    ),
  );
  await tester.tap(find.text('Open registration'));
  await tester.pumpAndSettle();
}

void main() {
  testWidgets('back protects edits and clean registration closes normally', (
    tester,
  ) async {
    await openRegistration(tester, _FakeRegistrationService());
    await tester.binding.handlePopRoute();
    await tester.pumpAndSettle();
    expect(find.text('Open registration'), findsOneWidget);
    await tester.tap(find.text('Open registration'));
    await tester.pumpAndSettle();
    await tester.enterText(
      find.widgetWithText(TextFormField, 'Full name'),
      'Asha',
    );
    await tester.pump();
    await tester.binding.handlePopRoute();
    await tester.pumpAndSettle();
    expect(find.text('Discard registration details?'), findsOneWidget);
    await tester.tap(find.text('Keep editing'));
    await tester.pumpAndSettle();
    expect(find.text('Asha'), findsOneWidget);
    await tester.pageBack();
    await tester.pumpAndSettle();
    await tester.tap(find.text('Discard'));
    await tester.pumpAndSettle();
    expect(find.text('Open registration'), findsOneWidget);
  });

  testWidgets('saving blocks back and completed registration leaves the form', (
    tester,
  ) async {
    final service = _PendingRegistrationService();
    await openRegistration(tester, service);
    for (final entry in {
      'Full name': 'Asha Nair',
      'Designation': 'Researcher',
      'Institution / organisation': 'Test Institute',
      'Email': 'asha@example.com',
      'Phone with country code': '+919876543210',
    }.entries) {
      await tester.scrollUntilVisible(
        find.widgetWithText(TextFormField, entry.key),
        200,
        scrollable: find.byType(Scrollable).first,
      );
      await tester.enterText(
        find.widgetWithText(TextFormField, entry.key),
        entry.value,
      );
    }
    await tester.scrollUntilVisible(
      find.text('Save registration'),
      400,
      scrollable: find.byType(Scrollable).first,
    );
    tester.testTextInput.hide();
    await tester.ensureVisible(find.byType(CheckboxListTile).last);
    await tester.pumpAndSettle();
    await tester.tap(find.byType(Checkbox).last);
    await tester.pumpAndSettle();
    await tester.ensureVisible(find.text('Save registration'));
    await tester.pumpAndSettle();
    await tester.tap(find.text('Save registration'));
    await tester.pump();
    expect(service.submissions, 1);
    await tester.binding.handlePopRoute();
    await tester.pump();
    expect(find.text('Saving your registration. Please wait.'), findsOneWidget);
    expect(service.submissions, 1);
    service.result.complete(
      const RegistrationResult(referenceUrl: 'https://example.com/payment'),
    );
    await tester.pumpAndSettle();
    expect(find.text('Registration saved'), findsOneWidget);
    await tester.binding.handlePopRoute();
    await tester.pumpAndSettle();
    expect(find.text('Open registration'), findsOneWidget);
    expect(find.text('Save registration'), findsNothing);
  });

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
