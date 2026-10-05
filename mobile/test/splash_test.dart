import 'package:bio_connect_app/models/event_content.dart';
import 'package:bio_connect_app/providers/content_provider.dart';
import 'package:bio_connect_app/services/content_service.dart';
import 'package:bio_connect_app/widgets/splash.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:provider/provider.dart';

/// Content that arrives only when the test says so.
class _SlowContent implements ContentService {
  final _bundle = CurrentContentService(remoteContent: false);
  bool release = false;
  @override
  Future<EventContent> loadEvent() async {
    while (!release) {
      await Future<void>.delayed(const Duration(milliseconds: 100));
    }
    return _bundle.loadEvent();
  }

  @override
  Future<EventContent> fetchEvent(EventContent current) async => current;
  @override
  Future<EventContent> refreshEvent(EventContent current) async => current;
  @override
  Future<List<Exhibitor>> loadExhibitors() async => const [];
}

Future<_SlowContent> _pump(
  WidgetTester tester, {
  bool reduceMotion = false,
}) async {
  final service = _SlowContent();
  await tester.pumpWidget(
    ChangeNotifierProvider(
      create: (_) => ContentProvider(service)..load(),
      child: MediaQuery(
        data: MediaQueryData(disableAnimations: reduceMotion),
        child: const MaterialApp(home: SplashGate(child: Text('Home'))),
      ),
    ),
  );
  return service;
}

Finder _mark() => find.byWidgetPredicate(
  (w) =>
      w is Image &&
      w.image is AssetImage &&
      (w.image as AssetImage).assetName == 'assets/images/splash-mark.png',
);

void main() {
  setUp(() => rootBundle.clear());

  testWidgets('the first frame matches the native launch screen', (
    tester,
  ) async {
    final service = await _pump(tester);
    addTearDown(() => service.release = true);
    // Only the mark, centred at the native 240dp size; nothing else yet.
    final mark = tester.getRect(_mark());
    expect(mark.size, const Size(240, 240));
    expect(mark.center, tester.getCenter(find.byType(MaterialApp)));
    expect(
      tester
          .widget<Opacity>(
            find
                .ancestor(
                  of: find.image(
                    const AssetImage('assets/images/bio-connect-logo.png'),
                  ),
                  matching: find.byType(Opacity),
                )
                .first,
          )
          .opacity,
      0,
    );
    service.release = true;
    await tester.pumpAndSettle();
  });

  testWidgets('the splash waits for content, then reveals the app', (
    tester,
  ) async {
    final service = await _pump(tester);
    // The intro has played, but content is still loading: keep the splash.
    await tester.pump(const Duration(seconds: 3));
    expect(_mark(), findsOneWidget);
    expect(find.bySemanticsLabel('Bio Connect 4.0, loading'), findsOneWidget);

    service.release = true;
    // The splash stops animating once it has gone, so this settles.
    await tester.pumpAndSettle();
    expect(_mark(), findsNothing);
    expect(find.text('Home'), findsOneWidget);
  });

  testWidgets('reduced motion skips the intro and simply fades', (
    tester,
  ) async {
    (await _pump(tester, reduceMotion: true)).release = true;
    await tester.pumpAndSettle();
    expect(_mark(), findsNothing);
    expect(find.text('Home'), findsOneWidget);
  });
}
