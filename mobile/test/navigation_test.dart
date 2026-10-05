import 'package:bio_connect_app/main.dart';
import 'package:bio_connect_app/screens/event_guide_screens.dart';
import 'package:bio_connect_app/providers/content_provider.dart';
import 'package:bio_connect_app/services/content_service.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:bio_connect_app/widgets/nav_bar.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:provider/provider.dart';

void main() {
  setUp(() => rootBundle.clear());
  testWidgets('tabs keep their position and reselect scrolls to the top', (
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
    final list = find
        .descendant(
          of: find.byType(AttendeeHomeScreen),
          matching: find.byType(Scrollable),
        )
        .first;
    await tester.drag(list, const Offset(0, -400));
    await tester.pumpAndSettle();
    final position = tester.state<ScrollableState>(list).position.pixels;
    expect(position, greaterThan(0));
    await tester.tap(find.text('Sessions').last);
    await tester.pumpAndSettle();
    await tester.tap(find.text('Home').last);
    await tester.pumpAndSettle();
    expect(tester.state<ScrollableState>(list).position.pixels, position);
    await tester.tap(find.text('Home').last);
    await tester.pumpAndSettle();
    expect(tester.state<ScrollableState>(list).position.pixels, 0);
  });

  testWidgets('back returns tabs home before asking to exit', (tester) async {
    final calls = <MethodCall>[];
    tester.binding.defaultBinaryMessenger.setMockMethodCallHandler(
      SystemChannels.platform,
      (call) async {
        calls.add(call);
        return null;
      },
    );
    addTearDown(
      () => tester.binding.defaultBinaryMessenger.setMockMethodCallHandler(
        SystemChannels.platform,
        null,
      ),
    );
    await tester.pumpWidget(
      ChangeNotifierProvider(
        create: (_) =>
            ContentProvider(CurrentContentService(remoteContent: false))
              ..load(),
        child: const BioConnectApp(),
      ),
    );
    await tester.pumpAndSettle();
    for (final tab in ['Sessions', 'Speakers', 'Guide']) {
      await tester.tap(find.text(tab).last);
      await tester.pumpAndSettle();
      await tester.binding.handlePopRoute();
      await tester.pumpAndSettle();
      expect(tester.widget<BioNavBar>(find.byType(BioNavBar)).selectedIndex, 0);
      expect(find.text('Exit Bio Connect?'), findsNothing);
    }
    await tester.binding.handlePopRoute();
    await tester.pumpAndSettle();
    expect(find.text('Exit Bio Connect?'), findsOneWidget);
    await tester.tap(find.text('Stay'));
    await tester.pumpAndSettle();
    expect(calls.where((c) => c.method == 'SystemNavigator.pop'), isEmpty);
    await tester.binding.handlePopRoute();
    await tester.pumpAndSettle();
    await tester.tap(find.text('Exit'));
    await tester.pumpAndSettle();
    expect(calls.where((c) => c.method == 'SystemNavigator.pop'), hasLength(1));
    expect(
      calls.where((c) => c.method == 'HapticFeedback.vibrate'),
      isNotEmpty,
    );
  });

  testWidgets('detail back returns to its tab; search can be cleared', (
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
    await tester.tap(find.text('Guide').last);
    await tester.pumpAndSettle();
    await tester.tap(find.text('Venue & directions'));
    await tester.pumpAndSettle();
    await tester.binding.handlePopRoute();
    await tester.pumpAndSettle();
    expect(tester.widget<BioNavBar>(find.byType(BioNavBar)).selectedIndex, 3);
    await tester.tap(find.text('Speakers').last);
    await tester.pumpAndSettle();
    await tester.enterText(find.byType(TextField), 'nonexistent');
    await tester.pumpAndSettle();
    await tester.tap(find.byTooltip('Clear search'));
    await tester.pumpAndSettle();
    expect(
      tester.widget<TextField>(find.byType(TextField)).controller!.text,
      isEmpty,
    );
  });
}
