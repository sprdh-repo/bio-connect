import 'event_guide.dart';

class EventContent {
  const EventContent({
    required this.event,
    this.guide = const EventGuide(),
    required this.themes,
    required this.programmeHighlights,
    required this.speakers,
    required this.productLaunch,
    required this.leadership,
    required this.sponsor,
    required this.ecosystemPartners,
  });
  final EventDetails event;
  final EventGuide guide;
  final List<EventTheme> themes;
  final List<String> programmeHighlights;
  final List<Speaker> speakers;
  final ProductLaunch productLaunch;
  final Leadership leadership;
  final Sponsor sponsor;
  final List<Partner> ecosystemPartners;

  factory EventContent.fromJson(Map<String, dynamic> json) => EventContent(
    guide: EventGuide.fromJson(
      json['event_guide'] as Map<String, dynamic>? ?? {},
    ),
    event: EventDetails.fromJson(json['event'] as Map<String, dynamic>),
    themes: (json['themes'] as List)
        .map((item) => EventTheme.fromJson(item as Map<String, dynamic>))
        .toList(),
    programmeHighlights: (json['programme_highlights'] as List).cast<String>(),
    speakers: (json['speakers'] as List)
        .map((item) => Speaker.fromJson(item as Map<String, dynamic>))
        .toList(),
    productLaunch: ProductLaunch.fromJson(
      json['product_launch'] as Map<String, dynamic>,
    ),
    leadership: Leadership.fromJson(json['leadership'] as Map<String, dynamic>),
    sponsor: Sponsor.fromJson(json['sponsor'] as Map<String, dynamic>),
    ecosystemPartners: (json['ecosystem_partners'] as List)
        .map((item) => Partner.fromJson(item as Map<String, dynamic>))
        .toList(),
  );
}

class ProductLaunch {
  const ProductLaunch({
    required this.eyebrow,
    required this.title,
    required this.description,
    required this.deadline,
    required this.eligibility,
    required this.focusAreas,
    required this.applyUrl,
  });
  final String eyebrow, title, description, deadline, applyUrl;
  final List<NamedDescription> eligibility;
  final List<String> focusAreas;

  factory ProductLaunch.fromJson(Map<String, dynamic> json) => ProductLaunch(
    eyebrow: json['eyebrow'] as String,
    title: json['title'] as String,
    description: json['description'] as String,
    deadline: json['deadline'] as String,
    eligibility: (json['eligibility'] as List)
        .map((item) => NamedDescription.fromJson(item as Map<String, dynamic>))
        .toList(),
    focusAreas: (json['focus_areas'] as List).cast<String>(),
    applyUrl: json['apply_url'] as String,
  );
}

class NamedDescription {
  const NamedDescription({required this.title, required this.description});
  final String title, description;
  factory NamedDescription.fromJson(Map<String, dynamic> json) =>
      NamedDescription(
        title: json['title'] as String,
        description: json['description'] as String,
      );
}

class Leadership {
  const Leadership({
    required this.intro,
    required this.advisoryNote,
    required this.people,
  });
  final String intro, advisoryNote;
  final List<Leader> people;
  factory Leadership.fromJson(Map<String, dynamic> json) => Leadership(
    intro: json['intro'] as String,
    advisoryNote: json['advisory_note'] as String,
    people: (json['people'] as List)
        .map((item) => Leader.fromJson(item as Map<String, dynamic>))
        .toList(),
  );
}

class Leader {
  const Leader({required this.name, required this.role, required this.badge});
  final String name, role, badge;
  factory Leader.fromJson(Map<String, dynamic> json) => Leader(
    name: json['name'] as String,
    role: json['role'] as String,
    badge: json['badge'] as String,
  );
}

class Sponsor {
  const Sponsor({
    required this.name,
    required this.description,
    required this.logoUrl,
    required this.websiteUrl,
  });
  final String name, description, logoUrl, websiteUrl;
  factory Sponsor.fromJson(Map<String, dynamic> json) => Sponsor(
    name: json['name'] as String,
    description: json['description'] as String,
    logoUrl: json['logo_url'] as String,
    websiteUrl: json['website_url'] as String,
  );
}

class Partner {
  const Partner({required this.name, required this.logoUrl});
  final String name, logoUrl;
  factory Partner.fromJson(Map<String, dynamic> json) => Partner(
    name: json['name'] as String,
    logoUrl: json['logo_url'] as String,
  );
}

class EventDetails {
  const EventDetails({
    required this.title,
    required this.tagline,
    required this.heroTitle,
    required this.description,
    required this.startDate,
    required this.endDate,
    required this.venue,
    required this.city,
    required this.brochureUrl,
    required this.sponsorshipEmail,
  });
  final String title,
      tagline,
      heroTitle,
      description,
      venue,
      city,
      brochureUrl,
      sponsorshipEmail;
  final DateTime startDate, endDate;

  factory EventDetails.fromJson(Map<String, dynamic> json) => EventDetails(
    title: json['title'] as String,
    tagline: json['tagline'] as String,
    heroTitle: json['hero_title'] as String,
    description: json['description'] as String,
    startDate: DateTime.parse(json['start_date'] as String),
    endDate: DateTime.parse(json['end_date'] as String),
    venue: json['venue'] as String,
    city: json['city'] as String,
    brochureUrl: json['brochure_url'] as String,
    sponsorshipEmail: json['sponsorship_email'] as String,
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
    image: (json['image_url'] ?? json['image'] ?? '') as String,
    linkedin: (json['linkedin'] ?? '') as String,
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
