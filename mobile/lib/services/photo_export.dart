import 'dart:io';
import 'dart:typed_data';
import 'dart:ui';

import 'package:path_provider/path_provider.dart';
import 'package:permission_handler/permission_handler.dart';
import 'package:saver_gallery/saver_gallery.dart';
import 'package:share_plus/share_plus.dart';

class PhotoExportException implements Exception {
  const PhotoExportException(this.message);
  final String message;
}

/// Saves a picture to the phone's gallery or hands it to the share sheet.
class PhotoExport {
  const PhotoExport();

  Future<void> save(Uint8List bytes, String fileName) async {
    if (Platform.isIOS && !await Permission.photosAddOnly.request().isGranted) {
      throw const PhotoExportException(
        'Allow photo access in Settings to save this photo.',
      );
    }
    Future<SaveResult> saveImage() => SaverGallery.saveImage(
      bytes,
      quality: 100,
      fileName: fileName,
      skipIfExists: false,
    );
    var result = await saveImage();
    if (!result.isSuccess &&
        Platform.isAndroid &&
        result.errorMessage?.toLowerCase().contains('permission') == true) {
      if (!await Permission.storage.request().isGranted) {
        throw const PhotoExportException(
          'Allow storage access in Settings to save this photo.',
        );
      }
      result = await saveImage();
    }
    if (!result.isSuccess) {
      throw const PhotoExportException(
        'Could not save the photo. Check photo permissions and try again.',
      );
    }
  }

  Future<void> share(
    Uint8List bytes,
    String fileName, {
    Rect? origin,
    String? text,
  }) async {
    final root = await getTemporaryDirectory();
    final directory = await Directory('${root.path}/photo-share')
        .create(recursive: true);
    final file = File(
      '${directory.path}/${DateTime.now().microsecondsSinceEpoch}_$fileName',
    );
    await file.writeAsBytes(bytes, flush: true);
    await SharePlus.instance.share(
      ShareParams(
        files: [XFile(file.path)],
        text: text,
        sharePositionOrigin: origin,
      ),
    );
  }
}
