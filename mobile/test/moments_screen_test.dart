import 'package:bio_connect_app/providers/pass_wallet.dart';
import 'package:bio_connect_app/screens/moments_screen.dart';
import 'package:bio_connect_app/services/moments_service.dart';
import 'package:bio_connect_app/services/pass_service.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:provider/provider.dart';

const attendeePass = AdmissionPass(
  id: 'pass-1',
  name: 'Asha Nair',
  institution: 'Bio Labs',
  designation: 'Scientist',
  category: 'Faculty',
  number: 'BC4-FC-1',
  qrId: '',
  downloadUrl: '',
);

class _Store implements PassStore {
  _Store(this.value);
  List<PassAccess> value;
  @override
  Future<List<PassAccess>> read() async => value;
  @override
  Future<void> write(List<PassAccess> access) async => value = access;
  @override
  Future<void> clear() async => value = [];
}

class _PassService extends PassService {
  @override
  Future<PassAccess> refresh(PassAccess access) async => access;
}

class _MomentsService extends MomentsService {
  _MomentsService(this.current);
  MomentsStatus current;

  @override
  Future<MomentsStatus> status(MomentsCredential credential) async => current;

  @override
  Future<MomentsAlbum> photos(MomentsCredential credential) async =>
      const MomentsAlbum(
        photos: [
          MomentPhoto(
            filename: 'photo.jpg',
            fileUrl: 'https://photos.test/photo.jpg',
            thumbnailUrl: 'https://photos.test/thumb.jpg',
          ),
        ],
      );
}

PassWallet wallet(List<PassAccess> access) => PassWallet(
  service: _PassService(),
  store: _Store(access),
  now: () => DateTime.utc(2026, 10, 5),
);

PassAccess savedAccess() => PassAccess(
  token: 'saved-token',
  expiresAt: DateTime.utc(2026, 11, 5),
  checkedAt: DateTime.utc(2026, 10, 5),
  passes: const [attendeePass],
);

void main() {
  testWidgets('debug builds show a usable Moments preview without a pass', (
    tester,
  ) async {
    final passWallet = wallet([]);
    addTearDown(passWallet.dispose);
    await tester.pumpWidget(
      MaterialApp(
        home: MomentsScreen(
          wallet: passWallet,
          service: _MomentsService(
            const MomentsStatus(
              albumReady: true,
              selfieUploaded: false,
              similarPhotosCount: 0,
              photosReady: false,
              processingStatus: 'pending',
            ),
          ),
        ),
      ),
    );
    await tester.pumpAndSettle();
    expect(find.text('DEBUG PREVIEW'), findsOneWidget);
    expect(find.text('Take a selfie'), findsWidgets);
    expect(find.byType(Image), findsWidgets);
  });

  testWidgets('ready state opens the matched photo album', (tester) async {
    final passWallet = wallet([savedAccess()]);
    addTearDown(passWallet.dispose);
    await tester.pumpWidget(
      MaterialApp(
        home: MomentsScreen(
          wallet: passWallet,
          service: _MomentsService(
            const MomentsStatus(
              albumReady: true,
              selfieUploaded: true,
              similarPhotosCount: 1,
              photosReady: true,
              processingStatus: 'completed',
            ),
          ),
        ),
      ),
    );
    await tester.pumpAndSettle();
    expect(find.text('Your photos are ready'), findsOneWidget);
    final viewPhotos = find.text('View my photos');
    await tester.drag(find.byType(ListView), const Offset(0, -250));
    await tester.pumpAndSettle();
    await tester.tap(viewPhotos);
    await tester.pump();
    await tester.pump(const Duration(milliseconds: 500));
    expect(find.byType(MomentsAlbumScreen), findsOneWidget);
    expect(find.text('1 photo'), findsOneWidget);
  });

  testWidgets('uses the app-wide wallet so passes added elsewhere count', (
    tester,
  ) async {
    final passWallet = wallet([]);
    addTearDown(passWallet.dispose);
    await passWallet.load();
    await passWallet.add(savedAccess());
    await tester.pumpWidget(
      ChangeNotifierProvider.value(
        value: passWallet,
        child: MaterialApp(
          home: MomentsScreen(
            service: _MomentsService(
              const MomentsStatus(
                albumReady: true,
                selfieUploaded: true,
                similarPhotosCount: 1,
                photosReady: true,
                processingStatus: 'completed',
              ),
            ),
          ),
        ),
      ),
    );
    await tester.pumpAndSettle();
    expect(find.text('DEBUG PREVIEW'), findsNothing);
    expect(find.text('Your photos are ready'), findsOneWidget);
  });
}
