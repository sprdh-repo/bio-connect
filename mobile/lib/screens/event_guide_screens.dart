import 'package:flutter/material.dart';
import 'package:provider/provider.dart';

import '../main.dart';
import '../widgets/destinations.dart';
import '../widgets/directory.dart';
import '../widgets/interaction.dart';
import '../models/event_content.dart';
import '../models/event_guide.dart';
import '../providers/content_provider.dart';
import 'delegate_registration_screen.dart';

void showGuidePage(BuildContext context, Widget page) =>
    Navigator.of(context).push(MaterialPageRoute<void>(builder: (_) => page));

class AttendeeHomeScreen extends StatelessWidget {
  const AttendeeHomeScreen(this.content, {super.key});
  final EventContent content;
  @override
  Widget build(BuildContext context) {
    final sessions = MenuEntry(
      'sessions',
      content.text('home.button', 'View sessions'),
    );
    final shortcuts = visibleMenu(content, 'home_shortcuts');
    final links = visibleMenu(content, 'home_links');
    final noticeTitle = content.override('home.notice_title');
    final noticeMessage = content.override('home.notice_message');
    final count = content.guide.sessions.length;
    return LiveRefresh(
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
                Text(
                  content.text('home.eyebrow', 'YOUR CONCLAVE COMPANION'),
                  style: const TextStyle(
                    color: lime,
                    fontSize: 11,
                    letterSpacing: 1.5,
                    fontWeight: FontWeight.bold,
                  ),
                ),
                const SizedBox(height: 12),
                Text(
                  content.text('home.title', 'Make the most\nof Bio Connect.'),
                  style: const TextStyle(
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
                if (content.event.venue.isNotEmpty) ...[
                  const SizedBox(height: 10),
                  IconText(Icons.place_outlined, content.event.venue),
                ],
                if (visibleTabs(content).any((t) => t.key == 'sessions') ||
                    count > 0) ...[
                  const SizedBox(height: 20),
                  FilledButton.icon(
                    onPressed: () =>
                        openDestination(context, content, sessions),
                    style: FilledButton.styleFrom(
                      backgroundColor: lime,
                      foregroundColor: forest,
                    ),
                    icon: const Icon(Icons.view_agenda_outlined),
                    label: Text(sessions.title),
                  ),
                ],
              ],
            ),
          ),
          if (shortcuts.isNotEmpty) ...[
            const SizedBox(height: 24),
            TitleText(content.text('home.glance', 'Your event, at a glance.')),
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
                    for (final entry in shortcuts)
                      _QuickLink(
                        entryIcon(entry),
                        entryTitle(entry),
                        entrySubtitle(entry, content),
                        () => openDestination(context, content, entry),
                        width,
                      ),
                  ],
                );
              },
            ),
          ],
          // Staff announcements take the place of the programme summary.
          if (noticeTitle != null || noticeMessage != null) ...[
            const SizedBox(height: 24),
            GuideNotice(
              title: noticeTitle ?? '',
              message: noticeMessage ?? '',
              icon: Icons.campaign_outlined,
            ),
          ] else if (count > 0) ...[
            const SizedBox(height: 24),
            GuideNotice(
              title: '$count ${count == 1 ? 'session' : 'sessions'} to explore',
              message: 'Browse the published programme, with timings shown in India time (IST).',
              icon: Icons.campaign_outlined,
            ),
          ],
          if (links.isNotEmpty) const SizedBox(height: 16),
          for (final entry in links)
            GuideCard(
              entryIcon(entry),
              entryTitle(entry),
              entrySubtitle(entry, content),
              () => openDestination(context, content, entry),
            ),
        ],
      ),
    );
  }
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
              if (subtitle.isNotEmpty) ...[
                const SizedBox(height: 4),
                Text(
                  subtitle,
                  style: const TextStyle(color: muted, fontSize: 12),
                ),
              ],
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
        if (title.isNotEmpty) ...[
          const SizedBox(height: 12),
          Text(
            title,
            style: const TextStyle(fontFamily: 'Manrope', fontSize: 18),
          ),
        ],
        if (message.isNotEmpty) ...[
          const SizedBox(height: 8),
          Text(message, style: const TextStyle(color: muted, height: 1.5)),
        ],
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
    final content = state.content!;
    final sessions = [...content.guide.sessions]
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
          Eyebrow(content.text('sessions.eyebrow', 'THE PROGRAMME')),
          const SizedBox(height: 8),
          TitleText(
            content.text('sessions.title', 'Find your next\nconversation.'),
          ),
          const SizedBox(height: 10),
          Text(
            content.text(
              'sessions.intro',
              'Sessions, speakers and places to be. All times are in IST.',
            ),
            style: const TextStyle(color: muted),
          ),
          const SizedBox(height: 24),
          if (sessions.isEmpty) ...[
            const GuideNotice(
              title: 'Session timetable to be announced',
              message: 'The confirmed programme, session times and halls will appear here. Pull down to check for updates.',
            ),
            if (content.event.brochureUrl.isNotEmpty) ...[
              const SizedBox(height: 20),
              OutlinedButton.icon(
                onPressed: () => openLink(context, content.event.brochureUrl),
                icon: const Icon(Icons.article_outlined),
                label: const Text('View event brochure'),
              ),
            ],
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
                  subtitle: switch ([
                    if (session.startsAt case final start?)
                      '${sessionDay(start)} · ${sessionTime(start)} IST',
                    if (session.location.isNotEmpty) session.location,
                  ]) {
                    [] => null,
                    final lines => Padding(
                      padding: const EdgeInsets.only(top: 8),
                      child: Text(lines.join('\n')),
                    ),
                  },
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
                      if (s.startsAt case final start?)
                        _DetailBlock(
                          'When',
                          '${sessionDay(start)}\n${sessionTime(start)}${s.endsAt == null ? '' : ' - ${sessionTime(s.endsAt!)}'} IST',
                        ),
                      if (s.location.isNotEmpty)
                        _DetailBlock('Where', s.location),
                      if (s.speakers.isNotEmpty)
                        _DetailBlock('Speakers', s.speakers),
                      if (s.description.isNotEmpty)
                        _DetailBlock('About the session', s.description),
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
    final content = state.content!;
    final event = content.event, venue = content.guide.venue;
    final address = venue.address.isNotEmpty ? venue.address : event.city;
    return Scaffold(
      appBar: AppBar(title: const Text('Venue & directions')),
      body: LiveRefresh(
        onRefresh: state.load,
        child: ListView(
          keyboardDismissBehavior: ScrollViewKeyboardDismissBehavior.onDrag,
          physics: const AlwaysScrollableScrollPhysics(),
          padding: const EdgeInsets.all(20),
          children: [
            Eyebrow(content.text('venue.eyebrow', 'GETTING HERE')),
            const SizedBox(height: 8),
            TitleText(event.venue.isEmpty ? 'The venue' : event.venue),
            if (address.isNotEmpty) ...[
              const SizedBox(height: 10),
              SelectableText(address),
            ],
            if (event.venue.isNotEmpty || event.city.isNotEmpty) ...[
              const SizedBox(height: 18),
              FilledButton.icon(
                onPressed: () => openLink(context, directionsUrl(event)),
                icon: const Icon(Icons.directions_outlined),
                label: Text(
                  content.text('venue.directions', 'Open directions'),
                ),
              ),
            ],
            const SizedBox(height: 24),
            if (venue.arrival.isNotEmpty)
              _DetailBlock('Arrival & check-in', venue.arrival),
            if (venue.accessibility.isNotEmpty)
              _DetailBlock('Accessibility', venue.accessibility),
            // Shown only once the organisers publish a floor plan.
            if (venue.floorPlanUrl.isNotEmpty)
              OutlinedButton.icon(
                onPressed: () => openLink(context, venue.floorPlanUrl),
                icon: const Icon(Icons.map_outlined),
                label: const Text('Open venue floor plan'),
              ),
            const SizedBox(height: 20),
            _HelpDesk(venue, content.text('help.title', 'Need a hand?')),
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
            Eyebrow(content.text('activities.eyebrow', 'BEYOND THE SESSIONS')),
            const SizedBox(height: 8),
            TitleText(
              content.text('activities.title', 'Discover. Meet. Connect.'),
            ),
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
                        for (final (i, line) in [
                          if (a.schedule.isNotEmpty) a.schedule,
                          if (a.location.isNotEmpty) a.location,
                        ].indexed) ...[
                          SizedBox(height: i == 0 ? 14 : 6),
                          Text(line, style: const TextStyle(color: muted)),
                        ],
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
    final content = state.content!;
    final guide = content.guide;
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
            Eyebrow(content.text('faqs.eyebrow', 'A LITTLE HELP')),
            const SizedBox(height: 8),
            TitleText(content.text('faqs.title', 'Good to know.')),
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
            _HelpDesk(guide.venue, content.text('help.title', 'Need a hand?')),
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

/// Event-day contacts. Absent entirely until staff publish one.
class _HelpDesk extends StatelessWidget {
  const _HelpDesk(this.venue, this.title);
  final VenueGuide venue;
  final String title;
  @override
  Widget build(BuildContext context) {
    if (!venue.hasHelp) return const SizedBox.shrink();
    final whatsapp = venue.helpWhatsApp.replaceAll(RegExp(r'[^0-9]'), '');
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Text(
          title,
          style: const TextStyle(fontFamily: 'Manrope', fontSize: 18),
        ),
        const SizedBox(height: 8),
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
        if (whatsapp.isNotEmpty)
          ListTile(
            contentPadding: EdgeInsets.zero,
            leading: const Icon(Icons.chat_outlined),
            title: Text('WhatsApp ${venue.helpWhatsApp}'),
            onTap: () => openLink(context, 'https://wa.me/$whatsapp'),
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
      ],
    );
  }
}
