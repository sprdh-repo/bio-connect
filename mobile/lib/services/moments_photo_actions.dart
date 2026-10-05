import 'dart:io';
import 'dart:typed_data';
import 'dart:ui';

import 'package:dio/dio.dart';
import 'package:path_provider/path_provider.dart';
import 'package:permission_handler/permission_handler.dart';
import 'package:saver_gallery/saver_gallery.dart';
import 'package:share_plus/share_plus.dart';

import 'moments_service.dart';

class MomentsPhotoActions {
  MomentsPhotoActions({Dio? dio})
    : _dio =
          dio ??
          Dio(
            BaseOptions(
              connectTimeout: const Duration(seconds: 20),
              receiveTimeout: const Duration(seconds: 60),
            ),
          );
  final Dio _dio;

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
    if (Platform.isIOS && !await Permission.photosAddOnly.request().isGranted) {
      throw const MomentsException(
        'Allow photo access in Settings to save this photo.',
      );
    }
    final bytes = await _download(url);
    Future<SaveResult> saveImage() => SaverGallery.saveImage(
      bytes,
      quality: 100,
      fileName: _filename(url),
      skipIfExists: false,
    );
    var result = await saveImage();
    if (!result.isSuccess &&
        Platform.isAndroid &&
        result.errorMessage?.toLowerCase().contains('permission') == true) {
      if (!await Permission.storage.request().isGranted) {
        throw const MomentsException(
          'Allow storage access in Settings to save this photo.',
        );
      }
      result = await saveImage();
    }
    if (!result.isSuccess) {
      throw const MomentsException(
        'Could not save the photo. Check photo permissions and try again.',
      );
    }
  }

  Future<void> share(String url, {Rect? origin}) async {
    final bytes = await _download(url);
    final root = await getTemporaryDirectory();
    final directory = await Directory('${root.path}/moments-share')
        .create(recursive: true);
    final file = File(
      '${directory.path}/${DateTime.now().microsecondsSinceEpoch}_${_filename(url)}',
    );
    await file.writeAsBytes(bytes, flush: true);
    await SharePlus.instance.share(
      ShareParams(files: [XFile(file.path)], sharePositionOrigin: origin),
    );
  }
}
