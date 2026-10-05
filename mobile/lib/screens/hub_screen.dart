import 'package:flutter/material.dart';
import 'package:provider/provider.dart';

import '../main.dart';
import '../models/event_guide.dart';
import '../providers/agenda.dart';
import '../providers/contact_book.dart';
import '../providers/content_provider.dart';
import '../providers/feedback_book.dart';
import '../widgets/directory.dart';
import '../widgets/motion.dart';
import 'contacts_screen.dart';
import 'event_guide_screens.dart';
import 'exhibitors_screen.dart';
import 'feedback_screen.dart';

/// After the event: feedback, the people met with their notes, and what the
/// attendee saved, ready to follow up.
class AfterEventScreen extends StatelessWidget {
  const AfterEventScreen({super.key, this.title = 'After Bio Connect'});
  final String title;
  @override
  Widget build(BuildContext context) {
    final state = context.watch<ContentProvider>();
    final content = state.content!;
    final agenda = context.watch<Agenda>();
    final contacts = context.watch<ContactBook>().contacts;
    final feedback = context.watch<FeedbackBook>();
    final sessions = agenda.sessions(content);
    final speakers = agenda.speakers(content);
    final exhibitors = agenda.exhibitors;
    final followUps = contacts.where((c) => c.tags.contains('Follow up'));
    return Scaffold(
      appBar: AppBar(title: Text(title)),
      body: LiveRefresh(
        onRefresh: state.load,
        child: ListView(
          physics: const AlwaysScrollableScrollPhysics(),
          padding: const EdgeInsets.fromLTRB(20, 16, 20, 36),
          children: [
            PageIntro(
              eyebrow: content.text('hub.eyebrow', 'AFTER BIO CONNECT'),
              title: content.text(
                'hub.title',
                'Keep the\nconversations going.',
              ),
            ),
            const SizedBox(height: 20),
            Row(
              children: [
                Expanded(
                  child: Metric(
                    '${contacts.length}',
                    'Contacts',
                    Icons.contacts_outlined,
                  ),
                ),
                const SizedBox(width: 10),
                Expanded(
                  child: Metric(
                    '${sessions.length}',
                    'Sessions saved',
                    Icons.event_note_outlined,
                  ),
                ),
                const SizedBox(width: 10),
                Expanded(
                  child: Metric(
                    '${followUps.length}',
                    'To follow up',
                    Icons.flag_outlined,
                  ),
                ),
              ],
            ),
            if (content.feedback.open) ...[
              const SizedBox(height: 18),
              Reveal(
                child: _HubCard(
                  icon: Icons.rate_review_outlined,
                  title: feedback.answer('') == null
                      ? 'Tell us how it went'
                      : 'Thanks for your feedback',
                  body: feedback.answer('') == null
                      ? 'Rate the event and the sessions you attended. It is anonymous and takes a minute.'
                      : 'You can still rate sessions or change your answers.',
                  action: feedback.answer('') == null
                      ? 'Give feedback'
                      : 'Rate sessions',
                  onTap: () => showGuidePage(context, const FeedbackScreen()),
                  dark: feedback.answer('') == null,
                ),
              ),
            ],
            const SizedBox(height: 26),
            SectionLabel('People you met', trailing: '${contacts.length}'),
            const SizedBox(height: 10),
            if (contacts.isEmpty)
              const StateMessage(
                'You did not save anyone this time. Scanning a badge at the next event keeps everyone in one place.',
              )
            else ...[
              for (final c
                  in (followUps.isNotEmpty ? followUps : contacts).take(4))
                ContactTile(c),
              Wrap(
                spacing: 10,
                runSpacing: 10,
                children: [
                  OutlinedButton.icon(
                    onPressed: () =>
                        showGuidePage(context, const ContactsScreen()),
                    icon: const Icon(Icons.contacts_outlined),
                    label: Text('All ${contacts.length} contacts'),
                  ),
                  OutlinedButton.icon(
                    onPressed: () => exportContacts(context, vcard: false),
                    icon: const Icon(Icons.table_chart_outlined),
                    label: const Text('Export CSV'),
                  ),
                  OutlinedButton.icon(
                    onPressed: () => exportContacts(context, vcard: true),
                    icon: const Icon(Icons.person_add_alt_outlined),
                    label: const Text('Save to phone'),
                  ),
                ],
              ),
            ],
            if (sessions.isNotEmpty) ...[
              const SizedBox(height: 26),
              SectionLabel('Your sessions', trailing: '${sessions.length}'),
              const SizedBox(height: 10),
              for (final s in sessions)
                GuideCard(
                  feedback.answer(s.id) == null
                      ? Icons.event_note_outlined
                      : Icons.star_rounded,
                  s.title,
                  [
                    if (s.startsAt case final start?)
                      '${sessionDay(start)} · ${sessionTime(start)} IST',
                    if (feedback.answer(s.id) case final a?)
                      'You rated it ${a.rating} of 5',
                  ].join('\n'),
                  () => content.feedback.open
                      ? rateSession(context, s)
                      : showGuidePage(context, SessionDetailScreen(s.id)),
                ),
            ],
            if (speakers.isNotEmpty) ...[
              const SizedBox(height: 26),
              SectionLabel(
                'Speakers you saved',
                trailing: '${speakers.length}',
              ),
              const SizedBox(height: 10),
              for (final s in speakers)
                GuideCard(
                  s.linkedin.isNotEmpty
                      ? Icons.open_in_new
                      : Icons.person_outline,
                  s.name,
                  [
                    s.role,
                    s.organization,
                  ].where((v) => v.isNotEmpty).join(' · '),
                  () => showGuidePage(context, SpeakerDetailScreen(s)),
                ),
            ],
            if (exhibitors.isNotEmpty) ...[
              const SizedBox(height: 26),
              SectionLabel(
                'Exhibitors you saved',
                trailing: '${exhibitors.length}',
              ),
              const SizedBox(height: 10),
              for (final e in exhibitors) ExhibitorTile(e),
            ],
          ],
        ),
      ),
    );
  }
}

class _HubCard extends StatelessWidget {
  const _HubCard({
    required this.icon,
    required this.title,
    required this.body,
    required this.action,
    required this.onTap,
    this.dark = false,
  });
  final IconData icon;
  final String title, body, action;
  final VoidCallback onTap;
  final bool dark;
  @override
  Widget build(BuildContext context) => Container(
    padding: const EdgeInsets.all(20),
    decoration: BoxDecoration(
      color: dark ? forest : cream,
      borderRadius: BorderRadius.circular(24),
    ),
    child: Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Icon(icon, color: dark ? lime : forest),
        const SizedBox(height: 12),
        Text(
          title,
          style: TextStyle(
            fontFamily: 'Manrope',
            fontSize: 20,
            color: dark ? Colors.white : ink,
          ),
        ),
        const SizedBox(height: 6),
        Text(
          body,
          style: TextStyle(
            color: dark ? Colors.white.withValues(alpha: .78) : muted,
            height: 1.45,
          ),
        ),
        const SizedBox(height: 16),
        FilledButton(
          onPressed: onTap,
          style: dark
              ? FilledButton.styleFrom(
                  backgroundColor: lime,
                  foregroundColor: forest,
                )
              : null,
          child: Text(action),
        ),
      ],
    ),
  );
}
