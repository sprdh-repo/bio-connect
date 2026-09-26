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
  String? error;
  String? exhibitorsError;

  Future<void> load() async {
    loading = true;
    error = null;
    notifyListeners();
    try {
      content = await _service.loadEvent();
    } catch (_) {
      error = 'Event details could not be loaded.';
    } finally {
      loading = false;
      notifyListeners();
    }
  }

  Future<void> loadExhibitors() async {
    exhibitorsLoading = true;
    exhibitorsError = null;
    notifyListeners();
    try {
      exhibitors = await _service.loadExhibitors();
    } catch (_) {
      exhibitorsError =
          'The exhibitor directory is unavailable. Please try again.';
    } finally {
      exhibitorsLoading = false;
      notifyListeners();
    }
  }
}
