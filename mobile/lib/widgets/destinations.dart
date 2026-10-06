import 'package:flutter/foundation.dart';
import 'package:flutter/material.dart';

import '../main.dart';
import '../models/event_content.dart';
import '../screens/agenda_screen.dart';
import '../screens/contacts_screen.dart';
import '../screens/event_guide_screens.dart';
import '../screens/exhibitors_screen.dart';
import '../screens/feedback_screen.dart';
import '../screens/hub_screen.dart';
import '../screens/my_passes_screen.dart';
import '../screens/moments_screen.dart';
import '../screens/partners_screens.dart';
import '../screens/selfie_frame_screen.dart';

/// Lets menu entries switch to a bottom tab instead of pushing a page.
class TabScope extends InheritedWidget {
  const TabScope({super.key, required this.select, required super.child});

  /// Selects the tab for a destination key; false when it is not a tab.
  final bool Function(String key) select;

  static TabScope? maybeOf(BuildContext context) =>
      context.dependOnInheritedWidgetOfExactType<TabScope>();

  @override
  bool updateShouldNotify(TabScope oldWidget) => select != oldWidget.select;
}

const _icons = <String, IconData>{
  'sessions': Icons.calendar_month_outlined,
  'speakers': Icons.people_outline,
  'agenda': Icons.bookmarks_outlined,
  'contacts': Icons.qr_code_scanner_rounded,
  'feedback': Icons.rate_review_outlined,
  'hub': Icons.waving_hand_outlined,
  'venue': Icons.place_outlined,
  'activities': Icons.local_activity_outlined,
  'faqs': Icons.help_outline,
  'exhibitors': Icons.storefront_outlined,
  'moments': Icons.photo_library_outlined,
  'selfie_frame': Icons.add_a_photo_outlined,
  'my_passes': Icons.confirmation_number_outlined,
  'registration': Icons.how_to_reg_outlined,
  'brochure': Icons.article_outlined,
  'product_launch': Icons.rocket_launch_outlined,
  'sponsors': Icons.handshake_outlined,
  'leadership': Icons.account_balance_outlined,
  'explore': Icons.explore_outlined,
  'privacy': Icons.privacy_tip_outlined,
};

/// The default label for an entry whose title staff left blank.
const _titles = <String, String>{
  'sessions': 'Sessions',
  'speakers': 'Speakers',
  'agenda': 'My agenda',
  'contacts': 'Contacts',
  'feedback': 'Share feedback',
  'hub': 'After Bio Connect',
  'venue': 'Venue & directions',
  'activities': 'Activities',
  'faqs': 'FAQs',
  'exhibitors': 'Exhibitors',
  'moments': 'Moments album',
  'selfie_frame': 'Selfie frame',
  'my_passes': 'My passes',
  'registration': 'Registration & passes',
  'brochure': 'Event brochure',
  'product_launch': 'Product launch',
  'sponsors': 'Sponsors',
  'leadership': 'Leadership',
  'explore': 'Explore Bio Connect',
  'privacy': 'Privacy policy',
};

IconData entryIcon(MenuEntry entry) => switch (entry.key) {
  'link' when entry.url.startsWith('tel:') => Icons.call_outlined,
  'link' when entry.url.startsWith('mailto:') => Icons.mail_outline,
  'link' => Icons.open_in_new,
  final key => _icons[key] ?? Icons.chevron_right,
};

String entryTitle(MenuEntry entry) =>
    entry.title.isNotEmpty ? entry.title : _titles[entry.key] ?? '';

String entrySubtitle(MenuEntry entry, EventContent content) {
  if (entry.subtitle.isNotEmpty) return entry.subtitle;
  return switch (entry.key) {
    'venue' => content.event.venue,
    'sponsors' when content.sponsors.length > 1 =>
      '${content.sponsors.length} sponsors and ecosystem partners',
    'sponsors' => 'Sponsors and ecosystem partners',
    _ => '',
  };
}

/// Whether a destination has anything to show. Entries without content are
/// left out of every menu, so staff never need to hide an empty page.
bool hasContent(String key, EventContent c, {String url = ''}) => switch (key) {
  'sessions' => c.guide.sessions.isNotEmpty,
  'speakers' => c.speakers.isNotEmpty,
  'feedback' => c.feedback.open,
  'venue' =>
    c.event.venue.isNotEmpty ||
        c.event.city.isNotEmpty ||
        c.guide.venue.address.isNotEmpty,
  'activities' => c.guide.activities.isNotEmpty,
  'faqs' => c.guide.faqs.isNotEmpty || c.guide.venue.hasHelp,
  'moments' => c.event.momentsAlbumId.isNotEmpty,
  'brochure' => c.event.brochureUrl.isNotEmpty,
  'product_launch' =>
    c.productLaunch.title.isNotEmpty || c.productLaunch.description.isNotEmpty,
  'sponsors' => c.sponsors.isNotEmpty || c.ecosystemPartners.isNotEmpty,
  'leadership' => !c.leadership.isEmpty,
  'explore' => c.themes.isNotEmpty || c.programmeHighlights.isNotEmpty,
  'privacy' => c.event.privacyUrl.isNotEmpty,
  'link' => url.isNotEmpty,
  _ => true,
};

List<MenuEntry> visibleMenu(EventContent content, String name) {
  final configured = content.menu(name);
  final entries =
      kDebugMode &&
          name == 'guide' &&
          !configured.any((entry) => entry.key == 'moments')
      ? [
          ...configured,
          const MenuEntry(
            'moments',
            'Moments album',
            'Preview your private event photos',
          ),
        ]
      : configured;
  return [
    for (final entry in entries)
      if (entryTitle(entry).isNotEmpty &&
          (hasContent(entry.key, content, url: entry.url) ||
              kDebugMode && entry.key == 'moments'))
        entry,
  ];
}

const tabKeys = {'sessions', 'speakers', 'agenda'};

/// Bottom tabs staff have published, in order, by destination key.
List<MenuEntry> visibleTabs(EventContent content) => [
  for (final entry in content.menu('tabs'))
    if (tabKeys.contains(entry.key)) entry,
];

void openDestination(
  BuildContext context,
  EventContent content,
  MenuEntry entry,
) {
  if (TabScope.maybeOf(context)?.select(entry.key) ?? false) return;
  final page = switch (entry.key) {
    'sessions' => Scaffold(
      appBar: AppBar(title: Text(entryTitle(entry))),
      body: const SessionsScreen(),
    ),
    'speakers' => Scaffold(
      appBar: AppBar(title: Text(entryTitle(entry))),
      body: SpeakersScreen(content.speakers),
    ),
    'agenda' => AgendaPage(title: entryTitle(entry)),
    'contacts' => ContactsScreen(title: entryTitle(entry)),
    'feedback' => FeedbackScreen(title: entryTitle(entry)),
    'hub' => AfterEventScreen(title: entryTitle(entry)),
    'venue' => const VenueScreen(),
    'activities' => const ActivitiesScreen(),
    'faqs' => const FaqScreen(),
    'exhibitors' => const ExhibitorsScreen(),
    'moments' => const MomentsScreen(),
    'selfie_frame' => SelfieFrameScreen(
      content.event,
      title: entryTitle(entry),
    ),
    'my_passes' => const MyPassesScreen(),
    'registration' => RegistrationScreen(content.event),
    'product_launch' => const ProductLaunchScreen(),
    'sponsors' => const SponsorsScreen(),
    'leadership' => const LeadershipScreen(),
    'explore' => Scaffold(
      appBar: AppBar(title: Text(entryTitle(entry))),
      body: const ExploreScreen(),
    ),
    _ => null,
  };
  if (page != null) {
    showGuidePage(context, page);
    return;
  }
  final url = switch (entry.key) {
    'brochure' => content.event.brochureUrl,
    'privacy' => content.event.privacyUrl,
    _ => entry.url,
  };
  if (url.isNotEmpty) openLink(context, url);
}
