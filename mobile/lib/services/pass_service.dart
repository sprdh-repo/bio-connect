import 'dart:convert';

import 'package:dio/dio.dart';
import 'package:flutter_secure_storage/flutter_secure_storage.dart';

import 'content_service.dart';

class AdmissionPass {
  const AdmissionPass({
    required this.id,
    required this.name,
    required this.institution,
    required this.designation,
    required this.category,
    required this.number,
    required this.qrId,
    required this.downloadUrl,
  });
  final String id,
      name,
      institution,
      designation,
      category,
      number,
      qrId,
      downloadUrl;

  factory AdmissionPass.fromJson(Map<String, dynamic> json) => AdmissionPass(
    id: json['id'] as String,
    name: json['name'] as String,
    institution: json['institution'] as String,
    designation: json['designation'] as String,
    category: json['category'] as String,
    number: json['number'] as String,
    qrId: json['qr_id'] as String,
    downloadUrl: json['download_url'] as String,
  );
  Map<String, dynamic> toJson() => {
    'id': id,
    'name': name,
    'institution': institution,
    'designation': designation,
    'category': category,
    'number': number,
    'qr_id': qrId,
    'download_url': downloadUrl,
  };
}

class PassChallenge {
  const PassChallenge(this.token, this.expiresAt, this.resendAfterSeconds);
  final String token;
  final DateTime expiresAt;
  final int resendAfterSeconds;
}

class PassAccess {
  const PassAccess({
    required this.token,
    required this.expiresAt,
    required this.checkedAt,
    required this.passes,
  });
  final String token;
  final DateTime expiresAt, checkedAt;
  final List<AdmissionPass> passes;
  factory PassAccess.fromJson(Map<String, dynamic> json) => PassAccess(
    token: json['token'] as String,
    expiresAt: DateTime.parse(json['expires_at'] as String),
    checkedAt: DateTime.parse(json['checked_at'] as String),
    passes: (json['passes'] as List)
        .map((p) => AdmissionPass.fromJson(p as Map<String, dynamic>))
        .toList(),
  );
  Map<String, dynamic> toJson() => {
    'token': token,
    'expires_at': expiresAt.toIso8601String(),
    'checked_at': checkedAt.toIso8601String(),
    'passes': passes.map((p) => p.toJson()).toList(),
  };
}

class PassException implements Exception {
  const PassException(this.message, {this.unauthorized = false});
  final String message;
  final bool unauthorized;
}

class PassService {
  PassService({Dio? dio})
    : _dio =
          dio ??
          Dio(
            BaseOptions(
              baseUrl: CurrentContentService.defaultApiBaseUrl,
              connectTimeout: const Duration(seconds: 10),
              receiveTimeout: const Duration(seconds: 15),
            ),
          );
  final Dio _dio;

  Future<PassChallenge> requestCode({
    required String channel,
    required String identifier,
    String qrId = '',
    bool whatsappConsent = false,
  }) async {
    final json = await _request(
      'POST',
      '/api/v1/mobile/pass-access/challenges',
      data: {
        'channel': channel,
        'identifier': identifier.trim(),
        'qr_id': qrId,
        'whatsapp_consent': whatsappConsent,
      },
    );
    return PassChallenge(
      json['challenge'] as String,
      DateTime.parse(json['expires_at'] as String),
      json['resend_after_seconds'] as int,
    );
  }

  Future<PassAccess> verify(PassChallenge challenge, String code) async {
    final json = await _request(
      'POST',
      '/api/v1/mobile/pass-access/verify',
      data: {'challenge': challenge.token, 'code': code.trim()},
    );
    final access = PassAccess(
      token: json['token'] as String,
      expiresAt: DateTime.parse(json['expires_at'] as String),
      checkedAt: DateTime.now(),
      passes: const [],
    );
    return access;
  }

  Future<PassAccess> refresh(PassAccess access) async {
    final json = await _request(
      'GET',
      '/api/v1/mobile/passes',
      token: access.token,
    );
    return PassAccess(
      token: access.token,
      expiresAt: access.expiresAt,
      checkedAt: DateTime.parse(json['checked_at'] as String),
      passes: (json['passes'] as List)
          .map((p) => AdmissionPass.fromJson(p as Map<String, dynamic>))
          .toList(),
    );
  }

  Future<void> revoke(PassAccess access) async {
    await _request('DELETE', '/api/v1/mobile/passes', token: access.token);
  }

  Future<Map<String, dynamic>> _request(
    String method,
    String path, {
    Map<String, dynamic>? data,
    String? token,
  }) async {
    try {
      final response = await _dio.request<Map<String, dynamic>>(
        path,
        data: data,
        options: Options(
          method: method,
          headers: token == null ? null : {'Authorization': 'Bearer $token'},
        ),
      );
      if (response.data == null) {
        throw const PassException(
          'The pass service returned an invalid response.',
        );
      }
      return response.data!;
    } on DioException catch (error) {
      final body = error.response?.data;
      throw PassException(
        body is Map && body['error'] is String
            ? body['error'] as String
            : 'Could not connect. Check your connection and try again.',
        unauthorized: error.response?.statusCode == 401,
      );
    }
  }
}

abstract interface class PassStore {
  Future<List<PassAccess>> read();
  Future<void> write(List<PassAccess> access);
  Future<void> clear();
}

class SecurePassStore implements PassStore {
  SecurePassStore({FlutterSecureStorage? storage})
    : _storage =
          storage ??
          const FlutterSecureStorage(
            iOptions: IOSOptions(
              accessibility: KeychainAccessibility.first_unlock_this_device,
            ),
          );
  final FlutterSecureStorage _storage;
  // Staging and production wallets must never share credentials or pass snapshots.
  final _key =
      'bioconnect_passes_v1_${Uri.encodeComponent(CurrentContentService.defaultApiBaseUrl)}';
  @override
  Future<List<PassAccess>> read() async {
    final raw = await _storage.read(key: _key);
    if (raw == null) return [];
    return (jsonDecode(raw) as List)
        .map((p) => PassAccess.fromJson(p as Map<String, dynamic>))
        .toList();
  }

  @override
  Future<void> write(List<PassAccess> access) => _storage.write(
    key: _key,
    value: jsonEncode(access.map((p) => p.toJson()).toList()),
  );
  @override
  Future<void> clear() => _storage.delete(key: _key);
}

/// Accepts the bare pass token (PDF passes, app QR) or a printed badge's
/// profile URL, which ends in the same token (`https://host/p/token`).
String? admissionQr(String raw) {
  final value = raw.trim();
  if (RegExp(r'^[A-Za-z0-9_-]{43}$').hasMatch(value)) return value;
  final uri = Uri.tryParse(value);
  if (uri == null || !uri.isScheme('https') && !uri.isScheme('http')) {
    return null;
  }
  final segments = uri.pathSegments;
  if (segments.length != 2 || segments.first != 'p') return null;
  return RegExp(r'^[A-Za-z0-9_-]{43}$').hasMatch(segments.last)
      ? segments.last
      : null;
}
