import 'package:dio/dio.dart';

import 'content_service.dart';
import 'pass_service.dart';

class MomentsException implements Exception {
  const MomentsException(this.message, {this.unauthorized = false});
  final String message;
  final bool unauthorized;
  @override
  String toString() => message;
}

class MomentsStatus {
  const MomentsStatus({
    required this.albumReady,
    required this.selfieUploaded,
    required this.similarPhotosCount,
    required this.photosReady,
    required this.processingStatus,
    this.uploadedPhotoUrl = '',
  });
  final bool albumReady, selfieUploaded, photosReady;
  final int similarPhotosCount;
  final String processingStatus, uploadedPhotoUrl;

  bool get isProcessing =>
      selfieUploaded &&
      (processingStatus == 'pending' || processingStatus == 'processing');
  bool get processingFailed => selfieUploaded && processingStatus == 'failed';

  factory MomentsStatus.fromJson(Map<String, dynamic> json) {
    if (json['album_ready'] is! bool ||
        json['selfie_uploaded'] is! bool ||
        json['similar_photos_count'] is! int ||
        json['similar_photos_ready'] is! bool) {
      throw const FormatException('Invalid Moments status');
    }
    final selfieUploaded = json['selfie_uploaded'] as bool;
    return MomentsStatus(
      albumReady: json['album_ready'] as bool,
      selfieUploaded: selfieUploaded,
      similarPhotosCount: json['similar_photos_count'] as int,
      photosReady: json['similar_photos_ready'] as bool,
      processingStatus:
          json['processing_status'] as String? ??
          (selfieUploaded ? 'completed' : 'pending'),
      uploadedPhotoUrl: json['uploaded_photo_url'] as String? ?? '',
    );
  }
}

class MomentPhoto {
  const MomentPhoto({
    required this.filename,
    required this.fileUrl,
    this.thumbnailUrl = '',
  });
  final String filename, fileUrl, thumbnailUrl;
  factory MomentPhoto.fromJson(Map<String, dynamic> json) {
    final filename = json['filename'], fileUrl = json['file_url'];
    if (filename is! String || fileUrl is! String || fileUrl.isEmpty) {
      throw const FormatException('Invalid Moments photo');
    }
    return MomentPhoto(
      filename: filename,
      fileUrl: fileUrl,
      thumbnailUrl: json['thumbnail_url'] as String? ?? '',
    );
  }
}

class MomentsAlbum {
  const MomentsAlbum({required this.photos, this.processingStatus = ''});
  final List<MomentPhoto> photos;
  final String processingStatus;
  factory MomentsAlbum.fromJson(Map<String, dynamic> json) {
    final results = json['results'];
    if (results is! List) throw const FormatException('Invalid Moments album');
    return MomentsAlbum(
      photos: results
          .whereType<Map>()
          .map((item) => MomentPhoto.fromJson(item.cast<String, dynamic>()))
          .toList(),
      processingStatus: json['status'] as String? ?? '',
    );
  }
}

class MomentsService {
  MomentsService({Dio? dio, String? apiBaseUrl})
    : _dio =
          dio ??
          Dio(
            BaseOptions(
              baseUrl: apiBaseUrl ?? CurrentContentService.defaultApiBaseUrl,
              connectTimeout: const Duration(seconds: 15),
              receiveTimeout: const Duration(seconds: 30),
              sendTimeout: const Duration(seconds: 90),
            ),
          );
  final Dio _dio;

  Options _options(MomentsCredential credential, {String method = 'GET'}) =>
      Options(
        method: method,
        headers: {'Authorization': 'Bearer ${credential.token}'},
      );

  Map<String, String> _query(MomentsCredential credential) => {
    'pass_id': credential.pass.id,
  };

  Future<MomentsStatus> status(MomentsCredential credential) async {
    final json = await _json('/api/v1/mobile/moments/status', credential);
    try {
      return MomentsStatus.fromJson(json);
    } on FormatException {
      throw const MomentsException(
        'Moments returned an unexpected response. Please try again.',
      );
    }
  }

  Future<MomentsAlbum> photos(MomentsCredential credential) async {
    final json = await _json('/api/v1/mobile/moments/photos', credential);
    try {
      return MomentsAlbum.fromJson(json);
    } on FormatException {
      throw const MomentsException(
        'Moments returned an unexpected response. Please try again.',
      );
    }
  }

  Future<void> uploadSelfie(MomentsCredential credential, String path) async {
    try {
      await _dio.post<Map<String, dynamic>>(
        '/api/v1/mobile/moments/selfie',
        queryParameters: _query(credential),
        data: FormData.fromMap({
          'photo': await MultipartFile.fromFile(
            path,
            filename: path.split('/').last,
          ),
        }),
        options: _options(credential, method: 'POST'),
      );
    } on DioException catch (error) {
      throw _error(error, 'Could not upload your selfie. Please try again.');
    }
  }

  Future<void> removeSelfie(MomentsCredential credential) async {
    try {
      await _dio.delete<void>(
        '/api/v1/mobile/moments/selfie',
        queryParameters: _query(credential),
        options: _options(credential, method: 'DELETE'),
      );
    } on DioException catch (error) {
      throw _error(error, 'Could not remove your selfie. Please try again.');
    }
  }

  Future<Map<String, dynamic>> _json(
    String path,
    MomentsCredential credential,
  ) async {
    try {
      final response = await _dio.get<Map<String, dynamic>>(
        path,
        queryParameters: _query(credential),
        options: _options(credential),
      );
      if (response.data == null) throw const FormatException();
      return response.data!;
    } on FormatException {
      throw const MomentsException(
        'Moments returned an unexpected response. Please try again.',
      );
    } on DioException catch (error) {
      throw _error(error, 'Could not load Moments. Please try again.');
    }
  }

  MomentsException _error(DioException error, String fallback) {
    final body = error.response?.data;
    final message = body is Map && body['error'] is String
        ? body['error'] as String
        : fallback;
    return MomentsException(
      message,
      unauthorized: error.response?.statusCode == 401,
    );
  }
}
