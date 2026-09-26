import 'dart:convert';

import 'package:dio/dio.dart';
import 'package:flutter/services.dart';

import '../models/event_content.dart';

/// A replaceable source for event content and the exhibitor directory.
abstract interface class ContentService {
  Future<EventContent> loadEvent();
  Future<List<Exhibitor>> loadExhibitors();
}

class CurrentContentService implements ContentService {
  CurrentContentService({Dio? dio})
    : _dio =
          dio ??
          Dio(
            BaseOptions(
              connectTimeout: const Duration(seconds: 10),
              receiveTimeout: const Duration(seconds: 10),
            ),
          );
  final Dio _dio;
  static const exhibitorsUrl =
      'https://reg.bioconnect.kerala.gov.in/api/v1/public/exhibitors';

  @override
  Future<EventContent> loadEvent() async {
    final source = await rootBundle.loadString('assets/content/event.json');
    return EventContent.fromJson(jsonDecode(source) as Map<String, dynamic>);
  }

  @override
  Future<List<Exhibitor>> loadExhibitors() async {
    final response = await _dio.get<Map<String, dynamic>>(exhibitorsUrl);
    final items = response.data?['exhibitors'];
    if (items is! List) {
      throw const FormatException('Invalid exhibitor directory');
    }
    return items
        .map((item) => Exhibitor.fromJson(item as Map<String, dynamic>))
        .toList();
  }
}

/// Future backend adapter. It expects the same JSON shape as event.json.
class ApiContentService extends CurrentContentService {
  ApiContentService({required String baseUrl, Dio? dio})
    : _api = dio ?? Dio(BaseOptions(baseUrl: baseUrl));
  final Dio _api;

  @override
  Future<EventContent> loadEvent() async {
    final response = await _api.get<Map<String, dynamic>>(
      '/api/v1/public/app-content',
    );
    return EventContent.fromJson(response.data!);
  }
}
