import 'dart:convert';
import 'dart:io';

import 'package:bio_connect_app/models/event_content.dart';
import 'package:bio_connect_app/services/content_service.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  test(
    'refreshes all mutable content and resolves exhibitor logo URLs',
    () async {
      final server = await HttpServer.bind(InternetAddress.loopbackIPv4, 0);
      addTearDown(server.close);
      server.listen((request) async {
        request.response.headers.contentType = ContentType.json;
        if (request.uri.path.endsWith('/app-content')) {
          request.response.write(
            jsonEncode({
              'event': {
                'title': 'Updated event',
                'tagline': 'Tagline',
                'hero_title': 'Updated hero',
                'description': 'Description',
                'start_date': '2026-10-08',
                'end_date': '2026-10-09',
                'venue': 'Updated venue',
                'city': 'City',
                'brochure_url': 'https://example.com/brochure.pdf',
                'sponsorship_email': 'team@example.com',
              },
              'themes': [
                {
                  'title': 'Theme',
                  'description': 'New theme',
                  'image': 'asset',
                },
              ],
              'speakers': [
                {
                  'id': 'speaker',
                  'name': 'New Speaker',
                  'role': 'Role',
                  'organization': 'Organisation',
                  'image_url': 'https://example.com/speaker.webp',
                  'linkedin': '',
                },
              ],
              'event_guide': {
                'sessions': [
                  {
                    'id': 's',
                    'title': 'Published session',
                    'starts_at': '2026-10-08T09:00:00+05:30',
                    'ends_at': '2026-10-08T10:00:00+05:30',
                  },
                ],
                'activities': [
                  {'id': 'a', 'title': 'Networking'},
                ],
                'faqs': [
                  {'id': 'f', 'question': 'Where?', 'answer': 'Foyer'},
                ],
                'venue': {'arrival': 'Use the main entrance'},
              },
              'programme_highlights': ['New highlight'],
              'product_launch': {
                'eyebrow': 'Host',
                'title': 'New launch',
                'description': 'Description',
                'deadline': 'Tomorrow',
                'eligibility': [
                  {'title': 'Teams', 'description': 'All teams'},
                ],
                'focus_areas': ['Health'],
                'apply_url': 'https://example.com/apply',
              },
              'leadership': {
                'intro': 'New intro',
                'advisory_note': 'New note',
                'people': [
                  {'name': 'Leader', 'role': 'Role', 'badge': 'Badge'},
                ],
              },
              'sponsor': {
                'name': 'New sponsor',
                'description': 'Sponsor description',
                'logo_url': 'https://example.com/sponsor.png',
                'website_url': 'https://example.com',
              },
              'ecosystem_partners': [
                {'name': 'Partner', 'logo_url': 'https://example.com/logo.png'},
              ],
            }),
          );
        } else {
          request.response.write(
            jsonEncode({
              'exhibitors': [
                {
                  'name': 'Expo Labs',
                  'description': 'Diagnostics',
                  'logo_url': '/api/v1/public/exhibitors/logos/logo-id',
                },
              ],
            }),
          );
        }
        await request.response.close();
      });
      final baseUrl = 'http://127.0.0.1:${server.port}';
      final service = CurrentContentService(apiBaseUrl: baseUrl);
      final bundled = EventContent.fromJson(
        jsonDecode(await File('assets/content/event.json').readAsString())
            as Map<String, dynamic>,
      );

      final refreshed = await service.refreshEvent(bundled);
      expect(refreshed.event.title, 'Updated event');
      expect(refreshed.guide.sessions.single.title, 'Published session');
      expect(refreshed.guide.activities.single.title, 'Networking');
      expect(refreshed.guide.faqs.single.answer, 'Foyer');
      expect(refreshed.guide.venue.arrival, 'Use the main entrance');
      expect(refreshed.speakers.single.name, 'New Speaker');
      expect(refreshed.productLaunch.title, 'New launch');
      expect(refreshed.sponsor.name, 'New sponsor');
      expect(refreshed.ecosystemPartners.single.name, 'Partner');

      final exhibitors = await service.loadExhibitors();
      expect(
        exhibitors.single.logoUrl,
        '$baseUrl/api/v1/public/exhibitors/logos/logo-id',
      );
    },
  );
}
