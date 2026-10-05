import 'dart:convert';
import 'dart:math';

import 'package:flutter/foundation.dart';

import '../services/attendee_service.dart';
import '../services/local_store.dart';

class FeedbackAnswer {
  const FeedbackAnswer(this.rating, this.comment);
  final int rating;
  final String comment;
}

/// Feedback the attendee sent from this phone. Answers are anonymous; a
/// random per-install ID lets a changed answer replace the earlier one.
class FeedbackBook extends ChangeNotifier {
  FeedbackBook({LocalStore? store, AttendeeService? service})
    : _store = store ?? PreferencesStore(),
      service = service ?? AttendeeService();

  static const _key = 'feedback_v1';
  final LocalStore _store;
  final AttendeeService service;
  String _device = '';

  /// Answers by session ID; the event itself is the empty ID.
  final Map<String, FeedbackAnswer> _answers = {};
  bool loaded = false;

  FeedbackAnswer? answer(String sessionId) => _answers[sessionId];

  Future<void> load() async {
    try {
      final raw = await _store.read(_key);
      if (raw != null) {
        final json = jsonDecode(raw) as Map<String, dynamic>;
        _device = json['device_id'] as String? ?? '';
        for (final MapEntry(:key, :value)
            in (json['answers'] as Map? ?? const {}).entries) {
          if (key is String && value is Map) {
            _answers[key] = FeedbackAnswer(
              value['rating'] as int? ?? 0,
              value['comment'] as String? ?? '',
            );
          }
        }
      }
    } catch (error) {
      debugPrint('Feedback could not be read: $error');
    }
    if (_device.isEmpty) {
      final random = Random.secure();
      const chars =
          'abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789';
      _device = List.generate(32, (_) => chars[random.nextInt(62)]).join();
    }
    loaded = true;
    notifyListeners();
  }

  /// Sends a rating for the event ([sessionId] empty) or a session.
  /// Throws [AttendeeException] when it cannot be sent.
  Future<void> submit(String sessionId, int rating, String comment) async {
    if (!loaded) await load();
    await service.sendFeedback(
      deviceId: _device,
      sessionId: sessionId,
      rating: rating,
      comment: comment,
    );
    _answers[sessionId] = FeedbackAnswer(rating, comment.trim());
    notifyListeners();
    try {
      await _store.write(
        _key,
        jsonEncode({
          'device_id': _device,
          'answers': {
            for (final MapEntry(:key, :value) in _answers.entries)
              key: {'rating': value.rating, 'comment': value.comment},
          },
        }),
      );
    } catch (error) {
      debugPrint('Feedback could not be saved: $error');
    }
  }
}
