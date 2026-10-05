import 'dart:async';
import 'dart:io';

import 'package:bio_connect_app/widgets/directory.dart';
import 'package:flutter_cache_manager/flutter_cache_manager.dart';
import 'package:flutter_test/flutter_test.dart';

/// Widget tests have no disk cache (sqflite, path_provider), so every test
/// file uses a cache that fails at once, like a phone with no connection and
/// nothing saved. Every remote image therefore shows its fallback.
Future<void> testExecutable(FutureOr<void> Function() testMain) async {
  imageCacheManager = _OfflineImageCache();
  await testMain();
}

class _OfflineImageCache extends Fake implements BaseCacheManager {
  @override
  Stream<FileResponse> getFileStream(
    String url, {
    String? key,
    Map<String, String>? headers,
    bool withProgress = false,
  }) => Stream.error(const SocketException('offline in tests'));
}
