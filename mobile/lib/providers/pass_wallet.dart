import 'package:flutter/foundation.dart';

import '../services/pass_service.dart';

class PassWallet extends ChangeNotifier {
  PassWallet({PassService? service, PassStore? store, DateTime Function()? now})
    : service = service ?? PassService(),
      _store = store ?? SecurePassStore(),
      _now = now ?? DateTime.now;
  bool _disposed = false;
  @override
  void dispose() {
    _disposed = true;
    super.dispose();
  }

  @override
  void notifyListeners() {
    if (!_disposed) super.notifyListeners();
  }

  final PassService service;
  final PassStore _store;
  final DateTime Function() _now;
  List<PassAccess> _access = [];
  bool busy = false, loaded = false, offline = false;
  String? error;
  List<AdmissionPass> get passes {
    final byId = <String, AdmissionPass>{};
    for (final access in _access) {
      if (access.expiresAt.isAfter(_now())) {
        for (final pass in access.passes) {
          byId[pass.id] = pass;
        }
      }
    }
    return byId.values.toList();
  }

  List<MomentsCredential> get momentsCredentials => [
    for (final access in _access)
      if (access.expiresAt.isAfter(_now()))
        for (final pass in access.passes)
          MomentsCredential(token: access.token, pass: pass),
  ];

  DateTime? checkedAt(String passId) {
    final times =
        _access
            .where((a) => a.passes.any((p) => p.id == passId))
            .map((a) => a.checkedAt)
            .toList()
          ..sort();
    return times.isEmpty ? null : times.last;
  }

  Future<void>? _syncing;

  /// Reads the passes saved on this phone, then checks them with the server.
  /// Callers that arrive mid-load wait for the same run.
  Future<void> load() {
    if (_syncing case final running?) return running;
    if (busy) return Future.value();
    return _syncing = _sync().whenComplete(() => _syncing = null);
  }

  /// Checks saved passes with the server. Re-reads the saved copy first, so
  /// passes saved since this wallet loaded are never overwritten.
  Future<void> refresh() => loaded ? load() : Future.value();

  Future<void> _sync() async {
    busy = true;
    offline = false;
    error = null;
    notifyListeners();
    try {
      _access = await _store.read();
      loaded = true;
      notifyListeners();
    } catch (_) {
      error = 'Saved passes could not be opened. Try again, or remove the saved passes from this phone.';
      busy = false;
      notifyListeners();
      return;
    }
    final next = <PassAccess>[];
    try {
      for (final access in _access) {
        if (!access.expiresAt.isAfter(_now())) continue;
        try {
          next.add(await service.refresh(access));
        } on PassException catch (e) {
          if (!e.unauthorized) {
            next.add(access);
            offline = true;
          }
        } catch (_) {
          next.add(access);
          offline = true;
        }
      }
      _access = next;
      await _store.write(_access);
      if (offline) error = 'Could not check the latest status. Showing passes saved on this phone.';
    } catch (_) {
      error = 'Passes were refreshed, but could not be saved on this phone. Try again.';
    }
    busy = false;
    notifyListeners();
  }

  Future<void> add(PassAccess access) async {
    if (!loaded || busy) {
      throw const PassException('Wait for saved passes to finish loading.');
    }
    busy = true;
    notifyListeners();
    final next = [..._access.where((a) => a.token != access.token), access];
    // Remove overlapping cached passes. A later refresh must not resurrect an old copy.
    final ids = access.passes.map((p) => p.id).toSet();
    final cleaned = next
        .map(
          (a) => a.token == access.token
              ? a
              : PassAccess(
                  token: a.token,
                  expiresAt: a.expiresAt,
                  checkedAt: a.checkedAt,
                  passes: a.passes.where((p) => !ids.contains(p.id)).toList(),
                ),
        )
        .toList();
    try {
      await _store.write(cleaned);
      _access = cleaned;
      offline = false;
      error = null;
    } catch (_) {
      throw const PassException(
        'Your pass could not be saved securely. Try again.',
      );
    } finally {
      busy = false;
      notifyListeners();
    }
  }

  /// Records whether people who scan [pass]'s badge get its holder's email
  /// or phone. Throws [PassException] when the choice could not be saved.
  Future<void> setSharing(
    AdmissionPass pass, {
    required bool email,
    required bool phone,
  }) async {
    final access = _access.where(
      (a) =>
          a.expiresAt.isAfter(_now()) && a.passes.any((p) => p.id == pass.id),
    );
    if (access.isEmpty) {
      throw const PassException('Add this pass again to change sharing.');
    }
    await service.setSharing(access.last, pass.id, email: email, phone: phone);
    _access = [
      for (final a in _access)
        PassAccess(
          token: a.token,
          expiresAt: a.expiresAt,
          checkedAt: a.checkedAt,
          passes: [
            for (final p in a.passes)
              p.id == pass.id ? p.withSharing(email: email, phone: phone) : p,
          ],
        ),
    ];
    notifyListeners();
    try {
      await _store.write(_access);
    } catch (_) {
      // The server holds the choice; the next refresh restores the copy.
    }
  }

  Future<void> forget() async {
    if (busy) return;
    busy = true;
    notifyListeners();
    try {
      await _store.clear();
      final old = _access;
      _access = [];
      loaded = true;
      error = null;
      offline = false;
      for (final access in old) {
        try {
          await service.revoke(access);
        } catch (_) {
          /* Local copies are already removed. */
        }
      }
    } catch (_) {
      error = 'Saved passes could not be removed. Try again.';
    }
    busy = false;
    notifyListeners();
  }
}
