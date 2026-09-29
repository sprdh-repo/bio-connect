import 'dart:convert';
import 'dart:io';

import 'package:bio_connect_app/services/registration_service.dart';
import 'package:dio/dio.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  test(
    'loads open delegate categories and submits the backend contract',
    () async {
      final server = await HttpServer.bind(InternetAddress.loopbackIPv4, 0);
      addTearDown(server.close);
      Map<String, dynamic>? submitted;
      String? idempotencyKey;
      server.listen((request) async {
        request.response.headers.contentType = ContentType.json;
        if (request.method == 'GET') {
          request.response.write(
            jsonEncode({
              'categories': [
                {
                  'id': 'industry',
                  'kind': 'delegate',
                  'label': 'Industry',
                  'early_paise': 600000,
                  'regular_paise': 700000,
                  'payable_paise': 600000,
                  'roster_count': 1,
                  'open': true,
                  'free_only': false,
                },
                {
                  'id': 'student',
                  'kind': 'delegate',
                  'label': 'Students',
                  'early_paise': 100000,
                  'regular_paise': 150000,
                  'payable_paise': 100000,
                  'roster_count': 1,
                  'open': false,
                  'free_only': false,
                },
                {
                  'id': 'table',
                  'kind': 'exhibitor',
                  'label': 'Table space',
                  'early_paise': 2000000,
                  'regular_paise': 2000000,
                  'payable_paise': 2000000,
                  'roster_count': 2,
                  'open': true,
                  'free_only': false,
                },
                {
                  'id': 'official',
                  'kind': 'delegate',
                  'label': 'Govt. Official',
                  'early_paise': 1,
                  'regular_paise': 1,
                  'payable_paise': 1,
                  'roster_count': 1,
                  'open': true,
                  'free_only': true,
                },
              ],
              'registration_enabled': true,
              'cutoff': '2026-10-01T00:00:00+05:30',
            }),
          );
        } else {
          idempotencyKey = request.headers.value('Idempotency-Key');
          submitted = jsonDecode(await utf8.decoder.bind(request).join());
          request.response.statusCode = HttpStatus.created;
          request.response.write(
            jsonEncode({
              'id': 'registration-id',
              'management_token': 'token',
              'manage_url': 'https://reg.example/manage/registration-id#token',
            }),
          );
        }
        await request.response.close();
      });
      final service = RegistrationService(
        dio: Dio(BaseOptions(baseUrl: 'http://127.0.0.1:${server.port}')),
      );

      final categories = await service.delegateCategories();
      expect(categories.map((item) => item.id), ['industry']);
      final options = await service.options();
      expect(options.categories.map((item) => item.id), [
        'industry',
        'student',
        'table',
      ]);

      final result = await service.registerDelegate(
        categoryID: 'industry',
        institution: 'Bio Labs',
        name: 'Asha Nair',
        designation: 'Scientist',
        email: 'asha@example.com',
        phone: '+919876543210',
        whatsappConsent: true,
      );
      expect(
        result.referenceUrl,
        'https://reg.example/manage/registration-id#token',
      );
      expect(idempotencyKey, hasLength(48));
      expect(submitted?['category_id'], 'industry');
      expect(submitted?['attendees'], [
        {
          'name': 'Asha Nair',
          'email': 'asha@example.com',
          'phone': '+919876543210',
          'designation': 'Scientist',
          'whatsapp_consent': true,
        },
      ]);
    },
  );
}
