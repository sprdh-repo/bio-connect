import 'package:bio_connect_app/main.dart';
import 'package:bio_connect_app/providers/content_provider.dart';
import 'package:bio_connect_app/services/content_service.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:provider/provider.dart';

void main() {
  testWidgets('bundled content supports navigation and speaker search', (
    tester,
  ) async {
    await tester.pumpWidget(
      ChangeNotifierProvider(
        create: (_) => ContentProvider(CurrentContentService())..load(),
        child: const BioConnectApp(),
      ),
    );
    await tester.pumpAndSettle();

    expect(find.text('Where science\nmeets what’s next.'), findsOneWidget);
    await tester.tap(find.text('Speakers').last);
    await tester.pumpAndSettle();
    expect(find.text('The voices\ntaking the stage.'), findsOneWidget);
    await tester.enterText(find.byType(TextField), 'Jayakrishna Ambati');
    await tester.pumpAndSettle();
    expect(find.text('1 speakers'), findsOneWidget);
    expect(find.text('Dr. Jayakrishna Ambati'), findsOneWidget);
  });
}
