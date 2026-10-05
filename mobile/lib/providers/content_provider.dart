import 'package:flutter/foundation.dart';

import '../models/event_content.dart';
import '../services/content_service.dart';

class ContentProvider extends ChangeNotifier {
  ContentProvider(this._service);
  final ContentService _service;
  EventContent? content;
  List<Exhibitor> exhibitors = const [];
  bool loading = false;
  bool exhibitorsLoading = false;

  /// True once the exhibitor directory has been fetched at least once.
  bool exhibitorsLoaded = false;
  String? error;
  String? exhibitorsError;

  /// True when the last refresh could not reach the backend, so the guide is
  /// showing saved (bundled or earlier) content.
  bool offline = false;
  Future<bool>? _contentRequest, _exhibitorRequest;

  /// Loads the bundled snapshot once, then refreshes from the backend.
  /// Concurrent callers share one request. Completes with whether live
  /// content was reached.
  Future<bool> load() =>
      _contentRequest ??= _load().whenComplete(() => _contentRequest = null);

  Future<bool> _load() async {
    loading = true;
    error = null;
    notifyListeners();
    try {
      content ??= await _service.loadEvent();
      notifyListeners();
      try {
        content = await _service.fetchEvent(content!);
        offline = false;
      } catch (_) {
        offline = true;
      }
    } catch (_) {
      error = 'Event details could not be loaded.';
    } finally {
      loading = false;
      notifyListeners();
    }
    return error == null && !offline;
  }

  /// Fetches the exhibitor directory, keeping the previous list on failure.
  Future<bool> loadExhibitors() => _exhibitorRequest ??= _loadExhibitors()
      .whenComplete(() => _exhibitorRequest = null);

  Future<bool> _loadExhibitors() async {
    exhibitorsLoading = true;
    exhibitorsError = null;
    notifyListeners();
    try {
      exhibitors = await _service.loadExhibitors();
      exhibitorsLoaded = true;
    } catch (_) {
      exhibitorsError =
          'The exhibitor directory is unavailable. Please try again.';
    } finally {
      exhibitorsLoading = false;
      notifyListeners();
    }
    return exhibitorsError == null;
  }
}
