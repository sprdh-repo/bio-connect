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

  Future<void> load() async {
    if (busy) return;
    busy = true;
    error = null;
    notifyListeners();
    try {
      _access = await _store.read();
      loaded = true;
    } catch (_) {
      error = 'Saved passes could not be opened. Try again, or remove the saved passes from this phone.';
    }
    busy = false;
    notifyListeners();
    if (loaded) await refresh();
  }

  Future<void> refresh() async {
    if (busy || !loaded) return;
    busy = true;
    offline = false;
    error = null;
    notifyListeners();
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
