class EventContent {
  const EventContent({
    required this.event,
    required this.themes,
    required this.programmeHighlights,
    required this.speakers,
  });
  final EventDetails event;
  final List<EventTheme> themes;
  final List<String> programmeHighlights;
  final List<Speaker> speakers;

  factory EventContent.fromJson(Map<String, dynamic> json) => EventContent(
    event: EventDetails.fromJson(json['event'] as Map<String, dynamic>),
    themes: (json['themes'] as List)
        .map((item) => EventTheme.fromJson(item as Map<String, dynamic>))
        .toList(),
    programmeHighlights: (json['programme_highlights'] as List)
        .cast<String>(),
    speakers: (json['speakers'] as List)
        .map((item) => Speaker.fromJson(item as Map<String, dynamic>))
        .toList(),
  );
}

class EventDetails {
  const EventDetails({
    required this.title,
    required this.tagline,
    required this.description,
    required this.startDate,
    required this.endDate,
    required this.venue,
    required this.city,
  });
  final String title, tagline, description, venue, city;
  final DateTime startDate, endDate;

  factory EventDetails.fromJson(Map<String, dynamic> json) => EventDetails(
    title: json['title'] as String,
    tagline: json['tagline'] as String,
    description: json['description'] as String,
    startDate: DateTime.parse(json['start_date'] as String),
    endDate: DateTime.parse(json['end_date'] as String),
    venue: json['venue'] as String,
    city: json['city'] as String,
  );
}

class EventTheme {
  const EventTheme({
    required this.title,
    required this.description,
    required this.image,
  });
  final String title, description, image;
  factory EventTheme.fromJson(Map<String, dynamic> json) => EventTheme(
    title: json['title'] as String,
    description: json['description'] as String,
    image: json['image'] as String,
  );
}

class Speaker {
  const Speaker({
    required this.id,
    required this.name,
    required this.role,
    required this.organization,
    required this.image,
    required this.linkedin,
  });
  final String id, name, role, organization, image, linkedin;
  factory Speaker.fromJson(Map<String, dynamic> json) => Speaker(
    id: json['id'] as String,
    name: json['name'] as String,
    role: json['role'] as String,
    organization: json['organization'] as String,
    image: json['image'] as String,
    linkedin: json['linkedin'] as String,
  );
}

class Exhibitor {
  const Exhibitor({
    required this.name,
    required this.description,
    required this.logoUrl,
  });
  final String name, description, logoUrl;
  factory Exhibitor.fromJson(Map<String, dynamic> json) => Exhibitor(
    name: json['name'] as String,
    description: json['description'] as String,
    logoUrl: json['logo_url'] as String,
  );
}
