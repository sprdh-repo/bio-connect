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
    this.sponsors = const [],
    this.sponsorsIntro = '',
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

  /// Every sponsor, in display order. Content published before the list
  /// existed only carries [sponsor].
  final List<Sponsor> sponsors;
  final String sponsorsIntro;
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
    sponsors: switch (json['sponsors']) {
      final List items when items.isNotEmpty =>
        items
            .map((item) => Sponsor.fromJson(item as Map<String, dynamic>))
            .toList(),
      _ => [Sponsor.fromJson(json['sponsor'] as Map<String, dynamic>)],
    },
    sponsorsIntro: json['sponsors_intro'] as String? ?? '',
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
    this.convenedBy,
    this.committee = const Committee(),
  });
  final String intro, advisoryNote;
  final List<Leader> people;
  final NamedNote? convenedBy;
  final Committee committee;
  factory Leadership.fromJson(Map<String, dynamic> json) => Leadership(
    intro: json['intro'] as String,
    advisoryNote: json['advisory_note'] as String,
    people: (json['people'] as List)
        .map((item) => Leader.fromJson(item as Map<String, dynamic>))
        .toList(),
    convenedBy: switch (json['convened_by']) {
      final Map<String, dynamic> item => NamedNote.fromJson(item),
      _ => null,
    },
    committee: Committee.fromJson(
      json['committee'] as Map<String, dynamic>? ?? const {},
    ),
  );
}

class Leader {
  const Leader({
    required this.name,
    required this.role,
    required this.badge,
    this.imageUrl = '',
    this.imageCredit = '',
    this.imageCreditUrl = '',
  });
  final String name, role, badge, imageUrl, imageCredit, imageCreditUrl;
  factory Leader.fromJson(Map<String, dynamic> json) => Leader(
    name: json['name'] as String,
    role: json['role'] as String,
    badge: json['badge'] as String,
    imageUrl: json['image_url'] as String? ?? '',
    imageCredit: json['image_credit'] as String? ?? '',
    imageCreditUrl: json['image_credit_url'] as String? ?? '',
  );
}

class NamedNote {
  const NamedNote({required this.name, this.note = ''});
  final String name, note;
  factory NamedNote.fromJson(Map<String, dynamic> json) => NamedNote(
    name: json['name'] as String? ?? '',
    note: json['note'] as String? ?? '',
  );
}

/// The advisory committee roster. Empty until published in app content.
class Committee {
  const Committee({
    this.title = '',
    this.orderNote = '',
    this.members = const [],
  });
  final String title, orderNote;
  final List<CommitteeMember> members;
  factory Committee.fromJson(Map<String, dynamic> json) => Committee(
    title: json['title'] as String? ?? '',
    orderNote: json['order_note'] as String? ?? '',
    members: (json['members'] as List? ?? const [])
        .map((item) => CommitteeMember.fromJson(item as Map<String, dynamic>))
        .toList(),
  );
}

class CommitteeMember {
  const CommitteeMember({
    required this.role,
    required this.name,
    required this.organization,
  });
  final String role, name, organization;

  /// Chairs and convenors lead the roster visually.
  bool get isLead => role != 'Member';
  factory CommitteeMember.fromJson(Map<String, dynamic> json) =>
      CommitteeMember(
        role: json['role'] as String? ?? 'Member',
        name: json['name'] as String? ?? '',
        organization: json['organization'] as String? ?? '',
      );
}

class Sponsor {
  const Sponsor({
    required this.name,
    required this.description,
    required this.logoUrl,
    required this.websiteUrl,
    this.category = '',
  });

  /// [category] is a short descriptor such as "Public sector bank · Mumbai".
  final String name, description, logoUrl, websiteUrl, category;
  factory Sponsor.fromJson(Map<String, dynamic> json) => Sponsor(
    name: json['name'] as String,
    description: json['description'] as String,
    logoUrl: json['logo_url'] as String,
    websiteUrl: json['website_url'] as String,
    category: json['category'] as String? ?? '',
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
    this.stallNumber = '',
    this.stallType = '',
  });

  /// [stallNumber] is empty until the organisers allocate a stall.
  final String name, description, logoUrl, stallNumber, stallType;
  bool get hasStall => stallNumber.isNotEmpty;

  Exhibitor withLogoUrl(String value) => Exhibitor(
    name: name,
    description: description,
    logoUrl: value,
    stallNumber: stallNumber,
    stallType: stallType,
  );

  factory Exhibitor.fromJson(Map<String, dynamic> json) => Exhibitor(
    name: json['name'] as String,
    description: json['description'] as String? ?? '',
    logoUrl: json['logo_url'] as String? ?? '',
    stallNumber: json['stall_number'] as String? ?? '',
    stallType: json['stall_type'] as String? ?? '',
  );
}

/// Orders stalls the way they read on a floor plan: "A-2" before "A-10".
int compareStallNumbers(String a, String b) {
  final pattern = RegExp(r'\d+|\D+');
  final left = pattern.allMatches(a).map((m) => m[0]!).toList();
  final right = pattern.allMatches(b).map((m) => m[0]!).toList();
  for (var i = 0; i < left.length && i < right.length; i++) {
    final x = int.tryParse(left[i]), y = int.tryParse(right[i]);
    final order = x != null && y != null
        ? x.compareTo(y)
        : left[i].toLowerCase().compareTo(right[i].toLowerCase());
    if (order != 0) return order;
  }
  return left.length.compareTo(right.length);
}
