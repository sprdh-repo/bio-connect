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

/// One entry in the programme. A break (tea, lunch, registration) is a
/// divider in the timetable: it cannot be saved, rated or reminded about.
class GuideSession {
  const GuideSession({
    required this.id,
    required this.title,
    this.kind = '',
    this.label = '',
    this.track = '',
    this.description = '',
    this.startsAt,
    this.endsAt,
    this.location = '',
    this.people = const [],
    this.segments = const [],
    this.speakers = '',
    this.speakerIds = const [],
  });
  final String id, title, kind, label, track, description, location, speakers;
  final DateTime? startsAt, endsAt;

  /// People as printed in the programme, in order.
  final List<SessionPerson> people;

  /// A ceremony's running order.
  final List<SessionSegment> segments;

  /// Speakers from the directory linked to this session or its running order.
  final List<String> speakerIds;

  bool get isBreak => kind == 'break';

  /// Whether attendees can save, rate and be reminded about it.
  bool get plannable => !isBreak;

  /// Whether the programme lists people or a running order, rather than only
  /// the plain speakers line of older content.
  bool get structured => people.isNotEmpty || segments.isNotEmpty;

  /// Everything a search should match, including every person named.
  String get searchText => [
    title,
    label,
    track,
    location,
    speakers,
    for (final p in [...people, for (final s in segments) ...s.people])
      '${p.name} ${p.designation}',
    for (final s in segments) s.title,
  ].join(' ').toLowerCase();

  /// Whether both sessions can be planned, have times and share any minute.
  bool clashesWith(GuideSession other) =>
      other.id != id &&
      plannable &&
      other.plannable &&
      startsAt != null &&
      endsAt != null &&
      other.startsAt != null &&
      other.endsAt != null &&
      startsAt!.isBefore(other.endsAt!) &&
      other.startsAt!.isBefore(endsAt!);

  /// Whether it is under way at [at].
  bool liveAt(DateTime at) =>
      startsAt != null &&
      endsAt != null &&
      !startsAt!.isAfter(at) &&
      endsAt!.isAfter(at);

  factory GuideSession.fromJson(Map<String, dynamic> j) => GuideSession(
    id: str(j['id']),
    title: str(j['title']),
    kind: str(j['kind']),
    label: str(j['label']),
    track: str(j['track']),
    description: str(j['description']),
    startsAt: DateTime.tryParse(str(j['starts_at'])),
    endsAt: DateTime.tryParse(str(j['ends_at'])),
    location: str(j['location']),
    people: SessionPerson.list(j['people']),
    segments: maps(j['segments'])
        .map(SessionSegment.fromJson)
        .where((s) => s.title.isNotEmpty)
        .toList(),
    speakers: str(j['speakers']),
    speakerIds: strings(j['speaker_ids']),
  );
}

/// A person on a session, with the designation printed for that session.
class SessionPerson {
  const SessionPerson({
    required this.name,
    this.designation = '',
    this.role = '',
    this.speakerId = '',
  });
  final String name, designation, role, speakerId;
  static List<SessionPerson> list(Object? json) => maps(json)
      .map(
        (p) => SessionPerson(
          name: str(p['name']),
          designation: str(p['designation']),
          role: str(p['role']),
          speakerId: str(p['speaker_id']),
        ),
      )
      .where((p) => p.name.isNotEmpty)
      .toList();
}

/// One item in a ceremony's running order.
class SessionSegment {
  const SessionSegment({
    required this.title,
    this.description = '',
    this.startsAt,
    this.endsAt,
    this.people = const [],
  });
  final String title, description;
  final DateTime? startsAt, endsAt;
  final List<SessionPerson> people;
  factory SessionSegment.fromJson(Map<String, dynamic> j) => SessionSegment(
    title: str(j['title']),
    description: str(j['description']),
    startsAt: DateTime.tryParse(str(j['starts_at'])),
    endsAt: DateTime.tryParse(str(j['ends_at'])),
    people: SessionPerson.list(j['people']),
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

const _weekdays = ['Mon', 'Tue', 'Wed', 'Thu', 'Fri', 'Sat', 'Sun'];
const _monthNames = [
  'Jan',
  'Feb',
  'Mar',
  'Apr',
  'May',
  'Jun',
  'Jul',
  'Aug',
  'Sep',
  'Oct',
  'Nov',
  'Dec',
];

/// A short day label in IST, like "Thu 8 Oct".
String sessionDayLabel(DateTime value) {
  final t = indiaTime(value);
  return '${_weekdays[t.weekday - 1]} ${t.day} ${_monthNames[t.month - 1]}';
}
