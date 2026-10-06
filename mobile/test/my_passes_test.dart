import 'package:bio_connect_app/main.dart';
import 'package:bio_connect_app/providers/pass_wallet.dart';
import 'package:bio_connect_app/screens/my_passes_screen.dart';
import 'package:bio_connect_app/services/pass_service.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:qr_flutter/qr_flutter.dart';

import 'pass_wallet_test.dart' show MemoryPassStore, access, pass;

class JourneyService extends PassService {
  String? requestedChannel, requestedIdentifier;
  bool? consent;
  int verifications = 0;
  bool failLoadOnce = false;
  @override
  Future<PassChallenge> requestCode({
    required String channel,
    required String identifier,
    String qrId = '',
    bool whatsappConsent = false,
  }) async {
    requestedChannel = channel;
    requestedIdentifier = identifier;
    consent = whatsappConsent;
    return PassChallenge(
      'challenge',
      DateTime.now().add(const Duration(minutes: 10)),
      60,
    );
  }

  @override
  Future<PassAccess> verify(PassChallenge challenge, String code) async {
    verifications++;
    if (code != '123456') throw const PassException('Code is incorrect');
    return access(
      expiresAt: DateTime.now().add(const Duration(days: 30)),
      passes: [],
    );
  }

  @override
  Future<PassAccess> refresh(PassAccess a) async {
    if (failLoadOnce) {
      failLoadOnce = false;
      throw const PassException('No connection');
    }
    return PassAccess(
      token: a.token,
      expiresAt: a.expiresAt,
      checkedAt: DateTime.now(),
      passes: [pass],
    );
  }
}

void main() {
  testWidgets(
    'email OTP journey saves and displays the admission QR; retry does not replay OTP',
    (tester) async {
      final service = JourneyService();
      final store = MemoryPassStore([]);
      final wallet = PassWallet(service: service, store: store);
      await tester.pumpWidget(
        MaterialApp(
          theme: ThemeData(
            colorScheme: ColorScheme.fromSeed(seedColor: forest),
          ),
          home: MyPassesScreen(wallet: wallet),
        ),
      );
      await tester.pumpAndSettle();
      // Nothing saved yet: the pitch leads and there is nothing to remove.
      expect(find.text('Your pass.\nOn your phone.'), findsOneWidget);
      expect(find.byTooltip('Remove saved passes'), findsNothing);
      await tester.tap(find.text('Add a pass'));
      await tester.pumpAndSettle();
      await tester.enterText(find.byType(TextFormField), 'asha@example.com');
      await tester.ensureVisible(find.text('Send verification code'));
      await tester.tap(find.text('Send verification code'));
      await tester.pumpAndSettle();
      expect(service.requestedChannel, 'email');
      await tester.enterText(
        find.widgetWithText(TextField, 'Six-digit code'),
        '123456',
      );
      service.failLoadOnce = true;
      await tester.ensureVisible(find.text('Verify and add pass'));
      await tester.tap(find.text('Verify and add pass'));
      await tester.pumpAndSettle();
      expect(find.text('No connection'), findsOneWidget);
      await tester.tap(find.text('Load my pass'));
      await tester.pumpAndSettle();
      expect(service.verifications, 1);
      expect(find.text('Asha Nair'), findsOneWidget);
      expect(find.byType(QrImageView), findsOneWidget);
      expect(store.data.single.passes.single.number, 'BC-1');
      // A saved pass leads the screen; adding another moves below it.
      expect(find.text('Your pass.\nOn your phone.'), findsNothing);
      expect(find.text('Add a pass'), findsNothing);
      await tester.scrollUntilVisible(
        find.text('Add another pass'),
        300,
        scrollable: find
            .descendant(
              of: find.byType(ListView),
              matching: find.byType(Scrollable),
            )
            .first,
      );
      expect(find.text('Add another pass'), findsOneWidget);
      expect(find.byTooltip('Remove saved passes'), findsOneWidget);
      await tester.pumpWidget(const SizedBox.shrink());
      wallet.dispose();
    },
  );
  testWidgets('WhatsApp verification requires explicit consent', (
    tester,
  ) async {
    final service = JourneyService();
    final wallet = PassWallet(service: service, store: MemoryPassStore([]));
    await wallet.load();
    await tester.pumpWidget(MaterialApp(home: AddPassScreen(wallet: wallet)));
    await tester.tap(find.text('WhatsApp'));
    await tester.pumpAndSettle();
    await tester.enterText(find.byType(TextFormField), '+91 98765 43210');
    expect(
      tester
          .widget<FilledButton>(
            find.widgetWithText(FilledButton, 'Send verification code'),
          )
          .onPressed,
      isNull,
    );
    await tester.tap(find.byType(CheckboxListTile));
    await tester.pump();
    await tester.ensureVisible(find.text('Send verification code'));
    await tester.tap(find.text('Send verification code'));
    await tester.pumpAndSettle();
    expect(service.requestedChannel, 'whatsapp');
    expect(service.consent, isTrue);
    await tester.pumpWidget(const SizedBox.shrink());
    wallet.dispose();
  });
}
