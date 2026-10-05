import 'dart:async';

import 'package:bio_connect_app/providers/pass_wallet.dart';
import 'package:bio_connect_app/services/pass_service.dart';
import 'package:flutter_test/flutter_test.dart';

const pass = AdmissionPass(
  id: 'p1',
  name: 'Asha Nair',
  institution: 'Bio Labs',
  designation: 'Scientist',
  category: 'Faculty',
  number: 'BC-1',
  qrId: 'abcdefghijklmnopqrstuvwxyz0123456789ABCDEFG',
  downloadUrl: 'https://reg.example/passes/private',
);
final now = DateTime.utc(2026, 10, 5);
PassAccess access({
  DateTime? expiresAt,
  List<AdmissionPass> passes = const [pass],
}) => PassAccess(
  token: 'session',
  expiresAt: expiresAt ?? now.add(const Duration(days: 30)),
  checkedAt: now,
  passes: passes,
);

class MemoryPassStore implements PassStore {
  MemoryPassStore(this.data);
  List<PassAccess> data;
  @override
  Future<List<PassAccess>> read() async => data;
  @override
  Future<void> write(List<PassAccess> access) async {
    data = access;
  }

  @override
  Future<void> clear() async {
    data = [];
  }
}

class FakePassService extends PassService {
  Object? failure;
  List<AdmissionPass> current = [pass];
  @override
  Future<PassAccess> refresh(PassAccess access) async {
    if (failure != null) throw failure!;
    return PassAccess(
      token: access.token,
      expiresAt: access.expiresAt,
      checkedAt: now.add(const Duration(minutes: 1)),
      passes: current,
    );
  }

  @override
  Future<void> revoke(PassAccess access) async {}
}

class DelayedPassStore extends MemoryPassStore {
  DelayedPassStore() : super([]);
  bool delayNext = false;
  final started = Completer<void>();
  final release = Completer<void>();
  @override
  Future<void> write(List<PassAccess> access) async {
    if (delayNext) {
      delayNext = false;
      started.complete();
      await release.future;
    }
    await super.write(access);
  }
}

void main() {
  sharingTests();
  test(
    'adding a pass serializes secure storage against foreground refresh',
    () async {
      final store = DelayedPassStore();
      final wallet = PassWallet(
        service: FakePassService(),
        store: store,
        now: () => now,
      );
      await wallet.load();
      store.delayNext = true;
      final saving = wallet.add(access());
      await store.started.future;
      try {
        expect(wallet.busy, isTrue);
        await wallet.refresh();
      } finally {
        store.release.complete();
        await saving;
      }
      expect(store.data.single.passes.single.id, 'p1');
    },
  );
  test(
    'saved passes remain available offline; online revocation removes the QR',
    () async {
      final store = MemoryPassStore([access()]);
      final service = FakePassService()
        ..failure = const PassException('No connection');
      final wallet = PassWallet(service: service, store: store, now: () => now);
      await wallet.load();
      expect(wallet.passes.single.name, 'Asha Nair');
      expect(wallet.offline, isTrue);
      service.failure = null;
      service.current = [];
      await wallet.refresh();
      expect(wallet.passes, isEmpty);
      expect(store.data.single.passes, isEmpty);
      expect(wallet.offline, isFalse);
    },
  );
  test(
    'expired and rejected sessions never display cached admission codes',
    () async {
      final store = MemoryPassStore([
        access(expiresAt: now.subtract(const Duration(seconds: 1))),
        access(),
      ]);
      final service = FakePassService()
        ..failure = const PassException('Expired session', unauthorized: true);
      final wallet = PassWallet(service: service, store: store, now: () => now);
      await wallet.load();
      expect(wallet.passes, isEmpty);
      expect(store.data, isEmpty);
      expect(wallet.offline, isFalse);
    },
  );
  test('forget removes local passes even without connectivity', () async {
    final store = MemoryPassStore([access()]);
    final wallet = PassWallet(
      service: FakePassService(),
      store: store,
      now: () => now,
    );
    await wallet.load();
    await wallet.forget();
    expect(store.data, isEmpty);
    expect(wallet.passes, isEmpty);
  });
  test(
    'Moments credentials pair each active pass with its session token',
    () async {
      final expired = access(
        expiresAt: now.subtract(const Duration(seconds: 1)),
        passes: const [
          AdmissionPass(
            id: 'old',
            name: 'Old Pass',
            institution: '',
            designation: '',
            category: '',
            number: '',
            qrId: '',
            downloadUrl: '',
          ),
        ],
      );
      final wallet = PassWallet(
        service: FakePassService()..failure = const PassException('offline'),
        store: MemoryPassStore([expired, access()]),
        now: () => now,
      );
      await wallet.load();
      expect(wallet.momentsCredentials, hasLength(1));
      expect(wallet.momentsCredentials.single.token, 'session');
      expect(wallet.momentsCredentials.single.pass.id, 'p1');
    },
  );
  test('scanner accepts admission identifiers and badge profile URLs, never arbitrary URLs', () {
    expect(admissionQr(pass.qrId), pass.qrId);
    expect(admissionQr('https://example.com/passes/token'), isNull);
    expect(
      admissionQr(' https://reg.bioconnect.kerala.gov.in/p/${pass.qrId} '),
      pass.qrId,
    );
    expect(
      admissionQr('https://reg.bioconnect.kerala.gov.in/x/${pass.qrId}'),
      isNull,
    );
    expect(admissionQr('https://reg.bioconnect.kerala.gov.in/p/short'), isNull);
    expect(admissionQr('BC-1'), isNull);
  });
}

class SharingPassService extends FakePassService {
  final calls = <(String, String, bool, bool)>[];
  @override
  Future<void> setSharing(
    PassAccess access,
    String passId, {
    required bool email,
    required bool phone,
  }) async {
    if (failure != null) throw failure!;
    calls.add((access.token, passId, email, phone));
  }
}

void sharingTests() {
  test(
    'contact sharing is saved for the pass and kept on this phone',
    () async {
      final store = MemoryPassStore([access()]);
      final service = SharingPassService();
      final wallet = PassWallet(service: service, store: store, now: () => now);
      await wallet.load();
      await wallet.setSharing(pass, email: true, phone: false);
      expect(service.calls.single, ('session', 'p1', true, false));
      expect(wallet.passes.single.shareEmail, isTrue);
      expect(store.data.single.passes.single.shareEmail, isTrue);
      expect(
        AdmissionPass.fromJson(store.data.single.passes.single.toJson())
            .shareEmail,
        isTrue,
      );

      service.failure = const PassException('offline');
      await expectLater(
        wallet.setSharing(pass, email: false, phone: true),
        throwsA(isA<PassException>()),
      );
      expect(
        wallet.passes.single.shareEmail,
        isTrue,
        reason: 'unchanged on failure',
      );
    },
  );
}
