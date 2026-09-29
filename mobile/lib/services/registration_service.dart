import 'dart:math';

import 'package:dio/dio.dart';

class RegistrationCategory {
  const RegistrationCategory({
    required this.id,
    required this.kind,
    required this.label,
    required this.earlyPaise,
    required this.regularPaise,
    required this.payablePaise,
    required this.rosterCount,
    required this.open,
    required this.freeOnly,
  });

  final String id, kind, label;
  final int earlyPaise, regularPaise, payablePaise, rosterCount;
  final bool open, freeOnly;

  factory RegistrationCategory.fromJson(Map<String, dynamic> json) =>
      RegistrationCategory(
        id: json['id'] as String,
        kind: json['kind'] as String,
        label: json['label'] as String,
        earlyPaise: json['early_paise'] as int,
        regularPaise: json['regular_paise'] as int,
        payablePaise: json['payable_paise'] as int,
        rosterCount: json['roster_count'] as int,
        open: json['open'] as bool,
        freeOnly: (json['free_only'] ?? false) as bool,
      );
}

class RegistrationOptions {
  const RegistrationOptions({
    required this.enabled,
    required this.categories,
    required this.cutoff,
  });
  final bool enabled;
  final List<RegistrationCategory> categories;
  final DateTime cutoff;
}

class RegistrationResult {
  const RegistrationResult({required this.referenceUrl});
  final String referenceUrl;
}

class RegistrationService {
  RegistrationService({Dio? dio})
    : _dio = dio ?? Dio(BaseOptions(baseUrl: apiBaseUrl)),
      _submissionKey = _newIdempotencyKey();

  static const apiBaseUrl = String.fromEnvironment(
    'BIO_CONNECT_API_BASE_URL',
    defaultValue: 'https://reg.bioconnect.kerala.gov.in',
  );
  final Dio _dio;
  final String _submissionKey;

  Future<RegistrationOptions> options() async {
    final response = await _dio.get<Map<String, dynamic>>('/api/v1/categories');
    final data = response.data;
    if (data == null) {
      throw const FormatException('Invalid registration options');
    }
    final items = data['categories'] as List? ?? const [];
    return RegistrationOptions(
      enabled: (data['registration_enabled'] ?? false) as bool,
      categories: items
          .map(
            (item) =>
                RegistrationCategory.fromJson(item as Map<String, dynamic>),
          )
          .where((category) => !category.freeOnly)
          .toList(),
      cutoff: DateTime.parse(data['cutoff'] as String),
    );
  }

  Future<List<RegistrationCategory>> delegateCategories() async {
    final result = await options();
    if (!result.enabled) return const [];
    return result.categories
        .where((category) => category.kind == 'delegate' && category.open)
        .toList();
  }

  Future<RegistrationResult> registerDelegate({
    required String categoryID,
    required String institution,
    required String name,
    required String designation,
    required String email,
    required String phone,
    required bool whatsappConsent,
  }) async {
    try {
      final response = await _dio.post<Map<String, dynamic>>(
        '/api/v1/registrations',
        data: {
          'category_id': categoryID,
          'institution': institution,
          'contact_name': name,
          'email': email,
          'phone': phone,
          'description': '',
          'coupon_code': '',
          'free_token': '',
          'attendees': [
            {
              'name': name,
              'email': email,
              'phone': phone,
              'designation': designation,
              'whatsapp_consent': whatsappConsent,
            },
          ],
        },
        options: Options(headers: {'Idempotency-Key': _submissionKey}),
      );
      final url = response.data?['manage_url'] as String?;
      if (url == null || url.isEmpty) {
        throw const RegistrationException(
          'This registration was already saved. Use the private link sent to your email to continue.',
        );
      }
      return RegistrationResult(referenceUrl: url);
    } on DioException catch (error) {
      final data = error.response?.data;
      if (data is Map && data['error'] is String) {
        throw RegistrationException(data['error'] as String);
      }
      throw const RegistrationException(
        'Registration could not be submitted. Check your connection and retry.',
      );
    }
  }

  static String _newIdempotencyKey() {
    final random = Random.secure();
    return List.generate(
      48,
      (_) => random.nextInt(16).toRadixString(16),
    ).join();
  }
}

class RegistrationException implements Exception {
  const RegistrationException(this.message);
  final String message;
  @override
  String toString() => message;
}
