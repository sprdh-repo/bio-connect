import 'event_content.dart' show map, maps, str, strings;

/// Published event-day content. Empty sections are deliberately unannounced.
class EventGuide {
  const EventGuide({
    this.sessions = const [],
    this.activities = const [],
    this.faqs = const [],
    this.venue = const VenueGuide(),
  });
  final List<GuideSession> sessions;
  final List<GuideActivity> activities;
  final List<GuideFaq> faqs;
  final VenueGuide venue;
  factory EventGuide.fromJson(Map<String, dynamic> json) => EventGuide(
    sessions: maps(json['sessions'])
        .map(GuideSession.fromJson)
        .where((s) => s.id.isNotEmpty && s.title.isNotEmpty)
        .toList(),
    activities: maps(json['activities'])
        .map(GuideActivity.fromJson)
        .where((a) => a.id.isNotEmpty && a.title.isNotEmpty)
        .toList(),
    faqs: maps(json['faqs'])
        .map(GuideFaq.fromJson)
        .where((f) => f.question.isNotEmpty && f.answer.isNotEmpty)
        .toList(),
    venue: VenueGuide.fromJson(map(json['venue'])),
  );
}

class GuideSession {
  const GuideSession({
    required this.id,
    required this.title,
    this.description = '',
    this.startsAt,
    this.endsAt,
    this.location = '',
    this.speakers = '',
    this.speakerIds = const [],
  });
  final String id, title, description, location, speakers;
  final DateTime? startsAt, endsAt;

  /// Speakers from the directory that staff linked to this session.
  final List<String> speakerIds;

  /// Whether both sessions have times and they share any minute.
  bool clashesWith(GuideSession other) =>
      other.id != id &&
      startsAt != null &&
      endsAt != null &&
      other.startsAt != null &&
      other.endsAt != null &&
      startsAt!.isBefore(other.endsAt!) &&
      other.startsAt!.isBefore(endsAt!);
  factory GuideSession.fromJson(Map<String, dynamic> j) => GuideSession(
    id: str(j['id']),
    title: str(j['title']),
    description: str(j['description']),
    startsAt: DateTime.tryParse(str(j['starts_at'])),
    endsAt: DateTime.tryParse(str(j['ends_at'])),
    location: str(j['location']),
    speakers: str(j['speakers']),
    speakerIds: strings(j['speaker_ids']),
  );
}

class GuideActivity {
  const GuideActivity({
    required this.id,
    required this.title,
    this.description = '',
    this.schedule = '',
    this.location = '',
  });
  final String id, title, description, schedule, location;
  factory GuideActivity.fromJson(Map<String, dynamic> j) => GuideActivity(
    id: str(j['id']),
    title: str(j['title']),
    description: str(j['description']),
    schedule: str(j['schedule']),
    location: str(j['location']),
  );
}

class GuideFaq {
  const GuideFaq({
    required this.id,
    required this.question,
    required this.answer,
  });
  final String id, question, answer;
  factory GuideFaq.fromJson(Map<String, dynamic> j) => GuideFaq(
    id: str(j['id']),
    question: str(j['question']),
    answer: str(j['answer']),
  );
}

class VenueGuide {
  const VenueGuide({
    this.address = '',
    this.arrival = '',
    this.accessibility = '',
    this.floorPlanUrl = '',
    this.helpEmail = '',
    this.helpPhone = '',
    this.helpWhatsApp = '',
  });
  final String address,
      arrival,
      accessibility,
      floorPlanUrl,
      helpEmail,
      helpPhone,
      helpWhatsApp;
  bool get hasHelp =>
      helpEmail.isNotEmpty || helpPhone.isNotEmpty || helpWhatsApp.isNotEmpty;
  factory VenueGuide.fromJson(Map<String, dynamic> j) => VenueGuide(
    address: str(j['address']),
    arrival: str(j['arrival']),
    accessibility: str(j['accessibility']),
    floorPlanUrl: str(j['floor_plan_url']),
    helpEmail: str(j['help_email']),
    helpPhone: str(j['help_phone']),
    helpWhatsApp: str(j['help_whatsapp']),
  );
}

/// Use event time in India regardless of the attendee's device timezone.
DateTime indiaTime(DateTime value) =>
    value.toUtc().add(const Duration(hours: 5, minutes: 30));
String sessionTime(DateTime value) {
  final t = indiaTime(value);
  return '${t.hour.toString().padLeft(2, '0')}:${t.minute.toString().padLeft(2, '0')}';
}

String sessionDay(DateTime value) {
  final t = indiaTime(value);
  return '${t.day.toString().padLeft(2, '0')}/${t.month.toString().padLeft(2, '0')}/${t.year}';
}
