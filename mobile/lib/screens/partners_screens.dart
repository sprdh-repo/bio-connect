import 'package:flutter/material.dart';
import 'package:provider/provider.dart';

import '../main.dart';
import '../models/event_content.dart';
import '../providers/content_provider.dart';
import '../widgets/directory.dart';

String _host(String url) =>
    (Uri.tryParse(url)?.host ?? '').replaceFirst(RegExp(r'^www\.'), '');

String _enquiry(String email, String subject) => Uri(
  scheme: 'mailto',
  path: email,
  query: 'subject=${Uri.encodeComponent('Bio Connect 4.0 - $subject')}',
).toString();

class SponsorsScreen extends StatelessWidget {
  const SponsorsScreen({super.key});

  @override
  Widget build(BuildContext context) {
    final state = context.watch<ContentProvider>();
    final content = state.content!;
    final sponsors = content.sponsors;
    return Scaffold(
      appBar: AppBar(title: const Text('Sponsors')),
      body: LiveRefresh(
        onRefresh: state.load,
        child: ListView(
          physics: const AlwaysScrollableScrollPhysics(),
          padding: const EdgeInsets.fromLTRB(20, 12, 20, 32),
          children: [
            PageIntro(
              eyebrow: 'OUR SPONSORS',
              title: 'Shared purpose.\nGreater possibilities.',
              lede: content.sponsorsIntro,
            ),
            const SizedBox(height: 26),
            SectionLabel(
              'Supporting the conclave',
              trailing:
                  '${sponsors.length} ${sponsors.length == 1 ? 'sponsor' : 'sponsors'}',
            ),
            const SizedBox(height: 12),
            for (final s in sponsors) SponsorCard(s),
            if (content.ecosystemPartners.isNotEmpty) ...[
              const SizedBox(height: 18),
              const SectionLabel('Ecosystem partners'),
              const SizedBox(height: 12),
              _PartnerGrid(content.ecosystemPartners),
            ],
            const SizedBox(height: 24),
            _CallToAction(
              eyebrow: 'BECOME A SPONSOR',
              title: 'Help make the next\nconnection possible.',
              action: 'Enquire about sponsorship',
              onTap: () => openLink(
                context,
                _enquiry(content.event.sponsorshipEmail, 'Sponsorship enquiry'),
              ),
            ),
          ],
        ),
      ),
    );
  }
}

class SponsorCard extends StatelessWidget {
  const SponsorCard(this.sponsor, {super.key});
  final Sponsor sponsor;

  @override
  Widget build(BuildContext context) {
    final s = sponsor;
    return Container(
      margin: const EdgeInsets.only(bottom: 12),
      padding: const EdgeInsets.all(14),
      decoration: BoxDecoration(
        color: Colors.white,
        borderRadius: BorderRadius.circular(20),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          LogoBox(
            url: s.logoUrl,
            name: s.name,
            size: double.infinity,
            height: 108,
            color: paper,
            padding: 18,
          ),
          Padding(
            padding: const EdgeInsets.fromLTRB(4, 16, 4, 0),
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                if (s.category.isNotEmpty) ...[
                  Text(
                    s.category.toUpperCase(),
                    style: const TextStyle(
                      color: forest,
                      fontSize: 10,
                      fontWeight: FontWeight.w700,
                      letterSpacing: 1.1,
                    ),
                  ),
                  const SizedBox(height: 6),
                ],
                Text(
                  s.name,
                  style: const TextStyle(
                    fontFamily: 'Manrope',
                    fontSize: 19,
                    height: 1.2,
                  ),
                ),
                const SizedBox(height: 8),
                Text(
                  s.description,
                  style: const TextStyle(
                    color: muted,
                    fontSize: 13,
                    height: 1.5,
                  ),
                ),
              ],
            ),
          ),
          if (s.websiteUrl.isNotEmpty)
            Padding(
              padding: const EdgeInsets.only(top: 6),
              child: TextButton.icon(
                onPressed: () => openLink(context, s.websiteUrl),
                icon: const Icon(Icons.arrow_outward, size: 16),
                iconAlignment: IconAlignment.end,
                label: Text(_host(s.websiteUrl)),
                style: TextButton.styleFrom(foregroundColor: forest),
              ),
            ),
        ],
      ),
    );
  }
}

class _PartnerGrid extends StatelessWidget {
  const _PartnerGrid(this.partners);
  final List<Partner> partners;
  @override
  Widget build(BuildContext context) => LayoutBuilder(
    builder: (context, constraints) {
      final columns = constraints.maxWidth > 600 ? 4 : 2;
      final width = (constraints.maxWidth - 10 * (columns - 1)) / columns;
      return Wrap(
        spacing: 10,
        runSpacing: 10,
        children: [
          for (final p in partners)
            Container(
              width: width,
              padding: const EdgeInsets.all(12),
              decoration: BoxDecoration(
                color: Colors.white,
                borderRadius: BorderRadius.circular(18),
              ),
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  LogoBox(
                    url: p.logoUrl,
                    name: p.name,
                    size: double.infinity,
                    height: 72,
                    color: paper,
                    padding: 10,
                  ),
                  const SizedBox(height: 10),
                  SizedBox(
                    height: 32,
                    child: Text(
                      p.name,
                      maxLines: 2,
                      overflow: TextOverflow.ellipsis,
                      style: const TextStyle(
                        fontSize: 11.5,
                        fontWeight: FontWeight.w600,
                        height: 1.3,
                      ),
                    ),
                  ),
                ],
              ),
            ),
        ],
      );
    },
  );
}

class LeadershipScreen extends StatelessWidget {
  const LeadershipScreen({super.key});

  @override
  Widget build(BuildContext context) {
    final state = context.watch<ContentProvider>();
    final content = state.content!;
    final leadership = content.leadership;
    final committee = leadership.committee;
    final credits = leadership.people.where((p) => p.imageCredit.isNotEmpty);
    return Scaffold(
      appBar: AppBar(title: const Text('Leadership')),
      body: LiveRefresh(
        onRefresh: state.load,
        child: ListView(
          physics: const AlwaysScrollableScrollPhysics(),
          padding: const EdgeInsets.fromLTRB(20, 12, 20, 32),
          children: [
            PageIntro(
              eyebrow: 'GOVERNMENT OF KERALA',
              title: 'The people convening\nthe conclave.',
              lede: leadership.intro,
            ),
            if (leadership.convenedBy case final convenor?) ...[
              const SizedBox(height: 20),
              Container(
                padding: const EdgeInsets.all(18),
                decoration: BoxDecoration(
                  color: cream,
                  borderRadius: BorderRadius.circular(18),
                ),
                child: Row(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    const CircleAvatar(
                      backgroundColor: Colors.white,
                      foregroundColor: forest,
                      child: Icon(Icons.account_balance_outlined),
                    ),
                    const SizedBox(width: 14),
                    Expanded(
                      child: Column(
                        crossAxisAlignment: CrossAxisAlignment.start,
                        children: [
                          const Eyebrow('CONVENED BY'),
                          const SizedBox(height: 6),
                          Text(
                            convenor.name,
                            style: const TextStyle(
                              fontFamily: 'Manrope',
                              fontSize: 17,
                            ),
                          ),
                          if (convenor.note.isNotEmpty) ...[
                            const SizedBox(height: 5),
                            Text(
                              convenor.note,
                              style: const TextStyle(
                                color: muted,
                                fontSize: 12,
                                height: 1.45,
                              ),
                            ),
                          ],
                        ],
                      ),
                    ),
                  ],
                ),
              ),
            ],
            if (leadership.people.isNotEmpty) ...[
              const SizedBox(height: 28),
              const SectionLabel('State leadership'),
              const SizedBox(height: 12),
              LayoutBuilder(
                builder: (context, constraints) {
                  final columns = constraints.maxWidth > 600 ? 3 : 2;
                  final width =
                      (constraints.maxWidth - 12 * (columns - 1)) / columns;
                  return Wrap(
                    spacing: 12,
                    runSpacing: 12,
                    children: [
                      for (final p in leadership.people)
                        SizedBox(width: width, child: LeaderCard(p)),
                    ],
                  );
                },
              ),
            ],
            if (committee.members.isNotEmpty) ...[
              const SizedBox(height: 30),
              SectionLabel(
                committee.title.isEmpty
                    ? 'Advisory Committee'
                    : committee.title,
                trailing: '${committee.members.length} members',
              ),
              const SizedBox(height: 8),
              Text(
                leadership.advisoryNote,
                style: const TextStyle(color: muted, height: 1.5),
              ),
              const SizedBox(height: 14),
              _Roster(committee.members),
              if (committee.orderNote.isNotEmpty) ...[
                const SizedBox(height: 12),
                Row(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    const Icon(
                      Icons.description_outlined,
                      size: 16,
                      color: muted,
                    ),
                    const SizedBox(width: 8),
                    Expanded(
                      child: Text(
                        committee.orderNote,
                        style: const TextStyle(
                          color: muted,
                          fontSize: 11,
                          height: 1.45,
                        ),
                      ),
                    ),
                  ],
                ),
              ],
            ] else ...[
              const SizedBox(height: 24),
              const Eyebrow('ADVISORY COMMITTEE'),
              const SizedBox(height: 8),
              Text(
                leadership.advisoryNote,
                style: const TextStyle(color: muted, height: 1.5),
              ),
            ],
            const SizedBox(height: 26),
            _CallToAction(
              eyebrow: 'WORKING WITH THE ORGANISERS',
              title: 'Partner with\nBio Connect 4.0.',
              action: 'Contact the event team',
              onTap: () => openLink(
                context,
                _enquiry(content.event.sponsorshipEmail, 'Committee enquiry'),
              ),
            ),
            for (final p in credits) ...[
              const SizedBox(height: 16),
              InkWell(
                onTap: p.imageCreditUrl.isEmpty
                    ? null
                    : () => openLink(context, p.imageCreditUrl),
                child: Text(
                  '${p.name}: ${p.imageCredit}',
                  style: TextStyle(
                    color: muted,
                    fontSize: 10.5,
                    height: 1.4,
                    decoration: p.imageCreditUrl.isEmpty
                        ? null
                        : TextDecoration.underline,
                    decorationColor: muted,
                  ),
                ),
              ),
            ],
          ],
        ),
      ),
    );
  }
}

class LeaderCard extends StatelessWidget {
  const LeaderCard(this.leader, {super.key});
  final Leader leader;

  @override
  Widget build(BuildContext context) {
    final fallback = ColoredBox(
      color: cream,
      child: Center(
        child: Text(
          initials(leader.name),
          style: const TextStyle(
            color: forest,
            fontFamily: 'Manrope',
            fontSize: 30,
          ),
        ),
      ),
    );
    return Container(
      decoration: BoxDecoration(
        color: Colors.white,
        borderRadius: BorderRadius.circular(18),
      ),
      clipBehavior: Clip.antiAlias,
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          AspectRatio(
            aspectRatio: 4 / 5,
            child: leader.imageUrl.isEmpty
                ? fallback
                : LayoutBuilder(
                    builder: (context, constraints) => CachedPicture(
                      leader.imageUrl,
                      alignment: Alignment.topCenter,
                      semanticLabel: 'Portrait of ${leader.name}',
                      decodeWidth: constraints.maxWidth,
                      fallback: fallback,
                    ),
                  ),
          ),
          Padding(
            padding: const EdgeInsets.fromLTRB(12, 12, 12, 14),
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Pill(leader.badge, background: lime.withValues(alpha: .45)),
                const SizedBox(height: 8),
                Text(
                  leader.name,
                  style: const TextStyle(
                    fontFamily: 'Manrope',
                    fontSize: 16,
                    height: 1.2,
                  ),
                ),
                const SizedBox(height: 4),
                Text(
                  leader.role,
                  style: const TextStyle(
                    color: muted,
                    fontSize: 11.5,
                    height: 1.4,
                  ),
                ),
              ],
            ),
          ),
        ],
      ),
    );
  }
}

class _Roster extends StatelessWidget {
  const _Roster(this.members);
  final List<CommitteeMember> members;
  @override
  Widget build(BuildContext context) => Container(
    decoration: BoxDecoration(
      color: Colors.white,
      borderRadius: BorderRadius.circular(18),
    ),
    child: Column(
      children: [
        for (var i = 0; i < members.length; i++) ...[
          if (i > 0)
            const Divider(height: 1, indent: 16, endIndent: 16, color: cream),
          Padding(
            padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 13),
            child: Row(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                SizedBox(
                  width: 82,
                  child: Align(
                    alignment: Alignment.centerLeft,
                    child: Pill(
                      members[i].role,
                      background: members[i].isLead
                          ? lime.withValues(alpha: .45)
                          : cream,
                      foreground: members[i].isLead ? forest : muted,
                    ),
                  ),
                ),
                const SizedBox(width: 10),
                Expanded(
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Text(
                        members[i].name,
                        style: const TextStyle(
                          fontWeight: FontWeight.w700,
                          fontSize: 13,
                          height: 1.3,
                        ),
                      ),
                      const SizedBox(height: 2),
                      Text(
                        members[i].organization,
                        style: const TextStyle(
                          color: muted,
                          fontSize: 12,
                          height: 1.35,
                        ),
                      ),
                    ],
                  ),
                ),
              ],
            ),
          ),
        ],
      ],
    ),
  );
}

class _CallToAction extends StatelessWidget {
  const _CallToAction({
    required this.eyebrow,
    required this.title,
    required this.action,
    required this.onTap,
  });
  final String eyebrow, title, action;
  final VoidCallback onTap;
  @override
  Widget build(BuildContext context) => Container(
    padding: const EdgeInsets.all(22),
    decoration: BoxDecoration(
      color: forest,
      borderRadius: BorderRadius.circular(20),
    ),
    child: Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Text(
          eyebrow,
          style: const TextStyle(
            color: lime,
            fontSize: 10,
            fontWeight: FontWeight.w700,
            letterSpacing: 1.3,
          ),
        ),
        const SizedBox(height: 9),
        Text(
          title,
          style: const TextStyle(
            color: Colors.white,
            fontFamily: 'Manrope',
            fontSize: 22,
            height: 1.2,
          ),
        ),
        const SizedBox(height: 16),
        FilledButton.icon(
          onPressed: onTap,
          style: FilledButton.styleFrom(
            backgroundColor: lime,
            foregroundColor: forest,
          ),
          icon: const Icon(Icons.mail_outline),
          label: Text(action),
        ),
      ],
    ),
  );
}
