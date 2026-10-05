import 'dart:convert';
import 'dart:io';

import 'package:bio_connect_app/main.dart';
import 'package:bio_connect_app/models/event_content.dart';
import 'package:bio_connect_app/providers/content_provider.dart';
import 'package:bio_connect_app/screens/event_guide_screens.dart';
import 'package:bio_connect_app/services/content_service.dart';
import 'package:bio_connect_app/widgets/destinations.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:bio_connect_app/widgets/nav_bar.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:provider/provider.dart';

// Synchronous: real file IO never completes inside testWidgets' fake clock.
Map<String, dynamic> bundled() =>
    jsonDecode(File('assets/content/event.json').readAsStringSync())
        as Map<String, dynamic>;

Future<ContentProvider> pumpApp(
  WidgetTester tester,
  Map<String, dynamic> json,
) async {
  final provider = ContentProvider(CurrentContentService(remoteContent: false))
    ..content = EventContent.fromJson(json);
  addTearDown(provider.dispose);
  await tester.pumpWidget(
    ChangeNotifierProvider.value(value: provider, child: const BioConnectApp()),
  );
  await tester.pumpAndSettle();
  return provider;
}

void main() {
  setUp(() => rootBundle.clear());

  test('malformed or partial content never breaks parsing', () {
    final content = EventContent.fromJson({
      'event': {'title': 7, 'start_date': 'soon'},
      'themes': 'none',
      'speakers': [
        {'id': 'a', 'name': 'Named'},
        {'id': 'b'},
        'junk',
      ],
      'leadership': {'convened_by': null, 'people': null},
      'sponsors': [
        {'name': ''},
        {'name': 'Sponsor', 'logo_url': null},
      ],
      'menus': {
        'guide': [
          {'key': 'faqs'},
          {'key': 'unknown'},
        ],
        'drawer': [],
      },
      'copy': {'home.title': 'Hello', 'bad': 3},
      'event_guide': {
        'sessions': [
          {'id': 's', 'title': 'Talk', 'starts_at': 12},
          {'title': 'No id'},
        ],
        'faqs': [
          {'id': 'f', 'question': 'Unanswered', 'answer': ''},
        ],
      },
    });
    expect(content.event.title, 'Bio Connect 4.0');
    expect(content.event.momentsAlbumId, isEmpty);
    expect(content.themes, isEmpty);
    expect(content.speakers.single.name, 'Named');
    expect(content.leadership.convenedBy, isNull);
    expect(content.sponsors.single.logoUrl, isEmpty);
    expect(content.menu('guide').single.key, 'faqs');
    expect(content.menus.containsKey('drawer'), isFalse);
    // Menus not published keep the built-in layout, now with My agenda.
    expect(content.menu('tabs').map((e) => e.key), [
      'sessions',
      'speakers',
      'agenda',
    ]);
    expect(content.text('home.title', 'x'), 'Hello');
    expect(content.text('bad', 'fallback'), 'fallback');
    expect(content.guide.sessions.single.startsAt, isNull);
    expect(content.guide.faqs, isEmpty);
  });

  test('an empty published sponsors list stays empty', () async {
    final json = bundled()..['sponsors'] = [];
    expect(EventContent.fromJson(json).sponsors, isEmpty);
  });

  test('Moments is offered only for a configured positive album', () {
    final json = bundled();
    json['menus'] = {
      'guide': [
        {'key': 'moments', 'title': 'Moments album'},
      ],
    };
    json['event'] = {...json['event'] as Map, 'moments_album_id': '42'};
    final configured = EventContent.fromJson(json);
    expect(configured.event.momentsAlbumId, '42');
    expect(visibleMenu(configured, 'guide').single.key, 'moments');

    json['event'] = {...json['event'] as Map, 'moments_album_id': '0'};
    final invalid = EventContent.fromJson(json);
    expect(invalid.event.momentsAlbumId, isEmpty);
    expect(hasContent('moments', invalid), isFalse);
  });

  testWidgets('staff control tabs, menus and headings', (tester) async {
    final json = bundled();
    json['menus'] = {
      'tabs': [
        {'key': 'sessions', 'title': 'Agenda'},
      ],
      'home_shortcuts': [
        {'key': 'speakers', 'title': 'Our speakers', 'subtitle': 'Meet them'},
      ],
      'home_links': [],
      'guide': [
        {
          'key': 'link',
          'title': 'Call the help desk',
          'url': 'tel:+914710000000',
        },
        {'key': 'link', 'title': 'Missing address'},
        {'key': 'activities', 'title': 'Nothing published'},
        {'key': 'leadership'},
      ],
    };
    json['copy'] = {
      'home.title': 'Welcome to day two.',
      'home.notice_title': 'Hall B is now open',
      'guide.title': 'Your toolkit',
      'speakers.title': 'On stage',
    };
    await pumpApp(tester, json);

    final bar = tester.widget<BioNavBar>(find.byType(BioNavBar));
    expect(bar.items.map((d) => d.label), ['Home', 'Agenda', 'Guide']);
    expect(find.text('Welcome to day two.'), findsOneWidget);
    await tester.scrollUntilVisible(
      find.text('Hall B is now open'),
      200,
      scrollable: find.byType(Scrollable).first,
    );
    expect(find.text('Hall B is now open'), findsOneWidget);
    expect(find.text('Explore Bio Connect'), findsNothing);
    await tester.scrollUntilVisible(
      find.text('Our speakers'),
      -200,
      scrollable: find.byType(Scrollable).first,
    );

    // A shortcut to a hidden tab opens the screen as a page instead.
    await tester.tap(find.text('Our speakers'));
    await tester.pumpAndSettle();
    expect(find.text('On stage'), findsOneWidget);
    await tester.pageBack();
    await tester.pumpAndSettle();

    await tester.tap(find.text('Guide').last);
    await tester.pumpAndSettle();
    expect(find.text('Your toolkit'), findsOneWidget);
    expect(find.text('Call the help desk'), findsOneWidget);
    expect(find.text('Missing address'), findsNothing);
    expect(find.text('Nothing published'), findsNothing);
    expect(find.text('Leadership'), findsOneWidget);
    expect(find.text('Event brochure'), findsNothing);
    expect(tester.takeException(), isNull);
  });

  testWidgets('venue and help show only published details', (tester) async {
    final json = bundled();
    json['event_guide'] = {
      'venue': {'arrival': 'Use gate 2.', 'help_whatsapp': '+91 98470 00000'},
    };
    json['event'] = {...json['event'] as Map, 'sponsorship_email': ''};
    await pumpApp(tester, json);
    await tester.tap(find.text('Guide').last);
    await tester.pumpAndSettle();
    // Help contacts alone make the FAQ page worth offering.
    expect(find.text('FAQs'), findsOneWidget);
    await tester.tap(find.text('Venue & directions'));
    await tester.pumpAndSettle();
    expect(find.text('Use gate 2.'), findsOneWidget);
    expect(find.text('Accessibility'), findsNothing);
    await tester.scrollUntilVisible(
      find.text('WhatsApp +91 98470 00000'),
      200,
      scrollable: find.byType(Scrollable).last,
    );
    expect(find.text('WhatsApp +91 98470 00000'), findsOneWidget);
    expect(find.byIcon(Icons.phone_outlined), findsNothing);
    await tester.pageBack();
    await tester.pumpAndSettle();
    final guide = find.byType(Scrollable).last;
    await tester.scrollUntilVisible(
      find.text('Sponsors'),
      200,
      scrollable: guide,
    );
    await tester.tap(find.text('Sponsors'));
    await tester.pumpAndSettle();
    // No organiser email, so no enquiry button.
    expect(find.text('Enquire about sponsorship'), findsNothing);
    expect(tester.takeException(), isNull);
  });

  testWidgets('session details omit unknown time, hall and speakers', (
    tester,
  ) async {
    final json = bundled();
    json['event_guide'] = {
      'sessions': [
        {'id': 'x', 'title': 'Keynote'},
      ],
    };
    await pumpApp(tester, json);
    await tester.tap(find.text('Sessions').last);
    await tester.pumpAndSettle();
    await tester.tap(find.text('Keynote'));
    await tester.pumpAndSettle();
    expect(find.byType(SessionDetailScreen), findsOneWidget);
    for (final text in ['When', 'Where', 'Speakers', 'About the session']) {
      expect(
        find.descendant(
          of: find.byType(SessionDetailScreen),
          matching: find.text(text),
        ),
        findsNothing,
      );
    }
  });
}
