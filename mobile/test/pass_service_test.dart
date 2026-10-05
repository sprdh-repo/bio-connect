import 'dart:convert';
import 'dart:io';

import 'package:bio_connect_app/services/pass_service.dart';
import 'package:dio/dio.dart';
import 'package:flutter_test/flutter_test.dart';

import 'pass_wallet_test.dart' show pass;

void main() {
  test('pass API contract sends scan and consent, verifies OTP, and authenticates retrieval', () async {
    final server = await HttpServer.bind(InternetAddress.loopbackIPv4, 0);
    addTearDown(server.close);
    final received = <String, Map<String, dynamic>>{};
    final methods = <String>[];
    var rejected = false;
    server.listen((request) async {
      final path = request.uri.path;
      methods.add(request.method);
      request.response.headers.contentType = ContentType.json;
      if (path.endsWith('/challenges')) {
        received[path] = jsonDecode(await utf8.decoder.bind(request).join());
        request.response.statusCode = 202;
        request.response.write(
          jsonEncode({
            'challenge': 'challenge',
            'expires_at': '2026-10-05T12:10:00Z',
            'resend_after_seconds': 60,
          }),
        );
      } else if (path.endsWith('/verify')) {
        received[path] = jsonDecode(await utf8.decoder.bind(request).join());
        request.response.write(
          jsonEncode({
            'token': 'verified-session',
            'expires_at': '2026-11-04T12:00:00Z',
          }),
        );
      } else {
        expect(
          request.headers.value('Authorization'),
          'Bearer verified-session',
        );
        if (rejected) {
          request.response.statusCode = 401;
          request.response.write(jsonEncode({'error': 'verify again'}));
        } else {
          request.response.write(
            jsonEncode({
              'passes': [pass.toJson()],
              'checked_at': '2026-10-05T12:00:00Z',
            }),
          );
        }
      }
      await request.response.close();
    });
    final service = PassService(
      dio: Dio(BaseOptions(baseUrl: 'http://127.0.0.1:${server.port}')),
    );
    final challenge = await service.requestCode(
      channel: 'whatsapp',
      identifier: '+919876543210',
      qrId: pass.qrId,
      whatsappConsent: true,
    );
    final access = await service.verify(challenge, '123456');
    expect(access.passes, isEmpty);
    expect(received['/api/v1/mobile/pass-access/challenges'], {
      'channel': 'whatsapp',
      'identifier': '+919876543210',
      'qr_id': pass.qrId,
      'whatsapp_consent': true,
    });
    expect(received['/api/v1/mobile/pass-access/verify'], {
      'challenge': 'challenge',
      'code': '123456',
    });
    final updated = await service.refresh(access);
    expect(updated.passes.single.number, 'BC-1');
    await service.revoke(updated);
    expect(methods, ['POST', 'POST', 'GET', 'DELETE']);
    rejected = true;
    await expectLater(
      service.refresh(updated),
      throwsA(
        isA<PassException>().having(
          (e) => e.unauthorized,
          'unauthorized',
          isTrue,
        ),
      ),
    );
  });
}
