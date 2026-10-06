import 'dart:typed_data';

import 'package:bio_connect_app/models/event_content.dart';
import 'package:bio_connect_app/screens/selfie_frame_screen.dart';
import 'package:bio_connect_app/services/photo_export.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

final event = EventDetails.fromJson({
  'title': 'Bio Connect 4.0',
  'start_date': '2026-10-08',
  'end_date': '2026-10-09',
  'venue': 'Hyatt Regency Trivandrum',
  'city': 'Thiruvananthapuram, Kerala',
});

class FakeExport extends PhotoExport {
  final saved = <Uint8List>[];
  final shared = <String?>[];

  @override
  Future<void> save(Uint8List bytes, String fileName) async => saved.add(bytes);

  @override
  Future<void> share(
    Uint8List bytes,
    String fileName, {
    Rect? origin,
    String? text,
  }) async => shared.add(text);
}

int pngWidth(Uint8List png) =>
    ByteData.sublistView(png, 16, 20).getUint32(0, Endian.big);

Widget app(FakeExport export, {String? photo}) => MaterialApp(
  home: SelfieFrameScreen(
    event,
    export: export,
    cameras: () async => [],
    pickImage: () async => photo,
  ),
);

/// Taps, then lets real async work (file decoding, frame rendering) and
/// frames alternate until the screen settles.
Future<void> tapAndWait(WidgetTester tester, Finder target) async {
  await tester.tap(target);
  for (var i = 0; i < 6; i++) {
    await tester.runAsync(
      () => Future<void>.delayed(const Duration(milliseconds: 100)),
    );
    await tester.pump();
  }
}

void main() {
  testWidgets('without a camera, offers the gallery instead', (tester) async {
    await tester.pumpWidget(app(FakeExport()));
    await tester.pump();

    expect(find.textContaining('No camera is available'), findsOneWidget);
    expect(find.byTooltip('Choose from gallery'), findsOneWidget);
    expect(find.byTooltip('Switch camera'), findsNothing);
    final takePhoto = tester.widget<ButtonStyleButton>(
      find.ancestor(
        of: find.text('Take photo'),
        matching: find.bySubtype<ButtonStyleButton>(),
      ),
    );
    expect(takePhoto.onPressed, isNull);
  });

  testWidgets('the chosen caption and event details appear on the frame', (
    tester,
  ) async {
    await tester.pumpWidget(app(FakeExport()));
    await tester.pump();

    expect(find.text("I'M ATTENDING"), findsOneWidget);
    expect(find.text('Bio Connect 4.0'), findsOneWidget);
    expect(
      find.text('08-09 October 2026  ·  Hyatt Regency Trivandrum'),
      findsOneWidget,
    );

    await tester.tap(find.text("I'm speaking at"));
    await tester.pump();
    expect(find.text("I'M SPEAKING AT"), findsOneWidget);

    await tester.tap(find.text('Editorial'));
    await tester.pump();
    expect(find.text('Hyatt Regency Trivandrum'), findsOneWidget);
  });

  testWidgets('a gallery photo is framed, saved and shared at 1080px', (
    tester,
  ) async {
    final export = FakeExport();
    await tester.pumpWidget(
      app(export, photo: 'assets/images/splash-mark.png'),
    );
    await tester.pump();

    await tapAndWait(tester, find.byTooltip('Choose from gallery'));
    expect(find.text('Retake'), findsOneWidget);
    expect(find.text('Share'), findsOneWidget);

    await tester.tap(find.text('Story'));
    await tester.pump();
    await tapAndWait(tester, find.byTooltip('Save to gallery'));
    expect(export.saved, hasLength(1));
    expect(pngWidth(export.saved.single), frameExportWidth.toInt());
    expect(find.text('Saved to your gallery.'), findsOneWidget);

    await tapAndWait(tester, find.text('Share'));
    expect(export.shared, [
      "I'm attending Bio Connect 4.0, 08-09 October 2026 at Hyatt Regency Trivandrum.",
    ]);

    await tester.tap(find.text('Retake'));
    await tester.pump();
    expect(find.text('Take photo'), findsOneWidget);
  });
}
