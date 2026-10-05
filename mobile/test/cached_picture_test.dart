import 'package:bio_connect_app/widgets/directory.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  test('initials skip separators and symbols', () {
    expect(initials('Haier Biomedical & Helixpro'), 'HB');
    expect(initials('CSIR-NIIST'), 'CN');
    expect(initials('KIMSHEALTH'), 'K');
  });

  testWidgets('a logo that cannot load falls back to initials', (tester) async {
    await tester.pumpWidget(
      const MaterialApp(
        home: Scaffold(
          body: LogoBox(
            url: 'https://logos.invalid/expo-labs.png',
            name: 'Expo Labs',
          ),
        ),
      ),
    );
    await tester.pumpAndSettle();
    expect(find.text('EL'), findsOneWidget);
    expect(find.byType(SkeletonBlock), findsNothing);
  });

  testWidgets('an empty logo URL shows initials without a request', (
    tester,
  ) async {
    await tester.pumpWidget(
      const MaterialApp(
        home: Scaffold(
          body: LogoBox(url: '', name: 'Mid Labs'),
        ),
      ),
    );
    expect(find.text('ML'), findsOneWidget);
    expect(find.byType(CachedPicture), findsNothing);
  });
}
