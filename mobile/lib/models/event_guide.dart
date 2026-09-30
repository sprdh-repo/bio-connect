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
    sessions: (json['sessions'] as List? ?? [])
        .map((e) => GuideSession.fromJson(e as Map<String, dynamic>))
        .toList(),
    activities: (json['activities'] as List? ?? [])
        .map((e) => GuideActivity.fromJson(e as Map<String, dynamic>))
        .toList(),
    faqs: (json['faqs'] as List? ?? [])
        .map((e) => GuideFaq.fromJson(e as Map<String, dynamic>))
        .toList(),
    venue: VenueGuide.fromJson(json['venue'] as Map<String, dynamic>? ?? {}),
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
  });
  final String id, title, description, location, speakers;
  final DateTime? startsAt, endsAt;
  factory GuideSession.fromJson(Map<String, dynamic> j) => GuideSession(
    id: j['id'] as String,
    title: j['title'] as String,
    description: j['description'] as String? ?? '',
    startsAt: DateTime.tryParse(j['starts_at'] as String? ?? ''),
    endsAt: DateTime.tryParse(j['ends_at'] as String? ?? ''),
    location: j['location'] as String? ?? '',
    speakers: j['speakers'] as String? ?? '',
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
    id: j['id'] as String,
    title: j['title'] as String,
    description: j['description'] as String? ?? '',
    schedule: j['schedule'] as String? ?? '',
    location: j['location'] as String? ?? '',
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
    id: j['id'] as String,
    question: j['question'] as String,
    answer: j['answer'] as String,
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
  });
  final String address,
      arrival,
      accessibility,
      floorPlanUrl,
      helpEmail,
      helpPhone;
  factory VenueGuide.fromJson(Map<String, dynamic> j) => VenueGuide(
    address: j['address'] as String? ?? '',
    arrival: j['arrival'] as String? ?? '',
    accessibility: j['accessibility'] as String? ?? '',
    floorPlanUrl: j['floor_plan_url'] as String? ?? '',
    helpEmail: j['help_email'] as String? ?? '',
    helpPhone: j['help_phone'] as String? ?? '',
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
