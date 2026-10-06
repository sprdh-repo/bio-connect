import 'package:bio_connect_app/models/event_content.dart';
import 'package:bio_connect_app/providers/content_provider.dart';
import 'package:bio_connect_app/providers/pass_wallet.dart';
import 'package:bio_connect_app/screens/event_guide_screens.dart';
import 'package:bio_connect_app/screens/my_passes_screen.dart';
import 'package:bio_connect_app/screens/selfie_frame_screen.dart';
import 'package:bio_connect_app/services/content_service.dart';
import 'package:bio_connect_app/services/pass_service.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:provider/provider.dart';

import 'mobile_control_test.dart' show bundled, pumpApp;
import 'pass_wallet_test.dart' show MemoryPassStore, access, pass;

final event = EventDetails.fromJson({
  'title': 'Bio Connect 4.0',
  'start_date': '2026-10-08',
  'end_date': '2026-10-09',
});

class _PassService extends PassService {
  @override
  Future<PassAccess> refresh(PassAccess a) async => a;
}

/// Bundled content without the selfie frame in any menu.
Map<String, dynamic> withoutFrame() {
  final json = bundled();
  final menus = json['menus'] as Map<String, dynamic>;
  for (final name in ['home_links', 'guide']) {
    menus[name] = [
      for (final entry in menus[name] as List)
        if (entry['key'] != 'selfie_frame') entry,
    ];
  }
  return json;
}

Future<PassWallet> pumpPasses(
  WidgetTester tester,
  Map<String, dynamic> json,
  AdmissionPass holder,
) async {
  final provider = ContentProvider(CurrentContentService(remoteContent: false))
    ..content = EventContent.fromJson(json);
  addTearDown(provider.dispose);
  final wallet = PassWallet(
    service: _PassService(),
    store: MemoryPassStore([
      access(passes: [holder]),
    ]),
  );
  addTearDown(wallet.dispose);
  await tester.pumpWidget(
    ChangeNotifierProvider.value(
      value: provider,
      child: MaterialApp(home: MyPassesScreen(wallet: wallet)),
    ),
  );
  await tester.pumpAndSettle();
  return wallet;
}

AdmissionPass holderIn(String category) => AdmissionPass(
  id: pass.id,
  name: pass.name,
  institution: pass.institution,
  designation: pass.designation,
  category: category,
  number: pass.number,
  qrId: pass.qrId,
  downloadUrl: pass.downloadUrl,
);

void main() {
  setUp(() => rootBundle.clear());

  test('the caption follows the event dates and the pass holder role', () {
    final before = DateTime(2026, 10, 7, 23, 59);
    final firstDay = DateTime(2026, 10, 8);
    final lastDay = DateTime(2026, 10, 9, 22);
    final after = DateTime(2026, 10, 10);

    expect(eventPhase(event, before), EventPhase.before);
    expect(eventPhase(event, firstDay), EventPhase.during);
    expect(eventPhase(event, lastDay), EventPhase.during);
    expect(eventPhase(event, after), EventPhase.after);

    expect(frameCaption(event, before), 'See you at');
    expect(frameCaption(event, lastDay), "I'm attending");
    expect(frameCaption(event, after), 'I was at');
    expect(
      frameCaption(event, before, passCategory: 'Speaker'),
      "I'm speaking at",
    );
    expect(
      frameCaption(event, firstDay, passCategory: 'Premium stall (6m × 3m)'),
      "I'm exhibiting at",
    );
    expect(frameCaption(event, after, passCategory: 'Speaker'), 'I was at');
    for (final caption in [
      frameCaption(event, before),
      frameCaption(event, firstDay, passCategory: 'Table space (2m × 2m)'),
    ]) {
      expect(frameCaptions, contains(caption));
    }
  });

  testWidgets('the home banner is worded for the event phase', (tester) async {
    final content = EventContent.fromJson(bundled());
    final entry = content.menu('home_links').first;
    for (final (now, title) in [
      (DateTime(2026, 10, 1), "Tell your network\nyou're coming."),
      (DateTime(2026, 10, 8), "Tell your network\nyou're here."),
      (DateTime(2026, 10, 12), 'Share that you\nwere there.'),
    ]) {
      await tester.pumpWidget(
        MaterialApp(
          home: Scaffold(body: SelfieFramePromo(content, entry, now: now)),
        ),
      );
      expect(find.text(title), findsOneWidget);
    }
  });

  testWidgets('a published frame link becomes the home banner', (tester) async {
    await pumpApp(tester, bundled());
    expect(find.byType(SelfieFramePromo), findsOneWidget);
    // The banner replaces the plain link rather than repeating it.
    expect(
      find.descendant(
        of: find.byType(AttendeeHomeScreen),
        matching: find.text('Selfie frame'),
      ),
      findsNothing,
    );

    await tester.ensureVisible(find.text('Create your frame'));
    await tester.pumpAndSettle();
    await tester.tap(find.text('Create your frame'));
    // The camera loader keeps animating, so pump past the page transition.
    await tester.pump();
    await tester.pump(const Duration(seconds: 1));
    expect(find.byType(SelfieFrameScreen), findsOneWidget);
  });

  testWidgets('no banner while staff do not offer the frame', (tester) async {
    await pumpApp(tester, withoutFrame());
    expect(find.byType(SelfieFramePromo), findsNothing);
  });

  testWidgets('a saved speaker pass opens the frame with its role', (
    tester,
  ) async {
    await pumpPasses(tester, bundled(), holderIn('Speaker'));
    final prompt = find.textContaining('Share it.');
    final now = DateTime.now();
    if (eventPhase(EventContent.fromJson(bundled()).event, now) ==
        EventPhase.after) {
      expect(find.text('Share that you were there.'), findsOneWidget);
      return;
    }
    await tester.scrollUntilVisible(
      prompt,
      200,
      scrollable: find
          .descendant(
            of: find.byType(ListView),
            matching: find.byType(Scrollable),
          )
          .first,
    );
    await tester.tap(prompt);
    await tester.pump();
    await tester.pump(const Duration(seconds: 1));
    expect(find.byType(SelfieFrameScreen), findsOneWidget);
    expect(find.text("I'M SPEAKING AT"), findsOneWidget);
  });

  testWidgets('no pass prompt while staff do not offer the frame', (
    tester,
  ) async {
    await pumpPasses(tester, withoutFrame(), pass);
    expect(find.text('Asha Nair'), findsOneWidget);
    expect(find.textContaining('Bio Connect frame'), findsNothing);
  });
}
