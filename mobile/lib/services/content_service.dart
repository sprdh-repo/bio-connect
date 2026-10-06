import 'dart:convert';

import 'package:dio/dio.dart';
import 'package:flutter/services.dart';

import '../models/event_content.dart';

/// A replaceable source for event content and the exhibitor directory.
abstract interface class ContentService {
  /// The bundled offline snapshot.
  Future<EventContent> loadEvent();

  /// Live content. Throws when the backend cannot be reached.
  Future<EventContent> fetchEvent(EventContent current);

  /// [fetchEvent] that falls back to [current].
  Future<EventContent> refreshEvent(EventContent current);
  Future<List<Exhibitor>> loadExhibitors();
}

class CurrentContentService implements ContentService {
  CurrentContentService({
    Dio? dio,
    this.remoteContent = true,
    this.apiBaseUrl = defaultApiBaseUrl,
  }) : _dio =
           dio ??
           Dio(
             BaseOptions(
               connectTimeout: const Duration(seconds: 10),
               receiveTimeout: const Duration(seconds: 10),
             ),
           );
  final Dio _dio;
  final bool remoteContent;
  final String apiBaseUrl;
  static const defaultApiBaseUrl = String.fromEnvironment(
    'BIO_CONNECT_API_BASE_URL',
    defaultValue: 'https://reg.bioconnect.kerala.gov.in',
  );
  // Breaks are only sent to app versions that show them as breaks.
  String get appContentUrl =>
      '$apiBaseUrl/api/v1/public/app-content?include=breaks';
  String get exhibitorsUrl => '$apiBaseUrl/api/v1/public/exhibitors';

  @override
  Future<EventContent> loadEvent() async {
    // Read and decode directly: loadString hands assets over 50 KB to a
    // background isolate, which is slower to start than this small decode.
    final bytes = await rootBundle.load('assets/content/event.json');
    final source = utf8.decode(bytes.buffer.asUint8List());
    return EventContent.fromJson(jsonDecode(source) as Map<String, dynamic>);
  }

  @override
  Future<EventContent> fetchEvent(EventContent current) async {
    if (!remoteContent) return current;
    final response = await _dio
        .get<Map<String, dynamic>>(appContentUrl)
        .timeout(const Duration(seconds: 6));
    final data = response.data;
    if (data == null) throw const FormatException('Invalid app content');
    return EventContent.fromJson(data);
  }

  @override
  Future<EventContent> refreshEvent(EventContent current) async {
    try {
      return await fetchEvent(current);
    } catch (_) {
      // The verified bundled snapshot keeps the event guide usable offline.
      return current;
    }
  }

  @override
  Future<List<Exhibitor>> loadExhibitors() async {
    final response = await _dio.get<Map<String, dynamic>>(exhibitorsUrl);
    final items = response.data?['exhibitors'];
    if (items is! List) {
      throw const FormatException('Invalid exhibitor directory');
    }
    return items.map((item) {
      final exhibitor = Exhibitor.fromJson(item as Map<String, dynamic>);
      if (exhibitor.logoUrl.isEmpty) return exhibitor;
      final parsed = Uri.parse(exhibitor.logoUrl);
      return parsed.hasScheme
          ? exhibitor
          : exhibitor.withLogoUrl(
              Uri.parse(apiBaseUrl).resolveUri(parsed).toString(),
            );
    }).toList();
  }
}
