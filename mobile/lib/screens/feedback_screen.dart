import 'package:flutter/material.dart';
import 'package:provider/provider.dart';

import '../main.dart';
import '../models/event_guide.dart';
import '../providers/agenda.dart';
import '../providers/content_provider.dart';
import '../providers/feedback_book.dart';
import '../services/attendee_service.dart';
import '../widgets/directory.dart';
import '../widgets/interaction.dart';
import 'event_guide_screens.dart';

/// Rate the event and the sessions attended. Open only while staff allow it.
class FeedbackScreen extends StatelessWidget {
  const FeedbackScreen({super.key, this.title = 'Feedback'});
  final String title;
  @override
  Widget build(BuildContext context) {
    final state = context.watch<ContentProvider>();
    final content = state.content!;
    final agenda = context.watch<Agenda>();
    final book = context.watch<FeedbackBook>();
    final saved = agenda.sessions(content);
    final savedIds = saved.map((s) => s.id).toSet();
    // Sessions that have begun, the attendee's own first.
    final started = sortSessions(
      content.guide.sessions.where(
        (s) =>
            s.plannable &&
            !savedIds.contains(s.id) &&
            (s.startsAt == null || s.startsAt!.isBefore(agenda.now())),
      ),
    );
    return Scaffold(
      appBar: AppBar(title: Text(title)),
      body: LiveRefresh(
        onRefresh: state.load,
        child: ListView(
          physics: const AlwaysScrollableScrollPhysics(),
          padding: const EdgeInsets.fromLTRB(20, 16, 20, 36),
          children: [
            PageIntro(
              eyebrow: 'YOUR FEEDBACK',
              title: content.text('feedback.title', 'How was\nBio Connect?'),
              lede: content.feedback.intro.isNotEmpty ? content.feedback.intro : 'Your answers are anonymous and help shape the next edition. It takes a minute.',
            ),
            const SizedBox(height: 20),
            if (!content.feedback.open)
              const GuideNotice(
                icon: Icons.lock_clock_outlined,
                title: 'Feedback is not open',
                message: 'The organisers open feedback during the event. Pull down to check again.',
              )
            else ...[
              _RatingCard(
                sessionId: '',
                title: 'The event overall',
                subtitle: content.event.title,
                answer: book.answer(''),
                prominent: true,
              ),
              if (saved.isNotEmpty) ...[
                const SizedBox(height: 26),
                const SectionLabel('Sessions you saved'),
                const SizedBox(height: 10),
                for (final s in saved) _SessionRating(s, book.answer(s.id)),
              ],
              if (started.isNotEmpty) ...[
                const SizedBox(height: 26),
                SectionLabel(saved.isEmpty ? 'Sessions' : 'Other sessions'),
                const SizedBox(height: 10),
                for (final s in started) _SessionRating(s, book.answer(s.id)),
              ],
            ],
          ],
        ),
      ),
    );
  }
}

class _SessionRating extends StatelessWidget {
  const _SessionRating(this.session, this.answer);
  final GuideSession session;
  final FeedbackAnswer? answer;
  @override
  Widget build(BuildContext context) => Card(
    color: Colors.white,
    margin: const EdgeInsets.only(bottom: 9),
    child: InkWell(
      onTap: () => rateSession(context, session),
      child: Padding(
        padding: const EdgeInsets.all(14),
        child: Row(
          children: [
            Expanded(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text(
                    session.title,
                    style: const TextStyle(
                      fontWeight: FontWeight.w700,
                      fontSize: 13,
                    ),
                  ),
                  if (session.startsAt case final start?) ...[
                    const SizedBox(height: 4),
                    Text(
                      '${sessionDay(start)} · ${sessionTime(start)}',
                      style: const TextStyle(color: muted, fontSize: 11),
                    ),
                  ],
                ],
              ),
            ),
            const SizedBox(width: 10),
            if (answer == null)
              const Text(
                'Rate',
                style: TextStyle(color: forest, fontWeight: FontWeight.w700),
              )
            else
              Stars(answer!.rating, size: 16),
          ],
        ),
      ),
    ),
  );
}

/// Opens a sheet to rate one session.
Future<void> rateSession(
  BuildContext context,
  GuideSession session,
) => showModalBottomSheet<void>(
  context: context,
  isScrollControlled: true,
  showDragHandle: true,
  backgroundColor: paper,
  builder: (context) => Padding(
    padding: EdgeInsets.fromLTRB(
      20,
      0,
      20,
      20 + MediaQuery.viewInsetsOf(context).bottom,
    ),
    child: SingleChildScrollView(
      child: _RatingCard(
        sessionId: session.id,
        title: session.title,
        subtitle: session.startsAt == null
            ? session.location
            : '${sessionDay(session.startsAt!)} · ${sessionTime(session.startsAt!)}',
        answer: context.read<FeedbackBook>().answer(session.id),
        closeOnSend: true,
      ),
    ),
  ),
);

class _RatingCard extends StatefulWidget {
  const _RatingCard({
    required this.sessionId,
    required this.title,
    required this.subtitle,
    required this.answer,
    this.prominent = false,
    this.closeOnSend = false,
  });
  final String sessionId, title, subtitle;
  final FeedbackAnswer? answer;
  final bool prominent, closeOnSend;
  @override
  State<_RatingCard> createState() => _RatingCardState();
}

class _RatingCardState extends State<_RatingCard> {
  late int _rating = widget.answer?.rating ?? 0;
  late final _comment = TextEditingController(text: widget.answer?.comment);
  bool _sending = false;
  String? _error;

  @override
  void dispose() {
    _comment.dispose();
    super.dispose();
  }

  Future<void> _send() async {
    setState(() {
      _sending = true;
      _error = null;
    });
    try {
      await context.read<FeedbackBook>().submit(
        widget.sessionId,
        _rating,
        _comment.text,
      );
      AppFeedback.success();
      if (!mounted) return;
      ScaffoldMessenger.of(context).showSnackBar(
        const SnackBar(content: Text('Thank you. Your feedback was sent.')),
      );
      if (widget.closeOnSend) Navigator.pop(context);
    } on AttendeeException catch (e) {
      setState(
        () => _error = e.closed
            ? 'Feedback has closed. Thank you for taking part.'
            : e.message,
      );
    } finally {
      if (mounted) setState(() => _sending = false);
    }
  }

  @override
  Widget build(BuildContext context) => Container(
    padding: const EdgeInsets.all(18),
    decoration: BoxDecoration(
      color: widget.prominent ? cream : Colors.white,
      borderRadius: BorderRadius.circular(22),
    ),
    child: Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        Text(
          widget.title,
          style: const TextStyle(fontFamily: 'Manrope', fontSize: 18),
        ),
        if (widget.subtitle.isNotEmpty) ...[
          const SizedBox(height: 4),
          Text(widget.subtitle, style: const TextStyle(color: muted)),
        ],
        const SizedBox(height: 14),
        Row(
          mainAxisAlignment: MainAxisAlignment.center,
          children: [
            for (var i = 1; i <= 5; i++)
              IconButton(
                tooltip: '$i of 5',
                iconSize: 36,
                color: i <= _rating ? gold : muted.withValues(alpha: .45),
                onPressed: () {
                  AppFeedback.selection();
                  setState(() => _rating = i);
                },
                icon: Icon(
                  i <= _rating
                      ? Icons.star_rounded
                      : Icons.star_outline_rounded,
                ),
              ),
          ],
        ),
        const SizedBox(height: 10),
        TextField(
          controller: _comment,
          minLines: 2,
          maxLines: 6,
          maxLength: 2000,
          textCapitalization: TextCapitalization.sentences,
          decoration: const InputDecoration(
            hintText: 'What worked, and what could be better? (optional)',
          ),
        ),
        if (_error != null) ...[
          Text(_error!, style: const TextStyle(color: Color(0xFFA33A2B))),
          const SizedBox(height: 8),
        ],
        FilledButton(
          onPressed: _rating == 0 || _sending ? null : _send,
          child: Text(
            _sending
                ? 'Sending…'
                : widget.answer == null
                ? 'Send feedback'
                : 'Update feedback',
          ),
        ),
      ],
    ),
  );
}

class Stars extends StatelessWidget {
  const Stars(this.rating, {super.key, this.size = 18});
  final int rating;
  final double size;
  @override
  Widget build(BuildContext context) => Semantics(
    label: '$rating of 5 stars',
    child: Row(
      mainAxisSize: MainAxisSize.min,
      children: [
        for (var i = 1; i <= 5; i++)
          Icon(
            i <= rating ? Icons.star_rounded : Icons.star_outline_rounded,
            size: size,
            color: i <= rating ? gold : muted.withValues(alpha: .4),
          ),
      ],
    ),
  );
}
