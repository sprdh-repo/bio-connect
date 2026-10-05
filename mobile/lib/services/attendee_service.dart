import 'package:dio/dio.dart';

import 'content_service.dart';

class AttendeeException implements Exception {
  const AttendeeException(
    this.message, {
    this.notFound = false,
    this.offline = false,
    this.closed = false,
  });
  final String message;

  /// The server answered that the badge or session does not exist.
  final bool notFound;

  /// The server could not be reached; the request may be retried later.
  final bool offline;

  /// Feedback has been closed by the organisers.
  final bool closed;
  @override
  String toString() => message;
}

/// What a scanned badge reveals: what it prints, plus the email and phone
/// its holder chose to share.
class BadgeProfile {
  const BadgeProfile({
    required this.qrId,
    required this.name,
    this.designation = '',
    this.institution = '',
    this.category = '',
    this.email = '',
    this.phone = '',
  });
  final String qrId, name, designation, institution, category, email, phone;
  factory BadgeProfile.fromJson(Map<String, dynamic> json) => BadgeProfile(
    qrId: json['qr_id'] as String? ?? '',
    name: json['name'] as String? ?? '',
    designation: json['designation'] as String? ?? '',
    institution: json['institution'] as String? ?? '',
    category: json['category'] as String? ?? '',
    email: json['email'] as String? ?? '',
    phone: json['phone'] as String? ?? '',
  );
}

/// Badge lookups and feedback. Neither needs the attendee to sign in.
class AttendeeService {
  AttendeeService({Dio? dio})
    : _dio =
          dio ??
          Dio(
            BaseOptions(
              baseUrl: CurrentContentService.defaultApiBaseUrl,
              connectTimeout: const Duration(seconds: 10),
              receiveTimeout: const Duration(seconds: 10),
            ),
          );
  final Dio _dio;

  Future<BadgeProfile> badge(String qrId) async {
    final json = await _request(
      'GET',
      '/api/v1/public/badges/${Uri.encodeComponent(qrId)}',
    );
    return BadgeProfile.fromJson(json);
  }

  Future<void> sendFeedback({
    required String deviceId,
    required String sessionId,
    required int rating,
    required String comment,
  }) => _request(
    'POST',
    '/api/v1/public/feedback',
    data: {
      'device_id': deviceId,
      'kind': sessionId.isEmpty ? 'event' : 'session',
      'session_id': sessionId,
      'rating': rating,
      'comment': comment.trim(),
    },
  );

  Future<Map<String, dynamic>> _request(
    String method,
    String path, {
    Map<String, dynamic>? data,
  }) async {
    try {
      final response = await _dio.request<Map<String, dynamic>>(
        path,
        data: data,
        options: Options(method: method),
      );
      return response.data ?? const {};
    } on DioException catch (error) {
      final status = error.response?.statusCode;
      final body = error.response?.data;
      if (status == null) {
        throw const AttendeeException(
          'Could not connect. Check your connection and try again.',
          offline: true,
        );
      }
      throw AttendeeException(
        body is Map && body['error'] is String
            ? body['error'] as String
            : 'Something went wrong. Please try again.',
        notFound: status == 404,
        closed: status == 409,
        offline: status >= 500 || status == 429,
      );
    }
  }
}
