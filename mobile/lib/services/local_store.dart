import 'package:flutter_secure_storage/flutter_secure_storage.dart';
import 'package:shared_preferences/shared_preferences.dart';

import 'content_service.dart';

/// Small JSON documents the attendee keeps on this phone: their agenda,
/// saved contacts and feedback. Nothing here leaves the phone on its own.
abstract interface class LocalStore {
  Future<String?> read(String key);
  Future<void> write(String key, String value);
}

/// Staging and production builds must never share saved data.
String scopedKey(String name) =>
    'bioconnect_${name}_${Uri.encodeComponent(CurrentContentService.defaultApiBaseUrl)}';

/// Plain app preferences, for data that is not personal to anyone else.
class PreferencesStore implements LocalStore {
  PreferencesStore([this._instance]);
  SharedPreferencesAsync? _instance;

  // Created on first use: construction throws where no platform is
  // registered, and callers already handle a failed read or write.
  SharedPreferencesAsync get _prefs => _instance ??= SharedPreferencesAsync();
  @override
  Future<String?> read(String key) async => _prefs.getString(scopedKey(key));
  @override
  Future<void> write(String key, String value) async =>
      _prefs.setString(scopedKey(key), value);
}

/// The keychain or keystore, for other people's contact details and notes.
class SecureStore implements LocalStore {
  SecureStore([FlutterSecureStorage? storage])
    : _storage =
          storage ??
          const FlutterSecureStorage(
            iOptions: IOSOptions(
              accessibility: KeychainAccessibility.first_unlock_this_device,
            ),
          );
  final FlutterSecureStorage _storage;
  @override
  Future<String?> read(String key) => _storage.read(key: scopedKey(key));
  @override
  Future<void> write(String key, String value) =>
      _storage.write(key: scopedKey(key), value: value);
}

class MemoryStore implements LocalStore {
  MemoryStore([Map<String, String>? values]) : values = values ?? {};
  final Map<String, String> values;
  @override
  Future<String?> read(String key) async => values[key];
  @override
  Future<void> write(String key, String value) async => values[key] = value;
}
