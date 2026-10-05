import 'package:flutter/material.dart';
import 'package:provider/provider.dart';

import '../main.dart';
import 'exhibitors_screen.dart';
import 'my_passes_screen.dart';
import '../widgets/directory.dart';
import '../widgets/interaction.dart';
import '../models/event_content.dart';
import '../models/event_guide.dart';
import '../providers/content_provider.dart';
import 'delegate_registration_screen.dart';

void showGuidePage(BuildContext context, Widget page) =>
    Navigator.of(context).push(MaterialPageRoute<void>(builder: (_) => page));

class AttendeeHomeScreen extends StatelessWidget {
  const AttendeeHomeScreen(
    this.content, {
    super.key,
    required this.sessions,
    required this.speakers,
  });
  final EventContent content;
  final VoidCallback sessions, speakers;
  @override
  Widget build(BuildContext context) => LiveRefresh(
    onRefresh: context.read<ContentProvider>().load,
    child: ListView(
      keyboardDismissBehavior: ScrollViewKeyboardDismissBehavior.onDrag,
      physics: const AlwaysScrollableScrollPhysics(),
      padding: const EdgeInsets.fromLTRB(20, 20, 20, 32),
      children: [
        Container(
          padding: const EdgeInsets.all(24),
          decoration: BoxDecoration(
            color: forest,
            borderRadius: BorderRadius.circular(24),
          ),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              const Text(
                'YOUR CONCLAVE COMPANION',
                style: TextStyle(
                  color: lime,
                  fontSize: 11,
                  letterSpacing: 1.5,
                  fontWeight: FontWeight.bold,
                ),
              ),
              const SizedBox(height: 12),
              const Text(
                'Make the most\nof Bio Connect.',
                style: TextStyle(
                  fontFamily: 'Manrope',
                  color: Colors.white,
                  fontSize: 30,
                  height: 1.15,
                ),
              ),
              const SizedBox(height: 20),
              IconText(
                Icons.calendar_today_outlined,
                eventDateRange(content.event),
              ),
              const SizedBox(height: 10),
              IconText(Icons.place_outlined, content.event.venue),
              const SizedBox(height: 20),
              FilledButton.icon(
                onPressed: sessions,
                style: FilledButton.styleFrom(
                  backgroundColor: lime,
                  foregroundColor: forest,
                ),
                icon: const Icon(Icons.view_agenda_outlined),
                label: const Text('View sessions'),
              ),
            ],
          ),
        ),
        const SizedBox(height: 24),
        const TitleText('Your event, at a glance.'),
        const SizedBox(height: 12),
        LayoutBuilder(
          builder: (context, constraints) {
            final width = constraints.maxWidth > 600
                ? (constraints.maxWidth - 24) / 3
                : (constraints.maxWidth - 12) / 2;
            return Wrap(
              spacing: 12,
              runSpacing: 12,
              children: [
                _QuickLink(
                  Icons.calendar_month_outlined,
                  'Sessions',
                  'Programme & timings',
                  sessions,
                  width,
                ),
                _QuickLink(
                  Icons.place_outlined,
                  'Venue',
                  'Directions & arrival',
                  () => showGuidePage(context, const VenueScreen()),
                  width,
                ),
                _QuickLink(
                  Icons.local_activity_outlined,
                  'Activities',
                  'Discover & connect',
                  () => showGuidePage(context, const ActivitiesScreen()),
                  width,
                ),
                _QuickLink(
                  Icons.help_outline,
                  'FAQs',
                  'Event-day answers',
                  () => showGuidePage(context, const FaqScreen()),
                  width,
                ),
                _QuickLink(
                  Icons.people_outline,
                  'Speakers',
                  'Meet the voices',
                  speakers,
                  width,
                ),
                _QuickLink(
                  Icons.storefront_outlined,
                  'Exhibitors',
                  'Explore the expo',
                  () => showGuidePage(context, const ExhibitorsScreen()),
                  width,
                ),
              ],
            );
          },
        ),
        const SizedBox(height: 24),
        GuideNotice(
          title: content.guide.sessions.isEmpty
              ? 'The programme is taking shape'
              : '${content.guide.sessions.length} sessions to explore',
          message: content.guide.sessions.isEmpty
              ? 'Session timings and event-day details will appear here as they are announced. Pull down to check for updates.'
              : 'Browse the published programme, with timings shown in India time (IST).',
          icon: Icons.campaign_outlined,
        ),
        const SizedBox(height: 16),
        GuideCard(
          Icons.confirmation_number_outlined,
          'My passes',
          'View your admission QR on this phone',
          () => showGuidePage(context, const MyPassesScreen()),
        ),
        GuideCard(
          Icons.confirmation_number_outlined,
          'Registration & passes',
          'Register or review delegate options',
          () => showGuidePage(context, RegistrationScreen(content.event)),
        ),
        GuideCard(
          Icons.explore_outlined,
          'Explore Bio Connect',
          'Themes, ideas and programme highlights',
          () => showGuidePage(
            context,
            Scaffold(
              appBar: AppBar(title: const Text('Explore Bio Connect')),
              body: const ExploreScreen(),
            ),
          ),
        ),
      ],
    ),
  );
}

class _QuickLink extends StatelessWidget {
  const _QuickLink(
    this.icon,
    this.title,
    this.subtitle,
    this.onTap,
    this.width,
  );
  final IconData icon;
  final String title, subtitle;
  final VoidCallback onTap;
  final double width;
  @override
  Widget build(BuildContext context) => SizedBox(
    width: width,
    child: Material(
      color: cream,
      borderRadius: BorderRadius.circular(18),
      child: InkWell(
        onTap: onTap,
        borderRadius: BorderRadius.circular(18),
        child: Padding(
          padding: const EdgeInsets.all(16),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Icon(icon, color: forest, size: 26),
              const SizedBox(height: 16),
              Text(
                title,
                style: const TextStyle(fontFamily: 'Manrope', fontSize: 16),
              ),
              const SizedBox(height: 4),
              Text(
                subtitle,
                style: const TextStyle(color: muted, fontSize: 12),
              ),
            ],
          ),
        ),
      ),
    ),
  );
}

class GuideNotice extends StatelessWidget {
  const GuideNotice({
    super.key,
    required this.title,
    required this.message,
    this.icon = Icons.schedule,
  });
  final String title, message;
  final IconData icon;
  @override
  Widget build(BuildContext context) => Container(
    padding: const EdgeInsets.all(20),
    decoration: BoxDecoration(
      color: cream,
      borderRadius: BorderRadius.circular(18),
    ),
    child: Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Icon(icon, color: forest),
        const SizedBox(height: 12),
        Text(
          title,
          style: const TextStyle(fontFamily: 'Manrope', fontSize: 18),
        ),
        const SizedBox(height: 8),
        Text(message, style: const TextStyle(color: muted, height: 1.5)),
      ],
    ),
  );
}

class SessionsScreen extends StatefulWidget {
  const SessionsScreen({super.key});
  @override
  State<SessionsScreen> createState() => _SessionsScreenState();
}

class _SessionsScreenState extends State<SessionsScreen> {
  String query = '', selectedDay = 'All days';
  @override
  Widget build(BuildContext context) {
    final state = context.watch<ContentProvider>();
    final sessions = [...state.content!.guide.sessions]
      ..sort((a, b) {
        if (a.startsAt == null) {
          return b.startsAt == null ? a.title.compareTo(b.title) : 1;
        }
        if (b.startsAt == null) return -1;
        return a.startsAt!.compareTo(b.startsAt!);
      });
    final days = sessions
        .where((s) => s.startsAt != null)
        .map((s) => sessionDay(s.startsAt!))
        .toSet()
        .toList();
    final activeDay = days.contains(selectedDay) ? selectedDay : 'All days';
    final filtered = sessions
        .where(
          (s) =>
              (activeDay == 'All days' ||
                  (s.startsAt != null &&
                      sessionDay(s.startsAt!) == activeDay)) &&
              '${s.title} ${s.speakers} ${s.location}'.toLowerCase().contains(
                query.toLowerCase(),
              ),
        )
        .toList();
    return LiveRefresh(
      onRefresh: state.load,
      child: ListView(
        keyboardDismissBehavior: ScrollViewKeyboardDismissBehavior.onDrag,
        physics: const AlwaysScrollableScrollPhysics(),
        padding: const EdgeInsets.fromLTRB(20, 24, 20, 30),
        children: [
          const Eyebrow('THE PROGRAMME'),
          const SizedBox(height: 8),
          const TitleText('Find your next\nconversation.'),
          const SizedBox(height: 10),
          const Text(
            'Sessions, speakers and places to be. All times are in IST.',
            style: TextStyle(color: muted),
          ),
          const SizedBox(height: 24),
          if (sessions.isEmpty) ...[
            const GuideNotice(
              title: 'Session timetable to be announced',
              message: 'The confirmed programme, session times and halls will appear here. Pull down to check for updates.',
            ),
            const SizedBox(height: 20),
            OutlinedButton.icon(
              onPressed: () =>
                  openLink(context, state.content!.event.brochureUrl),
              icon: const Icon(Icons.article_outlined),
              label: const Text('View event brochure'),
            ),
          ] else ...[
            GuideSearchField(
              label: 'Search sessions, speakers or halls',
              onChanged: (v) => setState(() => query = v),
            ),
            const SizedBox(height: 12),
            Wrap(
              spacing: 8,
              children: [
                for (final day in ['All days', ...days])
                  ChoiceChip(
                    label: Text(day),
                    selected: activeDay == day,
                    onSelected: (_) {
                      if (selectedDay != day) {
                        AppFeedback.selection();
                        FocusScope.of(context).unfocus();
                        setState(() => selectedDay = day);
                      }
                    },
                  ),
              ],
            ),
            const SizedBox(height: 16),
            if (filtered.isEmpty)
              const GuideNotice(
                title: 'No matching sessions',
                message: 'Try a different search or choose another day.',
                icon: Icons.search_off,
              ),
            for (final session in filtered)
              Card(
                color: cream,
                child: ListTile(
                  contentPadding: const EdgeInsets.all(16),
                  title: Text(
                    session.title,
                    style: const TextStyle(fontFamily: 'Manrope'),
                  ),
                  subtitle: Padding(
                    padding: const EdgeInsets.only(top: 8),
                    child: Text(
                      '${session.startsAt == null ? 'Time to be announced' : '${sessionDay(session.startsAt!)} · ${sessionTime(session.startsAt!)} IST'}\n${session.location.isEmpty ? 'Hall to be announced' : session.location}',
                    ),
                  ),
                  trailing: const Icon(Icons.chevron_right),
                  onTap: () =>
                      showGuidePage(context, SessionDetailScreen(session.id)),
                ),
              ),
          ],
        ],
      ),
    );
  }
}

class SessionDetailScreen extends StatelessWidget {
  const SessionDetailScreen(this.id, {super.key});
  final String id;
  @override
  Widget build(BuildContext context) {
    final state = context.watch<ContentProvider>();
    final items = state.content!.guide.sessions.where((s) => s.id == id);
    return Scaffold(
      appBar: AppBar(title: const Text('Session details')),
      body: LiveRefresh(
        onRefresh: state.load,
        child: items.isEmpty
            ? ListView(
                physics: const AlwaysScrollableScrollPhysics(),
                padding: const EdgeInsets.all(20),
                children: const [
                  GuideNotice(
                    title: 'This session is no longer published',
                    message: 'It may have been moved or withdrawn. Check the programme for the latest timetable.',
                    icon: Icons.event_busy_outlined,
                  ),
                ],
              )
            : Builder(
                builder: (context) {
                  final s = items.first;
                  return ListView(
                    keyboardDismissBehavior:
                        ScrollViewKeyboardDismissBehavior.onDrag,
                    physics: const AlwaysScrollableScrollPhysics(),
                    padding: const EdgeInsets.all(20),
                    children: [
                      TitleText(s.title),
                      const SizedBox(height: 20),
                      _DetailBlock(
                        'When',
                        s.startsAt == null
                            ? 'To be announced'
                            : '${sessionDay(s.startsAt!)}\n${sessionTime(s.startsAt!)}${s.endsAt == null ? '' : ' - ${sessionTime(s.endsAt!)}'} IST',
                      ),
                      _DetailBlock(
                        'Where',
                        s.location.isEmpty ? 'To be announced' : s.location,
                      ),
                      _DetailBlock(
                        'Speakers',
                        s.speakers.isEmpty ? 'To be announced' : s.speakers,
                      ),
                      _DetailBlock(
                        'About the session',
                        s.description.isEmpty
                            ? 'Details to be announced'
                            : s.description,
                      ),
                    ],
                  );
                },
              ),
      ),
    );
  }
}

class VenueScreen extends StatelessWidget {
  const VenueScreen({super.key});
  @override
  Widget build(BuildContext context) {
    final state = context.watch<ContentProvider>();
    final event = state.content!.event, venue = state.content!.guide.venue;
    return Scaffold(
      appBar: AppBar(title: const Text('Venue & directions')),
      body: LiveRefresh(
        onRefresh: state.load,
        child: ListView(
          keyboardDismissBehavior: ScrollViewKeyboardDismissBehavior.onDrag,
          physics: const AlwaysScrollableScrollPhysics(),
          padding: const EdgeInsets.all(20),
          children: [
            const Eyebrow('GETTING HERE'),
            const SizedBox(height: 8),
            TitleText(event.venue),
            const SizedBox(height: 10),
            SelectableText(venue.address.isEmpty ? event.city : venue.address),
            const SizedBox(height: 18),
            FilledButton.icon(
              onPressed: () => openLink(context, directionsUrl(event)),
              icon: const Icon(Icons.directions_outlined),
              label: const Text('Open directions'),
            ),
            const SizedBox(height: 24),
            _DetailBlock(
              'Arrival & check-in',
              venue.arrival.isEmpty
                  ? 'Arrival, parking and check-in details to be announced.'
                  : venue.arrival,
            ),
            _DetailBlock(
              'Accessibility',
              venue.accessibility.isEmpty
                  ? 'Venue accessibility details to be announced.'
                  : venue.accessibility,
            ),
            // Shown only once the organisers publish a floor plan.
            if (venue.floorPlanUrl.isNotEmpty)
              OutlinedButton.icon(
                onPressed: () => openLink(context, venue.floorPlanUrl),
                icon: const Icon(Icons.map_outlined),
                label: const Text('Open venue floor plan'),
              ),
            const SizedBox(height: 20),
            _HelpDesk(venue),
          ],
        ),
      ),
    );
  }
}

class ActivitiesScreen extends StatelessWidget {
  const ActivitiesScreen({super.key});
  @override
  Widget build(BuildContext context) {
    final state = context.watch<ContentProvider>();
    final content = state.content!;
    return Scaffold(
      appBar: AppBar(title: const Text('Activities')),
      body: LiveRefresh(
        onRefresh: state.load,
        child: ListView(
          keyboardDismissBehavior: ScrollViewKeyboardDismissBehavior.onDrag,
          physics: const AlwaysScrollableScrollPhysics(),
          padding: const EdgeInsets.all(20),
          children: [
            const Eyebrow('BEYOND THE SESSIONS'),
            const SizedBox(height: 8),
            const TitleText('Discover. Meet. Connect.'),
            const SizedBox(height: 20),
            if (content.guide.activities.isEmpty) ...[
              const GuideNotice(
                title: 'Activity details to be announced',
                message: 'Times, locations and participation details will appear here as they are confirmed.',
                icon: Icons.local_activity_outlined,
              ),
              const SizedBox(height: 24),
              const Text(
                'Programme highlights',
                style: TextStyle(fontFamily: 'Manrope', fontSize: 18),
              ),
              const SizedBox(height: 8),
              for (final title in content.programmeHighlights)
                ListTile(
                  contentPadding: EdgeInsets.zero,
                  leading: const Icon(Icons.arrow_outward, color: forest),
                  title: Text(title),
                ),
            ] else
              for (final a in content.guide.activities)
                Card(
                  color: cream,
                  child: Padding(
                    padding: const EdgeInsets.all(18),
                    child: Column(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: [
                        Text(
                          a.title,
                          style: const TextStyle(
                            fontFamily: 'Manrope',
                            fontSize: 18,
                          ),
                        ),
                        if (a.description.isNotEmpty) ...[
                          const SizedBox(height: 10),
                          Text(a.description),
                        ],
                        const SizedBox(height: 14),
                        Text(
                          a.schedule.isEmpty
                              ? 'Schedule to be announced'
                              : a.schedule,
                          style: const TextStyle(color: muted),
                        ),
                        const SizedBox(height: 6),
                        Text(
                          a.location.isEmpty
                              ? 'Location to be announced'
                              : a.location,
                          style: const TextStyle(color: muted),
                        ),
                      ],
                    ),
                  ),
                ),
          ],
        ),
      ),
    );
  }
}

class FaqScreen extends StatefulWidget {
  const FaqScreen({super.key});
  @override
  State<FaqScreen> createState() => _FaqScreenState();
}

class _FaqScreenState extends State<FaqScreen> {
  String query = '';
  @override
  Widget build(BuildContext context) {
    final state = context.watch<ContentProvider>();
    final guide = state.content!.guide;
    final items = guide.faqs.where(
      (f) => '${f.question} ${f.answer}'.toLowerCase().contains(
        query.toLowerCase(),
      ),
    );
    return Scaffold(
      appBar: AppBar(title: const Text('FAQs')),
      body: LiveRefresh(
        onRefresh: state.load,
        child: ListView(
          keyboardDismissBehavior: ScrollViewKeyboardDismissBehavior.onDrag,
          physics: const AlwaysScrollableScrollPhysics(),
          padding: const EdgeInsets.all(20),
          children: [
            const Eyebrow('A LITTLE HELP'),
            const SizedBox(height: 8),
            const TitleText('Good to know.'),
            const SizedBox(height: 20),
            if (guide.faqs.isEmpty)
              const GuideNotice(
                title: 'Event-day FAQs to be announced',
                message: 'Answers about entry, facilities and attending the conclave will appear here once confirmed.',
                icon: Icons.help_outline,
              )
            else ...[
              GuideSearchField(
                label: 'Search FAQs',
                onChanged: (v) => setState(() => query = v),
              ),
              const SizedBox(height: 16),
              if (items.isEmpty)
                const GuideNotice(
                  title: 'No matching answers',
                  message: 'Try another search term.',
                  icon: Icons.search_off,
                ),
              for (final f in items)
                Card(
                  child: ExpansionTile(
                    onExpansionChanged: (_) => AppFeedback.selection(),
                    key: ValueKey(f.id),
                    title: Text(f.question),
                    childrenPadding: const EdgeInsets.fromLTRB(16, 0, 16, 20),
                    children: [
                      Align(
                        alignment: Alignment.centerLeft,
                        child: Text(f.answer),
                      ),
                    ],
                  ),
                ),
            ],
            const SizedBox(height: 24),
            _HelpDesk(guide.venue),
            const SizedBox(height: 16),
            OutlinedButton.icon(
              onPressed: () =>
                  showGuidePage(context, const DelegateRegistrationScreen()),
              icon: const Icon(Icons.confirmation_number_outlined),
              label: const Text('Register as a delegate'),
            ),
          ],
        ),
      ),
    );
  }
}

class _DetailBlock extends StatelessWidget {
  const _DetailBlock(this.title, this.body);
  final String title, body;
  @override
  Widget build(BuildContext context) => Padding(
    padding: const EdgeInsets.only(bottom: 24),
    child: Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Text(
          title,
          style: const TextStyle(fontFamily: 'Manrope', fontSize: 18),
        ),
        const SizedBox(height: 8),
        Text(body, style: const TextStyle(color: muted, height: 1.5)),
      ],
    ),
  );
}

class _HelpDesk extends StatelessWidget {
  const _HelpDesk(this.venue);
  final VenueGuide venue;
  @override
  Widget build(BuildContext context) => Column(
    crossAxisAlignment: CrossAxisAlignment.start,
    children: [
      const Text(
        'Need a hand?',
        style: TextStyle(fontFamily: 'Manrope', fontSize: 18),
      ),
      const SizedBox(height: 8),
      if (venue.helpEmail.isEmpty && venue.helpPhone.isEmpty)
        const Text(
          'Event-day help desk contacts to be announced.',
          style: TextStyle(color: muted),
        ),
      if (venue.helpEmail.isNotEmpty)
        ListTile(
          contentPadding: EdgeInsets.zero,
          leading: const Icon(Icons.mail_outline),
          title: Text(venue.helpEmail),
          onTap: () => openLink(
            context,
            Uri(scheme: 'mailto', path: venue.helpEmail).toString(),
          ),
        ),
      if (venue.helpPhone.isNotEmpty)
        ListTile(
          contentPadding: EdgeInsets.zero,
          leading: const Icon(Icons.phone_outlined),
          title: Text(venue.helpPhone),
          onTap: () => openLink(
            context,
            Uri(scheme: 'tel', path: venue.helpPhone).toString(),
          ),
        ),
    ],
  );
}
