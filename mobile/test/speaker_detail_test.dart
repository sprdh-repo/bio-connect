import 'package:bio_connect_app/main.dart';
import 'package:bio_connect_app/models/event_content.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  testWidgets('speaker detail shows the whole portrait without a wide crop', (
    tester,
  ) async {
    tester.view.physicalSize = const Size(360, 800);
    tester.view.devicePixelRatio = 1;
    addTearDown(tester.view.resetPhysicalSize);
    addTearDown(tester.view.resetDevicePixelRatio);
    await tester.pumpWidget(
      const MaterialApp(
        home: SpeakerDetailScreen(
          Speaker(
            id: 'raj-shirumalla',
            name: 'Dr. Raj Shirumalla',
            role: 'Mission Director',
            organization: 'BIRAC',
            image: 'assets/images/speakers/raj-shirumalla.webp',
            linkedin: '',
          ),
        ),
      ),
    );
    await tester.pumpAndSettle();
    final image = tester.widget<Image>(find.byType(Image).first);
    expect(image.fit, BoxFit.contain);
    final size = tester.getSize(find.byType(SpeakerImage));
    expect(size.width, lessThanOrEqualTo(240));
    expect(size.height, greaterThan(size.width));
    expect(tester.takeException(), isNull);
  });
}
