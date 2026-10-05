import 'event_guide.dart';

// Content arrives from staff-edited JSON. Missing or mistyped values read as
// empty, and the screens leave empty values out, so one bad field never takes
// the guide down.
String str(Object? value) => value is String ? value.trim() : '';
List<Map<String, dynamic>> maps(Object? value) => value is List
    ? value.whereType<Map>().map((m) => m.cast<String, dynamic>()).toList()
    : const [];
List<String> strings(Object? value) => value is List
    ? value.whereType<String>().where((s) => s.trim().isNotEmpty).toList()
    : const [];
Map<String, dynamic> map(Object? value) =>
    value is Map ? value.cast<String, dynamic>() : const {};

class EventContent {
  const EventContent({
    required this.event,
    this.guide = const EventGuide(),
    required this.themes,
    required this.programmeHighlights,
    required this.speakers,
    required this.productLaunch,
    required this.leadership,
    this.sponsors = const [],
    this.sponsorsIntro = '',
    required this.ecosystemPartners,
    this.menus = const {},
    this.copy = const {},
  });
  final EventDetails event;
  final EventGuide guide;
  final List<EventTheme> themes;
  final List<String> programmeHighlights;
  final List<Speaker> speakers;
  final ProductLaunch productLaunch;
  final Leadership leadership;

  /// Every sponsor, in display order.
  final List<Sponsor> sponsors;
  final String sponsorsIntro;
  final List<Partner> ecosystemPartners;

  /// Staff-arranged menus by name. A missing menu keeps the built-in layout;
  /// a present one, even empty, is shown exactly as published.
  final Map<String, List<MenuEntry>> menus;

  /// Staff overrides of the app's headings, keyed like "sessions.title".
  final Map<String, String> copy;

  /// The staff override for [key], or the app's own wording.
  String text(String key, String fallback) {
    final value = copy[key];
    return value == null || value.trim().isEmpty ? fallback : value;
  }

  /// [text] that is empty unless staff set it.
  String? override(String key) {
    final value = copy[key];
    return value == null || value.trim().isEmpty ? null : value;
  }

  List<MenuEntry> menu(String name) => menus[name] ?? defaultMenus[name]!;

  factory EventContent.fromJson(Map<String, dynamic> json) => EventContent(
    guide: EventGuide.fromJson(map(json['event_guide'])),
    event: EventDetails.fromJson(map(json['event'])),
    themes: maps(json['themes']).map(EventTheme.fromJson).toList(),
    programmeHighlights: strings(json['programme_highlights']),
    speakers: maps(json['speakers'])
        .map(Speaker.fromJson)
        .where((s) => s.id.isNotEmpty && s.name.isNotEmpty)
        .toList(),
    productLaunch: ProductLaunch.fromJson(map(json['product_launch'])),
    leadership: Leadership.fromJson(map(json['leadership'])),
    // Content published before the list existed carries one sponsor object.
    sponsors:
        (json['sponsors'] is List
                ? maps(json['sponsors'])
                : [map(json['sponsor'])])
            .map(Sponsor.fromJson)
            .where((s) => s.name.isNotEmpty)
            .toList(),
    sponsorsIntro: str(json['sponsors_intro']),
    ecosystemPartners: maps(json['ecosystem_partners'])
        .map(Partner.fromJson)
        .where((p) => p.name.isNotEmpty)
        .toList(),
    menus: {
      for (final MapEntry(:key, :value) in map(json['menus']).entries)
        if (defaultMenus.containsKey(key) && value is List)
          key: maps(value)
              .map(MenuEntry.fromJson)
              .where((e) => destinationKeys.contains(e.key))
              .toList(),
    },
    copy: {
      for (final MapEntry(:key, :value) in map(json['copy']).entries)
        if (value is String) key: value,
    },
  );
}

/// One menu entry: an app destination, or "link" to [url].
class MenuEntry {
  const MenuEntry(
    this.key, [
    this.title = '',
    this.subtitle = '',
    this.url = '',
  ]);
  final String key, title, subtitle, url;
  factory MenuEntry.fromJson(Map<String, dynamic> json) => MenuEntry(
    str(json['key']),
    str(json['title']),
    str(json['subtitle']),
    str(json['url']),
  );
}

const destinationKeys = {
  'sessions',
  'speakers',
  'venue',
  'activities',
  'faqs',
  'exhibitors',
  'my_passes',
  'registration',
  'brochure',
  'product_launch',
  'sponsors',
  'leadership',
  'explore',
  'privacy',
  'link',
};

/// The layout released before menus were published from the console.
const defaultMenus = <String, List<MenuEntry>>{
  'tabs': [
    MenuEntry('sessions', 'Sessions'),
    MenuEntry('speakers', 'Speakers'),
  ],
  'home_shortcuts': [
    MenuEntry('sessions', 'Sessions', 'Programme & timings'),
    MenuEntry('venue', 'Venue', 'Directions & arrival'),
    MenuEntry('activities', 'Activities', 'Discover & connect'),
    MenuEntry('faqs', 'FAQs', 'Event-day answers'),
    MenuEntry('speakers', 'Speakers', 'Meet the voices'),
    MenuEntry('exhibitors', 'Exhibitors', 'Explore the expo'),
  ],
  'home_links': [
    MenuEntry('my_passes', 'My passes', 'View your admission QR on this phone'),
    MenuEntry(
      'registration',
      'Registration & passes',
      'Register or review delegate options',
    ),
    MenuEntry(
      'explore',
      'Explore Bio Connect',
      'Themes, ideas and programme highlights',
    ),
  ],
  'guide': [
    MenuEntry('venue', 'Venue & directions'),
    MenuEntry('activities', 'Activities', 'Discover what is happening'),
    MenuEntry('faqs', 'FAQs', 'Answers and event-day help'),
    MenuEntry('exhibitors', 'Exhibitors', 'Stalls and the expo line-up'),
    MenuEntry('my_passes', 'My passes', 'View your admission QR on this phone'),
    MenuEntry(
      'registration',
      'Registration & passes',
      'Delegate and exhibition options',
    ),
    MenuEntry('brochure', 'Event brochure', 'View the programme overview'),
    MenuEntry(
      'product_launch',
      'Product launch',
      'Kerala Startup Mission showcase',
    ),
    MenuEntry('sponsors', 'Sponsors'),
    MenuEntry(
      'leadership',
      'Leadership',
      'State leadership and the advisory committee',
    ),
    MenuEntry(
      'privacy',
      'Privacy policy',
      'How we use and protect your information',
    ),
  ],
};

class ProductLaunch {
  const ProductLaunch({
    this.eyebrow = '',
    this.title = '',
    this.description = '',
    this.deadline = '',
    this.eligibility = const [],
    this.focusAreas = const [],
    this.applyUrl = '',
    this.applyLabel = '',
  });
  final String eyebrow, title, description, deadline, applyUrl, applyLabel;
  final List<NamedDescription> eligibility;
  final List<String> focusAreas;

  factory ProductLaunch.fromJson(Map<String, dynamic> json) => ProductLaunch(
    eyebrow: str(json['eyebrow']),
    title: str(json['title']),
    description: str(json['description']),
    deadline: str(json['deadline']),
    eligibility: maps(json['eligibility'])
        .map(NamedDescription.fromJson)
        .where((e) => e.title.isNotEmpty)
        .toList(),
    focusAreas: strings(json['focus_areas']),
    applyUrl: str(json['apply_url']),
    applyLabel: str(json['apply_label']),
  );
}

class NamedDescription {
  const NamedDescription({required this.title, this.description = ''});
  final String title, description;
  factory NamedDescription.fromJson(Map<String, dynamic> json) =>
      NamedDescription(
        title: str(json['title']),
        description: str(json['description']),
      );
}

class Leadership {
  const Leadership({
    this.intro = '',
    this.advisoryNote = '',
    this.people = const [],
    this.convenedBy,
    this.committee = const Committee(),
  });
  final String intro, advisoryNote;
  final List<Leader> people;
  final NamedNote? convenedBy;
  final Committee committee;
  bool get isEmpty =>
      intro.isEmpty && people.isEmpty && committee.members.isEmpty;
  factory Leadership.fromJson(Map<String, dynamic> json) {
    final convenor = NamedNote.fromJson(map(json['convened_by']));
    return Leadership(
      intro: str(json['intro']),
      advisoryNote: str(json['advisory_note']),
      people: maps(json['people'])
          .map(Leader.fromJson)
          .where((p) => p.name.isNotEmpty)
          .toList(),
      convenedBy: convenor.name.isEmpty ? null : convenor,
      committee: Committee.fromJson(map(json['committee'])),
    );
  }
}

class Leader {
  const Leader({
    required this.name,
    this.role = '',
    this.badge = '',
    this.imageUrl = '',
    this.imageCredit = '',
    this.imageCreditUrl = '',
  });
  final String name, role, badge, imageUrl, imageCredit, imageCreditUrl;
  factory Leader.fromJson(Map<String, dynamic> json) => Leader(
    name: str(json['name']),
    role: str(json['role']),
    badge: str(json['badge']),
    imageUrl: str(json['image_url']),
    imageCredit: str(json['image_credit']),
    imageCreditUrl: str(json['image_credit_url']),
  );
}

class NamedNote {
  const NamedNote({required this.name, this.note = ''});
  final String name, note;
  factory NamedNote.fromJson(Map<String, dynamic> json) =>
      NamedNote(name: str(json['name']), note: str(json['note']));
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
    title: str(json['title']),
    orderNote: str(json['order_note']),
    members: maps(json['members'])
        .map(CommitteeMember.fromJson)
        .where((m) => m.name.isNotEmpty)
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
        role: switch (str(json['role'])) {
          '' => 'Member',
          final role => role,
        },
        name: str(json['name']),
        organization: str(json['organization']),
      );
}

class Sponsor {
  const Sponsor({
    required this.name,
    this.description = '',
    this.logoUrl = '',
    this.websiteUrl = '',
    this.category = '',
  });

  /// [category] is a short descriptor such as "Public sector bank · Mumbai".
  final String name, description, logoUrl, websiteUrl, category;
  factory Sponsor.fromJson(Map<String, dynamic> json) => Sponsor(
    name: str(json['name']),
    description: str(json['description']),
    logoUrl: str(json['logo_url']),
    websiteUrl: str(json['website_url']),
    category: str(json['category']),
  );
}

class Partner {
  const Partner({required this.name, this.logoUrl = '', this.websiteUrl = ''});
  final String name, logoUrl, websiteUrl;
  factory Partner.fromJson(Map<String, dynamic> json) => Partner(
    name: str(json['name']),
    logoUrl: str(json['logo_url']),
    websiteUrl: str(json['website_url']),
  );
}

class EventDetails {
  const EventDetails({
    required this.title,
    this.tagline = '',
    this.heroTitle = '',
    this.description = '',
    required this.startDate,
    required this.endDate,
    this.venue = '',
    this.city = '',
    this.brochureUrl = '',
    this.sponsorshipEmail = '',
    this.privacyUrl = '',
  });
  final String title,
      tagline,
      heroTitle,
      description,
      venue,
      city,
      brochureUrl,
      sponsorshipEmail,
      privacyUrl;
  final DateTime startDate, endDate;

  factory EventDetails.fromJson(Map<String, dynamic> json) {
    // Dates are required by the console; a malformed one cannot hide the guide.
    final start = DateTime.tryParse(str(json['start_date']));
    final end = DateTime.tryParse(str(json['end_date'])) ?? start;
    return EventDetails(
      title: switch (str(json['title'])) {
        '' => 'Bio Connect 4.0',
        final title => title,
      },
      tagline: str(json['tagline']),
      heroTitle: str(json['hero_title']),
      description: str(json['description']),
      startDate: start ?? end ?? DateTime(2026, 10, 8),
      endDate: end ?? DateTime(2026, 10, 9),
      venue: str(json['venue']),
      city: str(json['city']),
      brochureUrl: str(json['brochure_url']),
      sponsorshipEmail: str(json['sponsorship_email']),
      privacyUrl: str(json['privacy_url']),
    );
  }
}

class EventTheme {
  const EventTheme({
    required this.title,
    this.description = '',
    this.image = '',
  });

  /// [image] is a bundled asset path or an HTTPS URL.
  final String title, description, image;
  factory EventTheme.fromJson(Map<String, dynamic> json) => EventTheme(
    title: str(json['title']),
    description: str(json['description']),
    image: str(json['image']),
  );
}

class Speaker {
  const Speaker({
    required this.id,
    required this.name,
    this.role = '',
    this.organization = '',
    this.image = '',
    this.linkedin = '',
  });
  final String id, name, role, organization, image, linkedin;
  factory Speaker.fromJson(Map<String, dynamic> json) => Speaker(
    id: str(json['id']),
    name: str(json['name']),
    role: str(json['role']),
    organization: str(json['organization']),
    image: switch (str(json['image_url'])) {
      '' => str(json['image']),
      final url => url,
    },
    linkedin: str(json['linkedin']),
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
