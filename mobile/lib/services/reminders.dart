import 'package:flutter/foundation.dart';
import 'package:flutter_local_notifications/flutter_local_notifications.dart';
import 'package:timezone/timezone.dart' as tz;

/// One scheduled local notification. [payload] is a deep link such as
/// `session:<id>` that opens the matching page when the notification is tapped.
class Reminder {
  const Reminder({
    required this.id,
    required this.at,
    required this.title,
    required this.body,
    required this.payload,
  });
  final int id;
  final DateTime at;
  final String title, body, payload;
}

/// Session reminders are scheduled on the phone itself, so they arrive
/// without a connection and need no server.
abstract interface class Reminders {
  /// Prepares notifications; [onTap] receives the payload of a tapped one.
  Future<void> init(void Function(String payload) onTap);

  /// The payload of the notification that launched the app, if any.
  Future<String?> launchPayload();

  /// Asks for permission to notify. True when notifications may be shown.
  Future<bool> requestPermission();

  /// Replaces every scheduled reminder with [reminders].
  Future<void> replace(List<Reminder> reminders);
}

class NoReminders implements Reminders {
  const NoReminders();
  @override
  Future<void> init(void Function(String payload) onTap) async {}
  @override
  Future<String?> launchPayload() async => null;
  @override
  Future<bool> requestPermission() async => false;
  @override
  Future<void> replace(List<Reminder> reminders) async {}
}

class LocalReminders implements Reminders {
  LocalReminders([FlutterLocalNotificationsPlugin? plugin])
    : _plugin = plugin ?? FlutterLocalNotificationsPlugin();
  final FlutterLocalNotificationsPlugin _plugin;
  Future<bool>? _ready;

  static const _details = NotificationDetails(
    android: AndroidNotificationDetails(
      'session_reminders',
      'Session reminders',
      channelDescription: 'A reminder before sessions you saved begin',
      importance: Importance.high,
      priority: Priority.high,
      icon: 'ic_stat_notify',
    ),
    iOS: DarwinNotificationDetails(),
  );

  @override
  Future<void> init(void Function(String payload) onTap) async {
    _ready ??= () async {
      try {
        await _plugin.initialize(
          settings: const InitializationSettings(
            android: AndroidInitializationSettings('ic_stat_notify'),
            // Permission is asked for when the attendee first saves a session.
            iOS: DarwinInitializationSettings(
              requestAlertPermission: false,
              requestBadgePermission: false,
              requestSoundPermission: false,
            ),
          ),
          onDidReceiveNotificationResponse: (response) {
            if (response.payload case final payload?) onTap(payload);
          },
        );
        return true;
      } catch (error) {
        debugPrint('Reminders unavailable: $error');
        return false;
      }
    }();
    await _ready;
  }

  @override
  Future<String?> launchPayload() async {
    if (await _ready != true) return null;
    try {
      final details = await _plugin.getNotificationAppLaunchDetails();
      return details?.didNotificationLaunchApp == true
          ? details?.notificationResponse?.payload
          : null;
    } catch (_) {
      return null;
    }
  }

  @override
  Future<bool> requestPermission() async {
    if (await _ready != true) return false;
    try {
      final android = _plugin
          .resolvePlatformSpecificImplementation<
            AndroidFlutterLocalNotificationsPlugin
          >();
      if (android != null) {
        return await android.requestNotificationsPermission() ?? false;
      }
      final ios = _plugin
          .resolvePlatformSpecificImplementation<
            IOSFlutterLocalNotificationsPlugin
          >();
      return await ios?.requestPermissions(
            alert: true,
            badge: false,
            sound: true,
          ) ??
          false;
    } catch (_) {
      return false;
    }
  }

  @override
  Future<void> replace(List<Reminder> reminders) async {
    if (await _ready != true) return;
    try {
      await _plugin.cancelAllPendingNotifications();
      for (final r in reminders) {
        // Inexact delivery needs no exact-alarm permission; a reminder a few
        // minutes before a session tolerates the system's batching.
        await _plugin.zonedSchedule(
          id: r.id,
          scheduledDate: tz.TZDateTime.from(r.at.toUtc(), tz.UTC),
          notificationDetails: _details,
          androidScheduleMode: AndroidScheduleMode.inexactAllowWhileIdle,
          title: r.title,
          body: r.body,
          payload: r.payload,
        );
      }
    } catch (error) {
      debugPrint('Could not schedule reminders: $error');
    }
  }
}
