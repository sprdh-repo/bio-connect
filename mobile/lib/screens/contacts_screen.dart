import 'dart:async';
import 'dart:io';

import 'package:flutter/material.dart';
import 'package:path_provider/path_provider.dart';
import 'package:provider/provider.dart';
import 'package:share_plus/share_plus.dart';

import '../main.dart';
import '../models/event_guide.dart';
import '../providers/contact_book.dart';
import '../providers/content_provider.dart';
import '../services/attendee_service.dart';
import '../services/pass_service.dart' show admissionQr;
import '../widgets/detail_header.dart';
import '../widgets/directory.dart';
import '../widgets/interaction.dart';
import '../widgets/motion.dart';
import 'event_guide_screens.dart';
import 'my_passes_screen.dart';
import 'qr_scanner_screen.dart';

/// Opens the badge scanner, saves the scanned attendee and shows them.
Future<void> scanBadge(BuildContext context) async {
  final book = context.read<ContactBook>();
  final messenger = ScaffoldMessenger.of(context);
  final navigator = Navigator.of(context);
  final qr = await navigator.push<String>(
    MaterialPageRoute(
      builder: (_) => const QrScannerScreen(
        title: 'Scan a badge',
        eyebrow: 'SAVE A CONTACT',
        instructions: 'Fit the QR on another attendee’s badge or pass inside the frame to save their event profile.',
        invalid: 'This is not a Bio Connect badge. Scan the QR printed on an attendee’s badge.',
        fallback: 'Back to contacts',
        cameraHelp:
            'Allow camera access in your phone settings to scan badges.',
        accept: admissionQr,
      ),
    ),
  );
  if (qr == null) return;
  try {
    final outcome = await book.addScan(qr);
    final contact = book.byQr(qr);
    messenger.showSnackBar(
      SnackBar(
        content: Text(switch (outcome) {
          ScanOutcome.added => 'Saved ${contact?.name ?? 'contact'}',
          ScanOutcome.updated =>
            '${contact?.name ?? 'Contact'} is already saved. Details refreshed.',
          ScanOutcome.savedOffline =>
            'Saved offline. Their profile loads when you are back online.',
        }),
      ),
    );
    if (contact != null) {
      unawaited(
        navigator.push(
          MaterialPageRoute<void>(builder: (_) => ContactDetailScreen(qr)),
        ),
      );
    }
  } on AttendeeException catch (e) {
    messenger.showSnackBar(
      SnackBar(
        content: Text(
          e.notFound
              ? 'This badge is not active. Ask them to check with the help desk.'
              : e.message,
        ),
      ),
    );
  }
}

/// Exports saved contacts through the share sheet as CSV or vCard.
Future<void> exportContacts(BuildContext context, {required bool vcard}) async {
  final contacts = context.read<ContactBook>().contacts;
  final messenger = ScaffoldMessenger.of(context);
  if (contacts.isEmpty) {
    messenger.showSnackBar(
      const SnackBar(
        content: Text('Save a contact first by scanning a badge.'),
      ),
    );
    return;
  }
  AppFeedback.action();
  try {
    final dir = await getTemporaryDirectory();
    final file = File(
      '${dir.path}/Bio-Connect-4.0-contacts.${vcard ? 'vcf' : 'csv'}',
    );
    await file.writeAsString(
      vcard ? contactsVcf(contacts) : contactsCsv(contacts),
    );
    await SharePlus.instance.share(
      ShareParams(
        subject: 'Contacts from Bio Connect 4.0',
        files: [XFile(file.path, mimeType: vcard ? 'text/vcard' : 'text/csv')],
      ),
    );
  } catch (_) {
    messenger.showSnackBar(
      const SnackBar(
        content: Text('Could not export contacts. Please try again.'),
      ),
    );
  }
}

class ContactsScreen extends StatefulWidget {
  const ContactsScreen({super.key, this.title = 'Contacts'});
  final String title;
  @override
  State<ContactsScreen> createState() => _ContactsScreenState();
}

class _ContactsScreenState extends State<ContactsScreen> {
  String query = '';
  String? tag;

  @override
  Widget build(BuildContext context) {
    final book = context.watch<ContactBook>();
    final content = context.watch<ContentProvider>().content;
    final tags = book.usedTags;
    final activeTag = tags.contains(tag) ? tag : null;
    final q = query.toLowerCase();
    final visible = book.contacts
        .where(
          (c) =>
              (activeTag == null || c.tags.contains(activeTag)) &&
              '${c.name} ${c.designation} ${c.institution} ${c.notes} ${c.tags.join(' ')}'
                  .toLowerCase()
                  .contains(q),
        )
        .toList();
    return Scaffold(
      appBar: AppBar(
        title: Text(widget.title),
        actions: [
          PopupMenuButton<bool>(
            tooltip: 'Export contacts',
            icon: const Icon(Icons.ios_share_rounded),
            onSelected: (vcard) => exportContacts(context, vcard: vcard),
            itemBuilder: (_) => const [
              PopupMenuItem(
                value: false,
                child: Text('Export as spreadsheet (CSV)'),
              ),
              PopupMenuItem(
                value: true,
                child: Text('Export to phone contacts (vCard)'),
              ),
            ],
          ),
        ],
      ),
      body: LiveRefresh(
        onRefresh: () => book.resolvePending(all: true),
        child: ListView(
          keyboardDismissBehavior: ScrollViewKeyboardDismissBehavior.onDrag,
          physics: const AlwaysScrollableScrollPhysics(),
          padding: const EdgeInsets.fromLTRB(20, 16, 20, 30),
          children: [
            PageIntro(
              eyebrow:
                  content?.text('contacts.eyebrow', 'PEOPLE YOU MET') ??
                  'PEOPLE YOU MET',
              title:
                  content?.text(
                    'contacts.title',
                    'Every conversation,\nin one place.',
                  ) ??
                  'Every conversation,\nin one place.',
              lede: 'Scan a badge to save someone’s event profile. Their email and phone appear only if they chose to share them.',
            ),
            const SizedBox(height: 18),
            FilledButton.icon(
              onPressed: () => scanBadge(context),
              icon: const Icon(Icons.qr_code_scanner_rounded),
              label: const Text('Scan a badge'),
            ),
            if (book.error != null) ...[
              const SizedBox(height: 12),
              Text(book.error!, style: const TextStyle(color: muted)),
            ],
            const SizedBox(height: 20),
            if (!book.loaded)
              const Center(child: BioLoader(width: 60))
            else if (book.contacts.isEmpty)
              const GuideNotice(
                icon: Icons.contacts_outlined,
                title: 'No contacts yet',
                message: 'When you meet someone, scan the QR on their badge. Add notes and tags like “Follow up” or “Investor”, then export everyone after the event.',
              )
            else ...[
              GuideSearchField(
                label: 'Search names, organisations or notes',
                onChanged: (v) => setState(() => query = v),
              ),
              if (tags.isNotEmpty) ...[
                const SizedBox(height: 10),
                SingleChildScrollView(
                  scrollDirection: Axis.horizontal,
                  child: Row(
                    children: [
                      for (final t in [null, ...tags])
                        Padding(
                          padding: const EdgeInsets.only(right: 8),
                          child: ChoiceChip(
                            label: Text(t ?? 'All'),
                            selected: activeTag == t,
                            onSelected: (_) {
                              AppFeedback.selection();
                              setState(() => tag = t);
                            },
                          ),
                        ),
                    ],
                  ),
                ),
              ],
              const SizedBox(height: 14),
              SectionLabel(
                'Saved',
                trailing: '${visible.length} of ${book.contacts.length}',
              ),
              const SizedBox(height: 10),
              if (visible.isEmpty)
                const StateMessage(
                  'No contacts match. Try another search or tag.',
                ),
              for (final (i, c) in visible.indexed)
                i < 8
                    ? Reveal(order: i, child: ContactTile(c))
                    : ContactTile(c),
            ],
            const SizedBox(height: 22),
            const _ShareYourselfNote(),
          ],
        ),
      ),
    );
  }
}

class _ShareYourselfNote extends StatelessWidget {
  const _ShareYourselfNote();
  @override
  Widget build(BuildContext context) => Card(
    color: cream,
    child: InkWell(
      onTap: () => showGuidePage(context, const MyPassesScreen()),
      child: const Padding(
        padding: EdgeInsets.all(16),
        child: Row(
          children: [
            Icon(Icons.badge_outlined, color: forest),
            SizedBox(width: 12),
            Expanded(
              child: Text(
                'Want people who scan your badge to get your email or phone? Turn on sharing in My passes.',
                style: TextStyle(height: 1.4),
              ),
            ),
            Icon(Icons.chevron_right, color: forest),
          ],
        ),
      ),
    ),
  );
}

class ContactAvatar extends StatelessWidget {
  const ContactAvatar(this.contact, {super.key, this.size = 46});
  final SavedContact contact;
  final double size;
  @override
  Widget build(BuildContext context) {
    final initials = contact.name
        .split(RegExp(r'\s+'))
        .where((w) => w.isNotEmpty && RegExp(r'[A-Za-z]').hasMatch(w[0]))
        .take(2)
        .map((w) => w[0].toUpperCase())
        .join();
    return Container(
      width: size,
      height: size,
      alignment: Alignment.center,
      decoration: BoxDecoration(
        color: contact.pending ? cream : forest,
        borderRadius: BorderRadius.circular(size * .32),
      ),
      child: contact.pending || initials.isEmpty
          ? Icon(Icons.person_outline, color: forest, size: size * .5)
          : Text(
              initials,
              style: TextStyle(
                fontFamily: 'Manrope',
                color: lime,
                fontSize: size * .36,
              ),
            ),
    );
  }
}

class ContactTile extends StatelessWidget {
  const ContactTile(this.contact, {super.key});
  final SavedContact contact;
  @override
  Widget build(BuildContext context) {
    final c = contact;
    final line = [
      c.designation,
      c.institution,
    ].where((s) => s.isNotEmpty).join(' · ');
    return Pressable(
      child: Card(
        color: Colors.white,
        margin: const EdgeInsets.only(bottom: 9),
        child: InkWell(
          onTap: () => showGuidePage(context, ContactDetailScreen(c.qrId)),
          child: Padding(
            padding: const EdgeInsets.all(14),
            child: Row(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                ContactAvatar(c),
                const SizedBox(width: 13),
                Expanded(
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Text(
                        c.displayName,
                        style: const TextStyle(
                          fontWeight: FontWeight.w700,
                          fontSize: 14,
                        ),
                      ),
                      if (c.pending)
                        const Text(
                          'Profile loads when you are online',
                          style: TextStyle(color: muted, fontSize: 12),
                        )
                      else if (line.isNotEmpty) ...[
                        const SizedBox(height: 3),
                        Text(
                          line,
                          style: const TextStyle(color: muted, fontSize: 12),
                        ),
                      ],
                      if (c.tags.isNotEmpty || c.sharedDetails) ...[
                        const SizedBox(height: 8),
                        Wrap(
                          spacing: 6,
                          runSpacing: 6,
                          children: [
                            if (c.sharedDetails)
                              const Pill(
                                'SHARED CONTACT',
                                background: forest,
                                foreground: lime,
                              ),
                            for (final t in c.tags) Pill(t),
                          ],
                        ),
                      ],
                    ],
                  ),
                ),
                const Icon(Icons.chevron_right, color: forest),
              ],
            ),
          ),
        ),
      ),
    );
  }
}

class ContactDetailScreen extends StatefulWidget {
  const ContactDetailScreen(this.qrId, {super.key});
  final String qrId;
  @override
  State<ContactDetailScreen> createState() => _ContactDetailScreenState();
}

class _ContactDetailScreenState extends State<ContactDetailScreen> {
  late final ContactBook _book;
  late final TextEditingController _notes;
  Timer? _debounce;

  @override
  void initState() {
    super.initState();
    _book = context.read<ContactBook>();
    _notes = TextEditingController(text: _book.byQr(widget.qrId)?.notes ?? '');
  }

  @override
  void dispose() {
    _flushNotes();
    _notes.dispose();
    super.dispose();
  }

  void _flushNotes() {
    _debounce?.cancel();
    final c = _book.byQr(widget.qrId);
    if (c != null && c.notes != _notes.text) {
      _book.update(c, notes: _notes.text);
    }
  }

  Future<void> _addTag(SavedContact c) async {
    final controller = TextEditingController();
    final tag = await showDialog<String>(
      context: context,
      builder: (context) => AlertDialog(
        title: const Text('New tag'),
        content: TextField(
          controller: controller,
          autofocus: true,
          maxLength: 30,
          textCapitalization: TextCapitalization.sentences,
          decoration: const InputDecoration(hintText: 'e.g. Distributor'),
          onSubmitted: (v) => Navigator.pop(context, v),
        ),
        actions: [
          TextButton(
            onPressed: () => Navigator.pop(context),
            child: const Text('Cancel'),
          ),
          FilledButton(
            onPressed: () => Navigator.pop(context, controller.text),
            child: const Text('Add'),
          ),
        ],
      ),
    );
    controller.dispose();
    final value = tag?.trim() ?? '';
    if (value.isEmpty || !mounted) return;
    if (!c.tags.contains(value)) {
      await context.read<ContactBook>().update(c, tags: [...c.tags, value]);
    }
  }

  Future<void> _remove(SavedContact c) async {
    final confirmed = await showDialog<bool>(
      context: context,
      builder: (context) => AlertDialog(
        title: Text('Remove ${c.name.isEmpty ? 'this contact' : c.name}?'),
        content: const Text(
          'Their profile, your notes and tags are deleted from this phone.',
        ),
        actions: [
          TextButton(
            onPressed: () => Navigator.pop(context, false),
            child: const Text('Keep'),
          ),
          TextButton(
            onPressed: () => Navigator.pop(context, true),
            child: const Text('Remove'),
          ),
        ],
      ),
    );
    if (confirmed != true || !mounted) return;
    _debounce?.cancel();
    _notes.text = c.notes;
    await context.read<ContactBook>().remove(c);
    if (mounted) Navigator.pop(context);
  }

  @override
  Widget build(BuildContext context) {
    final book = context.watch<ContactBook>();
    final c = book.byQr(widget.qrId);
    if (c == null) {
      return Scaffold(
        appBar: AppBar(),
        body: const StateMessage('This contact was removed.'),
      );
    }
    final tags = {...ContactBook.suggestedTags, ...c.tags}.toList();
    final whatsapp = c.phone.replaceAll(RegExp(r'[^0-9]'), '');
    return Scaffold(
      appBar: AppBar(
        title: const Text('Contact'),
        actions: [
          IconButton(
            tooltip: 'Remove contact',
            onPressed: () => _remove(c),
            icon: const Icon(Icons.delete_outline),
          ),
        ],
      ),
      body: ListView(
        keyboardDismissBehavior: ScrollViewKeyboardDismissBehavior.onDrag,
        padding: const EdgeInsets.fromLTRB(20, 12, 20, 36),
        children: [
          Row(
            children: [
              ContactAvatar(c, size: 64),
              const SizedBox(width: 16),
              Expanded(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    SelectableText(
                      c.displayName,
                      style: const TextStyle(
                        fontFamily: 'Manrope',
                        fontSize: 24,
                        height: 1.2,
                      ),
                    ),
                    if (c.category.isNotEmpty) ...[
                      const SizedBox(height: 6),
                      Pill(c.category.toUpperCase()),
                    ],
                  ],
                ),
              ),
            ],
          ),
          const SizedBox(height: 20),
          if (c.pending)
            const GuideNotice(
              icon: Icons.cloud_off_outlined,
              title: 'Saved offline',
              message: 'This badge was scanned without a connection. Their profile loads automatically when you are back online. Your notes are kept.',
            )
          else
            DetailPanel(
              children: [
                if (c.designation.isNotEmpty)
                  DetailFact(Icons.work_outline_rounded, 'Role', c.designation),
                if (c.institution.isNotEmpty)
                  DetailFact(
                    Icons.apartment_rounded,
                    'Organisation',
                    c.institution,
                  ),
                if (c.email.isNotEmpty)
                  DetailFact(Icons.mail_outline, 'Email', c.email),
                if (c.phone.isNotEmpty)
                  DetailFact(Icons.call_outlined, 'Phone', c.phone),
                DetailFact(
                  Icons.schedule_rounded,
                  'Saved',
                  '${sessionDay(c.savedAt)} · ${sessionTime(c.savedAt)}',
                ),
              ],
            ),
          if (c.sharedDetails) ...[
            const SizedBox(height: 14),
            Wrap(
              spacing: 10,
              runSpacing: 10,
              children: [
                if (c.email.isNotEmpty)
                  FilledButton.icon(
                    onPressed: () => openLink(context, 'mailto:${c.email}'),
                    icon: const Icon(Icons.mail_outline),
                    label: const Text('Email'),
                  ),
                if (c.phone.isNotEmpty) ...[
                  OutlinedButton.icon(
                    onPressed: () => openLink(context, 'tel:${c.phone}'),
                    icon: const Icon(Icons.call_outlined),
                    label: const Text('Call'),
                  ),
                  if (whatsapp.length >= 10)
                    OutlinedButton.icon(
                      onPressed: () =>
                          openLink(context, 'https://wa.me/$whatsapp'),
                      icon: const Icon(Icons.chat_outlined),
                      label: const Text('WhatsApp'),
                    ),
                ],
              ],
            ),
          ] else if (!c.pending) ...[
            const SizedBox(height: 12),
            const Text(
              'They have not shared an email or phone. Note how to reach them below, or exchange details in person.',
              style: TextStyle(color: muted, height: 1.45, fontSize: 13),
            ),
          ],
          const SizedBox(height: 26),
          const Eyebrow('TAGS'),
          const SizedBox(height: 10),
          Wrap(
            spacing: 8,
            runSpacing: 8,
            children: [
              for (final t in tags)
                FilterChip(
                  label: Text(t),
                  selected: c.tags.contains(t),
                  onSelected: (on) {
                    AppFeedback.selection();
                    book.update(
                      c,
                      tags: on
                          ? [...c.tags, t]
                          : c.tags.where((x) => x != t).toList(),
                    );
                  },
                ),
              ActionChip(
                avatar: const Icon(Icons.add, size: 18, color: forest),
                label: const Text('New tag'),
                onPressed: () => _addTag(c),
              ),
            ],
          ),
          const SizedBox(height: 26),
          const Eyebrow('PRIVATE NOTES'),
          const SizedBox(height: 10),
          TextField(
            controller: _notes,
            minLines: 4,
            maxLines: 12,
            maxLength: 4000,
            textCapitalization: TextCapitalization.sentences,
            decoration: const InputDecoration(
              hintText: 'What you talked about, what to follow up on…',
            ),
            onChanged: (_) {
              _debounce?.cancel();
              _debounce = Timer(const Duration(milliseconds: 600), _flushNotes);
            },
          ),
          const Text(
            'Notes and tags stay on this phone. They are included when you export.',
            style: TextStyle(color: muted, fontSize: 12),
          ),
        ],
      ),
    );
  }
}
