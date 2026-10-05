import 'dart:io';

import 'package:bio_connect_app/services/moments_service.dart';
import 'package:bio_connect_app/services/pass_service.dart';
import 'package:flutter_test/flutter_test.dart';

const credential = MomentsCredential(
  token: 'saved-session',
  pass: AdmissionPass(
    id: 'pass-42',
    name: 'Asha Nair',
    institution: 'Bio Labs',
    designation: 'Scientist',
    category: 'Faculty',
    number: 'BC-1',
    qrId: '',
    downloadUrl: '',
  ),
);

void main() {
  test('uses the verified pass for the complete Moments API flow', () async {
    final requests = <String>[];
    final server = await HttpServer.bind(InternetAddress.loopbackIPv4, 0);
    addTearDown(server.close);
    server.listen((request) async {
      expect(request.headers.value('authorization'), 'Bearer saved-session');
      expect(request.uri.queryParameters['pass_id'], 'pass-42');
      requests.add('${request.method} ${request.uri.path}');
      request.response.headers.contentType = ContentType.json;
      if (request.uri.path.endsWith('/status')) {
        request.response.write(
          '{"album_ready":true,"selfie_uploaded":true,"similar_photos_count":1,"similar_photos_ready":true,"processing_status":"completed"}',
        );
      } else if (request.uri.path.endsWith('/photos')) {
        request.response.write(
          '{"status":"completed","results":[{"filename":"one.jpg","file_url":"https://photos.test/one.jpg","thumbnail_url":"https://photos.test/thumb.jpg"}]}',
        );
      } else if (request.method == 'POST') {
        final body = await request.fold<List<int>>(
          [],
          (all, part) => all..addAll(part),
        );
        expect(String.fromCharCodes(body), contains('jpeg-data'));
        request.response.statusCode = 201;
        request.response.write('{"status":"processing"}');
      } else {
        request.response.statusCode = 204;
      }
      await request.response.close();
    });
    final service = MomentsService(
      apiBaseUrl: 'http://127.0.0.1:${server.port}',
    );
    final status = await service.status(credential);
    expect(status.similarPhotosCount, 1);
    expect(status.photosReady, isTrue);
    final album = await service.photos(credential);
    expect(album.photos.single.thumbnailUrl, contains('thumb.jpg'));
    final selfie = File('${Directory.systemTemp.path}/bio-moments-selfie.jpg');
    await selfie.writeAsString('jpeg-data');
    addTearDown(() => selfie.delete().catchError((_) => selfie));
    await service.uploadSelfie(credential, selfie.path);
    await service.removeSelfie(credential);
    expect(requests, [
      'GET /api/v1/mobile/moments/status',
      'GET /api/v1/mobile/moments/photos',
      'POST /api/v1/mobile/moments/selfie',
      'DELETE /api/v1/mobile/moments/selfie',
    ]);
  });

  test('rejects malformed Moments responses', () {
    expect(
      () => MomentsStatus.fromJson({'album_ready': 'yes'}),
      throwsFormatException,
    );
    expect(
      () => MomentsAlbum.fromJson({'results': 'none'}),
      throwsFormatException,
    );
  });
}
