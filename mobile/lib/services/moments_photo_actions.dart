import 'dart:typed_data';
import 'dart:ui';

import 'package:dio/dio.dart';

import 'moments_service.dart';
import 'photo_export.dart';

class MomentsPhotoActions {
  MomentsPhotoActions({Dio? dio, this.export = const PhotoExport()})
    : _dio =
          dio ??
          Dio(
            BaseOptions(
              connectTimeout: const Duration(seconds: 20),
              receiveTimeout: const Duration(seconds: 60),
            ),
          );
  final Dio _dio;
  final PhotoExport export;

  Future<Uint8List> _download(String url) async {
    final response = await _dio.get<List<int>>(
      url,
      options: Options(responseType: ResponseType.bytes),
    );
    if (response.data == null || response.data!.isEmpty) {
      throw const MomentsException('The photo could not be downloaded.');
    }
    return Uint8List.fromList(response.data!);
  }

  String _filename(String url) {
    final name = Uri.tryParse(url)?.pathSegments.last ?? '';
    return name.isEmpty ? 'bio-connect-moment.jpg' : name;
  }

  Future<void> save(String url) async {
    final bytes = await _download(url);
    try {
      await export.save(bytes, _filename(url));
    } on PhotoExportException catch (error) {
      throw MomentsException(error.message);
    }
  }

  Future<void> share(String url, {Rect? origin}) async {
    final bytes = await _download(url);
    await export.share(bytes, _filename(url), origin: origin);
  }
}
