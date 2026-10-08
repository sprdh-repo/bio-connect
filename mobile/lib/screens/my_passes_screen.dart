import 'dart:async';
import 'dart:io';

import 'package:flutter/foundation.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:provider/provider.dart';
import 'package:qr_flutter/qr_flutter.dart';

import '../main.dart';
import '../providers/content_provider.dart';
import '../providers/pass_wallet.dart';
import '../services/content_service.dart';
import '../services/pass_service.dart';
import '../widgets/destinations.dart';
import '../widgets/interaction.dart';
import '../widgets/motion.dart';
import 'qr_scanner_screen.dart';
import 'selfie_frame_screen.dart';

/// Debug builds on a device show a sample pass while none are saved, so the
/// pass design can be checked without an issued registration. Never in release
/// builds or widget tests.
final bool _showSamplePass =
    kDebugMode && !Platform.environment.containsKey('FLUTTER_TEST');

final _samplePass = AdmissionPass(
  id: 'sample',
  name: 'Ananya Menon',
  institution: 'Rajiv Gandhi Centre for Biotechnology',
  designation: 'Senior Scientist',
  category: 'Delegate · Academia',
  number: 'BC4-DEL-000042',
  qrId: 'SAMPLE-NOT-VALID-FOR-ADMISSION',
  downloadUrl: CurrentContentService.defaultApiBaseUrl,
);

class MyPassesScreen extends StatefulWidget {
  const MyPassesScreen({super.key, this.wallet});
  final PassWallet? wallet;
  @override
  State<MyPassesScreen> createState() => _MyPassesScreenState();
}

class _MyPassesScreenState extends State<MyPassesScreen>
    with WidgetsBindingObserver {
  late final PassWallet _wallet;
  PassWallet? _ownWallet;
  @override
  void initState() {
    super.initState();
    _wallet =
        widget.wallet ??
        context.read<PassWallet?>() ??
        (_ownWallet = PassWallet());
    WidgetsBinding.instance.addObserver(this);
    _wallet.addListener(_changed);
    _wallet.load();
  }

  void _changed() {
    if (mounted) setState(() {});
  }

  @override
  void didChangeAppLifecycleState(AppLifecycleState state) {
    if (state == AppLifecycleState.resumed) _wallet.refresh();
  }

  @override
  void dispose() {
    WidgetsBinding.instance.removeObserver(this);
    _wallet.removeListener(_changed);
    _ownWallet?.dispose();
    super.dispose();
  }

  Future<void> _forget() async {
    final confirmed = await showDialog<bool>(
      context: context,
      builder: (context) => AlertDialog(
        title: const Text('Remove saved passes?'),
        content: const Text(
          'This removes pass copies from this phone. Your event registration stays active. You can add your passes again using OTP.',
        ),
        actions: [
          TextButton(
            onPressed: () => Navigator.pop(context, false),
            child: const Text('Keep passes'),
          ),
          TextButton(
            onPressed: () => Navigator.pop(context, true),
            child: const Text('Remove'),
          ),
        ],
      ),
    );
    if (confirmed == true) await _wallet.forget();
  }

  void _add() => Navigator.push(
    context,
    MaterialPageRoute(builder: (_) => AddPassScreen(wallet: _wallet)),
  );

  @override
  Widget build(BuildContext context) {
    final hasPasses = _wallet.passes.isNotEmpty;
    final canAdd = !_wallet.busy && _wallet.loaded;
    return Scaffold(
      appBar: AppBar(
        title: const Text('My passes'),
        actions: [
          if (hasPasses)
            IconButton(
              tooltip: 'Remove saved passes',
              onPressed: _wallet.busy ? null : _forget,
              icon: const Icon(Icons.delete_outline),
            ),
        ],
        // A saved pass stays put while its status is checked, so the QR is
        // never pushed around on open or resume.
        bottom: hasPasses && _wallet.busy
            ? const PreferredSize(
                preferredSize: Size.fromHeight(2),
                child: LinearProgressIndicator(minHeight: 2),
              )
            : null,
      ),
      body: BioRefresh(
        onRefresh: () => AppFeedback.refresh(
          _wallet.loaded ? _wallet.refresh : _wallet.load,
        ),
        child: ListView(
          padding: const EdgeInsets.all(20),
          physics: const AlwaysScrollableScrollPhysics(),
          children: [
            // The pitch and the add button lead only until a pass is saved.
            // After that the pass itself is the first thing on screen.
            if (!hasPasses) ...[
              const Eyebrow('READY FOR BIO CONNECT'),
              const SizedBox(height: 8),
              const TitleText('Your pass.\nOn your phone.'),
              const SizedBox(height: 12),
              const Text(
                'Add an issued pass using your registered email or mobile number. Show its QR at the event entrance.',
              ),
              const SizedBox(height: 20),
              FilledButton.icon(
                onPressed: canAdd ? _add : null,
                icon: const Icon(Icons.add),
                label: const Text('Add a pass'),
              ),
              AnimatedSize(
                duration: const Duration(milliseconds: 260),
                curve: Curves.easeOutCubic,
                child: _wallet.busy
                    ? const Padding(
                        padding: EdgeInsets.only(top: 22),
                        child: Center(
                          child: BioLoader(
                            width: 60,
                            label: 'Checking your passes',
                          ),
                        ),
                      )
                    : const SizedBox(width: double.infinity),
              ),
            ],
            if (_wallet.error != null) ...[
              if (!hasPasses) const SizedBox(height: 16),
              Text(_wallet.error!, style: const TextStyle(color: muted)),
              TextButton.icon(
                onPressed: _wallet.busy
                    ? null
                    : (_wallet.loaded ? _wallet.refresh : _wallet.load),
                icon: const Icon(Icons.refresh),
                label: const Text('Try again'),
              ),
              if (hasPasses) const SizedBox(height: 8),
            ],
            if (!hasPasses) const SizedBox(height: 20),
            if (_showSamplePass &&
                !hasPasses &&
                !_wallet.busy &&
                _wallet.loaded) ...[
              const _SampleBadge(),
              const SizedBox(height: 10),
              Reveal(
                child: _PassCard(
                  pass: _samplePass,
                  checkedAt: null,
                  offline: false,
                ),
              ),
              const SizedBox(height: 18),
            ] else if (!hasPasses && !_wallet.busy && _wallet.loaded)
              const Card(
                child: Padding(
                  padding: EdgeInsets.all(20),
                  child: Column(
                    children: [
                      Icon(
                        Icons.confirmation_number_outlined,
                        size: 40,
                        color: forest,
                      ),
                      SizedBox(height: 12),
                      Text(
                        'No issued passes saved',
                        style: TextStyle(
                          fontSize: 18,
                          fontWeight: FontWeight.w700,
                        ),
                      ),
                      SizedBox(height: 8),
                      Text(
                        'Passes appear after registration approval. If a saved pass was revoked or replaced, add your current pass again.',
                        textAlign: TextAlign.center,
                      ),
                    ],
                  ),
                ),
              ),
            for (final (i, pass) in _wallet.passes.indexed) ...[
              Reveal(
                order: i,
                child: _PassCard(
                  pass: pass,
                  checkedAt: _wallet.checkedAt(pass.id),
                  offline: _wallet.offline,
                ),
              ),
              const SizedBox(height: 12),
              _SharingPanel(wallet: _wallet, pass: pass),
              const SizedBox(height: 18),
            ],
            if (hasPasses) _FramePrompt(_wallet.passes.first),
            // Still reachable for a colleague's or a second registration's
            // pass, but below the passes rather than above them.
            if (hasPasses) ...[
              OutlinedButton.icon(
                onPressed: canAdd ? _add : null,
                icon: const Icon(Icons.add),
                label: const Text('Add another pass'),
              ),
              const SizedBox(height: 20),
            ],
            const Text(
              'Passes are saved securely on this phone. Pull down to check the latest status. Admission is confirmed by event staff.',
              style: TextStyle(color: muted, height: 1.5),
            ),
          ],
        ),
      ),
    );
  }
}

/// Once a pass is saved, invites its holder to share that they are going,
/// with a caption for their role. Shown while staff offer the selfie frame.
class _FramePrompt extends StatelessWidget {
  const _FramePrompt(this.pass);
  final AdmissionPass pass;

  @override
  Widget build(BuildContext context) {
    final content = context.watch<ContentProvider?>()?.content;
    final entry = content == null ? null : selfieFrameEntry(content);
    if (content == null || entry == null) return const SizedBox.shrink();
    final now = DateTime.now();
    final title = switch (eventPhase(content.event, now)) {
      EventPhase.before => "You're registered. Share it.",
      EventPhase.during => "You're here. Share it.",
      EventPhase.after => 'Share that you were there.',
    };
    return Padding(
      padding: const EdgeInsets.only(bottom: 18),
      child: Pressable(
        child: Material(
          color: forest,
          borderRadius: BorderRadius.circular(18),
          clipBehavior: Clip.antiAlias,
          child: InkWell(
            onTap: () => openSelfieFrame(
              context,
              content,
              title: entryTitle(entry),
              caption: frameCaption(
                content.event,
                now,
                passCategory: pass.category,
              ),
            ),
            child: Padding(
              padding: const EdgeInsets.all(14),
              child: Row(
                children: [
                  Container(
                    width: 44,
                    height: 44,
                    decoration: BoxDecoration(
                      color: lime.withValues(alpha: .18),
                      borderRadius: BorderRadius.circular(14),
                    ),
                    child: const Icon(
                      Icons.add_a_photo_outlined,
                      color: lime,
                      size: 22,
                    ),
                  ),
                  const SizedBox(width: 13),
                  Expanded(
                    child: Column(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: [
                        Text(
                          title,
                          style: const TextStyle(
                            color: Colors.white,
                            fontWeight: FontWeight.w700,
                            fontSize: 14,
                          ),
                        ),
                        const SizedBox(height: 4),
                        const Text(
                          'Post your photo in a Bio Connect frame.',
                          style: TextStyle(color: Colors.white70, fontSize: 12),
                        ),
                      ],
                    ),
                  ),
                  const Icon(Icons.chevron_right, color: lime),
                ],
              ),
            ),
          ),
        ),
      ),
    );
  }
}

/// The holder's consent to give their email or phone to people who scan
/// their badge in the app. Off until they turn it on.
class _SharingPanel extends StatefulWidget {
  const _SharingPanel({required this.wallet, required this.pass});
  final PassWallet wallet;
  final AdmissionPass pass;
  @override
  State<_SharingPanel> createState() => _SharingPanelState();
}

class _SharingPanelState extends State<_SharingPanel> {
  bool _saving = false;

  Future<void> _set({bool? email, bool? phone}) async {
    final pass = widget.pass;
    setState(() => _saving = true);
    AppFeedback.selection();
    try {
      await widget.wallet.setSharing(
        pass,
        email: email ?? pass.shareEmail,
        phone: phone ?? pass.sharePhone,
      );
    } on PassException catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(context)
            .showSnackBar(SnackBar(content: Text(e.message)));
      }
    } finally {
      if (mounted) setState(() => _saving = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    final pass = widget.pass;
    // Material, not a decorated box, so the switches' ink shows.
    return Material(
      color: Colors.white,
      borderRadius: BorderRadius.circular(20),
      child: Padding(
        padding: const EdgeInsets.fromLTRB(16, 14, 8, 8),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            const Row(
              children: [
                Icon(Icons.badge_outlined, color: forest, size: 20),
                SizedBox(width: 8),
                Text(
                  'When someone scans your badge',
                  style: TextStyle(fontWeight: FontWeight.w700),
                ),
              ],
            ),
            const SizedBox(height: 6),
            const Padding(
              padding: EdgeInsets.only(right: 8),
              child: Text(
                'They always see what your badge prints. Choose what else they may save in their Bio Connect contacts.',
                style: TextStyle(color: muted, fontSize: 12, height: 1.4),
              ),
            ),
            SwitchListTile(
              contentPadding: EdgeInsets.zero,
              dense: true,
              value: pass.shareEmail,
              onChanged: _saving ? null : (v) => _set(email: v),
              title: const Text('Share my email'),
            ),
            SwitchListTile(
              contentPadding: EdgeInsets.zero,
              dense: true,
              value: pass.sharePhone,
              onChanged: _saving ? null : (v) => _set(phone: v),
              title: const Text('Share my phone number'),
            ),
          ],
        ),
      ),
    );
  }
}

class _SampleBadge extends StatelessWidget {
  const _SampleBadge();
  @override
  Widget build(BuildContext context) => Container(
    padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 9),
    decoration: BoxDecoration(
      color: gold.withValues(alpha: .18),
      borderRadius: BorderRadius.circular(12),
    ),
    child: const Row(
      children: [
        Icon(Icons.science_outlined, size: 18, color: ink),
        SizedBox(width: 8),
        Expanded(
          child: Text(
            'Sample pass for design preview. Shown in debug builds only and not valid for admission.',
            style: TextStyle(fontSize: 12, height: 1.35),
          ),
        ),
      ],
    ),
  );
}

/// An admission pass drawn as a ticket: the attendee on a forest header, a
/// perforated tear line, then the admission QR.
class _PassCard extends StatelessWidget {
  const _PassCard({
    required this.pass,
    required this.checkedAt,
    required this.offline,
  });
  final AdmissionPass pass;
  final DateTime? checkedAt;
  final bool offline;

  void _enlarge(BuildContext context) {
    AppFeedback.action();
    Navigator.push(
      context,
      PageRouteBuilder<void>(
        opaque: false,
        barrierColor: deepForest.withValues(alpha: .7),
        barrierDismissible: true,
        transitionDuration: const Duration(milliseconds: 320),
        reverseTransitionDuration: const Duration(milliseconds: 240),
        pageBuilder: (_, _, _) => _QrFullScreen(pass),
        transitionsBuilder: (_, animation, _, child) =>
            FadeTransition(opacity: animation, child: child),
      ),
    );
  }

  @override
  Widget build(BuildContext context) {
    final event = context.watch<ContentProvider?>()?.content?.event;
    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        DecoratedBox(
          decoration: const ShapeDecoration(
            shape: _TicketHalf(notchBottom: true),
            color: forest,
            shadows: _ticketShadow,
            image: DecorationImage(
              image: AssetImage('assets/images/pass-texture.webp'),
              fit: BoxFit.cover,
              // Tones down the texture's light sheen so copy stays legible.
              colorFilter: ColorFilter.mode(
                Color(0x73051C17),
                BlendMode.srcOver,
              ),
            ),
          ),
          child: Padding(
            padding: const EdgeInsets.fromLTRB(22, 22, 22, 24),
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                const Text(
                  'BIO CONNECT 4.0',
                  style: TextStyle(
                    color: lime,
                    fontSize: 11,
                    fontWeight: FontWeight.w700,
                    letterSpacing: 1.6,
                  ),
                ),
                const SizedBox(height: 10),
                Container(
                  padding: const EdgeInsets.symmetric(
                    horizontal: 12,
                    vertical: 6,
                  ),
                  decoration: BoxDecoration(
                    color: lime,
                    borderRadius: BorderRadius.circular(99),
                  ),
                  child: Text(
                    pass.category,
                    style: const TextStyle(
                      color: forest,
                      fontSize: 12,
                      fontWeight: FontWeight.w700,
                    ),
                  ),
                ),
                const SizedBox(height: 16),
                Text(
                  pass.name,
                  style: const TextStyle(
                    color: Colors.white,
                    fontSize: 27,
                    height: 1.12,
                    fontFamily: 'Manrope',
                    fontWeight: FontWeight.w700,
                  ),
                ),
                if (pass.designation.isNotEmpty) ...[
                  const SizedBox(height: 8),
                  Text(
                    pass.designation,
                    style: const TextStyle(color: Color(0xE6FFFFFF)),
                  ),
                ],
                const SizedBox(height: 2),
                Text(
                  pass.institution,
                  style: const TextStyle(
                    color: Colors.white,
                    fontWeight: FontWeight.w500,
                  ),
                ),
                if (event != null) ...[
                  const SizedBox(height: 18),
                  IconText(
                    Icons.calendar_today_outlined,
                    eventDateRange(event),
                  ),
                  if (event.venue.isNotEmpty) ...[
                    const SizedBox(height: 8),
                    IconText(Icons.place_outlined, event.venue),
                  ],
                ],
              ],
            ),
          ),
        ),
        DecoratedBox(
          decoration: const ShapeDecoration(
            shape: _TicketHalf(notchTop: true, edge: _ticketEdge),
            color: Colors.white,
            shadows: _ticketShadow,
          ),
          child: Column(
            children: [
              const Padding(
                padding: EdgeInsets.fromLTRB(
                  _TicketHalf.notch + 8,
                  2,
                  _TicketHalf.notch + 8,
                  0,
                ),
                child: CustomPaint(
                  size: Size(double.infinity, 1),
                  painter: _TearLine(),
                ),
              ),
              Padding(
                padding: const EdgeInsets.fromLTRB(22, 20, 22, 22),
                child: Column(
                  children: [
                    const Text(
                      'ADMISSION PASS',
                      style: TextStyle(
                        color: muted,
                        fontSize: 11,
                        fontWeight: FontWeight.w700,
                        letterSpacing: 1.8,
                      ),
                    ),
                    const SizedBox(height: 14),
                    Semantics(
                      button: true,
                      label:
                          'Admission QR for ${pass.name}. Show this code at the entrance. Double tap to enlarge.',
                      excludeSemantics: true,
                      child: GestureDetector(
                        onTap: () => _enlarge(context),
                        child: Hero(
                          tag: 'pass-qr-${pass.id}',
                          child: Container(
                            decoration: BoxDecoration(
                              color: Colors.white,
                              borderRadius: BorderRadius.circular(22),
                              border: Border.all(
                                color: _ticketEdge,
                                width: 1.5,
                              ),
                            ),
                            child: QrImageView(
                              data: pass.qrId,
                              size: 220,
                              padding: const EdgeInsets.all(16),
                              backgroundColor: Colors.white,
                              eyeStyle: const QrEyeStyle(
                                eyeShape: QrEyeShape.square,
                                color: forest,
                              ),
                              dataModuleStyle: const QrDataModuleStyle(
                                dataModuleShape: QrDataModuleShape.square,
                                color: ink,
                              ),
                            ),
                          ),
                        ),
                      ),
                    ),
                    const SizedBox(height: 8),
                    const Row(
                      mainAxisAlignment: MainAxisAlignment.center,
                      children: [
                        Icon(
                          Icons.zoom_out_map_rounded,
                          size: 14,
                          color: muted,
                        ),
                        SizedBox(width: 6),
                        Text(
                          'Tap to enlarge for scanning',
                          style: TextStyle(color: muted, fontSize: 12),
                        ),
                      ],
                    ),
                    const SizedBox(height: 18),
                    Container(
                      width: double.infinity,
                      padding: const EdgeInsets.symmetric(
                        horizontal: 16,
                        vertical: 12,
                      ),
                      decoration: BoxDecoration(
                        color: cream,
                        borderRadius: BorderRadius.circular(14),
                      ),
                      child: Column(
                        children: [
                          const Text(
                            'PASS NUMBER',
                            style: TextStyle(
                              color: muted,
                              fontSize: 10,
                              fontWeight: FontWeight.w700,
                              letterSpacing: 1.4,
                            ),
                          ),
                          const SizedBox(height: 4),
                          SelectableText(
                            pass.number,
                            textAlign: TextAlign.center,
                            style: const TextStyle(
                              fontFamily: 'Manrope',
                              fontWeight: FontWeight.w700,
                              fontSize: 19,
                              letterSpacing: 1.6,
                              color: forest,
                            ),
                          ),
                        ],
                      ),
                    ),
                    if (checkedAt != null) ...[
                      const SizedBox(height: 12),
                      Row(
                        mainAxisAlignment: MainAxisAlignment.center,
                        children: [
                          Icon(
                            offline
                                ? Icons.cloud_off_outlined
                                : Icons.verified_outlined,
                            size: 15,
                            color: offline ? muted : forest,
                          ),
                          const SizedBox(width: 6),
                          Flexible(
                            child: Text(
                              'Last checked ${_checkedTime(checkedAt!)}${offline ? ' · Saved copy' : ''}',
                              textAlign: TextAlign.center,
                              style: const TextStyle(
                                color: muted,
                                fontSize: 12,
                              ),
                            ),
                          ),
                        ],
                      ),
                    ],
                    const SizedBox(height: 16),
                    SizedBox(
                      width: double.infinity,
                      child: OutlinedButton.icon(
                        onPressed: () => openLink(context, pass.downloadUrl),
                        icon: const Icon(Icons.picture_as_pdf_outlined),
                        label: const Text('Open official PDF'),
                      ),
                    ),
                  ],
                ),
              ),
            ],
          ),
        ),
      ],
    );
  }
}

/// The QR alone on a bright card, large enough for any entrance scanner.
class _QrFullScreen extends StatelessWidget {
  const _QrFullScreen(this.pass);
  final AdmissionPass pass;

  @override
  Widget build(BuildContext context) => GestureDetector(
    onTap: () => Navigator.pop(context),
    behavior: HitTestBehavior.opaque,
    child: SafeArea(
      child: Center(
        child: Padding(
          padding: const EdgeInsets.all(24),
          child: Material(
            color: Colors.white,
            borderRadius: BorderRadius.circular(28),
            child: Padding(
              padding: const EdgeInsets.fromLTRB(20, 24, 20, 14),
              child: Column(
                mainAxisSize: MainAxisSize.min,
                children: [
                  Text(
                    pass.name,
                    textAlign: TextAlign.center,
                    style: const TextStyle(
                      fontFamily: 'Manrope',
                      fontWeight: FontWeight.w700,
                      fontSize: 22,
                    ),
                  ),
                  const SizedBox(height: 4),
                  Text(
                    pass.number,
                    style: const TextStyle(
                      color: muted,
                      letterSpacing: 1.4,
                      fontWeight: FontWeight.w600,
                    ),
                  ),
                  const SizedBox(height: 12),
                  LayoutBuilder(
                    builder: (context, box) => Hero(
                      tag: 'pass-qr-${pass.id}',
                      child: QrImageView(
                        data: pass.qrId,
                        size: box.maxWidth.clamp(0, 360),
                        padding: const EdgeInsets.all(12),
                        backgroundColor: Colors.white,
                        semanticsLabel: 'Admission QR for ${pass.name}',
                      ),
                    ),
                  ),
                  TextButton.icon(
                    onPressed: () => Navigator.pop(context),
                    icon: const Icon(Icons.close),
                    label: const Text('Close'),
                  ),
                ],
              ),
            ),
          ),
        ),
      ),
    ),
  );
}

const _ticketEdge = Color(0xFFE2DECF);
const _ticketShadow = [
  BoxShadow(color: Color(0x1A051C17), blurRadius: 18, offset: Offset(0, 8)),
];

/// One half of a ticket: rounded outer corners and a semicircle bitten out of
/// each side where the two halves meet.
class _TicketHalf extends ShapeBorder {
  const _TicketHalf({
    this.notchTop = false,
    this.notchBottom = false,
    this.edge,
  });
  final bool notchTop, notchBottom;

  /// A hairline around the half, so a white half stands off the paper page.
  final Color? edge;
  static const radius = 24.0, notch = 13.0;

  @override
  EdgeInsetsGeometry get dimensions => EdgeInsets.zero;

  @override
  Path getInnerPath(Rect rect, {TextDirection? textDirection}) =>
      getOuterPath(rect, textDirection: textDirection);

  @override
  Path getOuterPath(Rect rect, {TextDirection? textDirection}) {
    const outer = Radius.circular(radius);
    final body = Path()
      ..addRRect(
        RRect.fromRectAndCorners(
          rect,
          topLeft: notchTop ? Radius.zero : outer,
          topRight: notchTop ? Radius.zero : outer,
          bottomLeft: notchBottom ? Radius.zero : outer,
          bottomRight: notchBottom ? Radius.zero : outer,
        ),
      );
    final y = notchTop ? rect.top : rect.bottom;
    final bites = Path()
      ..addOval(Rect.fromCircle(center: Offset(rect.left, y), radius: notch))
      ..addOval(Rect.fromCircle(center: Offset(rect.right, y), radius: notch));
    return Path.combine(PathOperation.difference, body, bites);
  }

  @override
  void paint(Canvas canvas, Rect rect, {TextDirection? textDirection}) {
    if (edge == null) return;
    canvas.drawPath(
      getOuterPath(rect.deflate(.5)),
      Paint()
        ..style = PaintingStyle.stroke
        ..strokeWidth = 1
        ..color = edge!,
    );
  }

  @override
  ShapeBorder scale(double t) => this;
}

class _TearLine extends CustomPainter {
  const _TearLine();
  @override
  void paint(Canvas canvas, Size size) {
    final paint = Paint()
      ..color = const Color(0xFFC9C5B4)
      ..strokeWidth = 1.6
      ..strokeCap = StrokeCap.round;
    const dash = 6.0, gap = 6.0;
    for (var x = 0.0; x < size.width; x += dash + gap) {
      canvas.drawLine(Offset(x, 0), Offset(x + dash, 0), paint);
    }
  }

  @override
  bool shouldRepaint(_TearLine old) => false;
}

String _checkedTime(DateTime value) {
  final t = value.toLocal();
  return '${t.day}/${t.month}/${t.year} ${t.hour.toString().padLeft(2, '0')}:${t.minute.toString().padLeft(2, '0')}';
}

class AddPassScreen extends StatefulWidget {
  const AddPassScreen({super.key, required this.wallet});
  final PassWallet wallet;
  @override
  State<AddPassScreen> createState() => _AddPassScreenState();
}

class _AddPassScreenState extends State<AddPassScreen> {
  final _form = GlobalKey<FormState>();
  final _identifier = TextEditingController();
  final _code = TextEditingController();
  String _channel = 'email', _qr = '';
  bool _consent = false, _busy = false;
  String? _error;
  PassChallenge? _challenge;
  PassAccess? _verified;
  Timer? _timer;
  DateTime? _resendAt;
  int get _remaining => _resendAt == null
      ? 0
      : _resendAt!.difference(DateTime.now()).inSeconds.clamp(0, 3600);
  @override
  void dispose() {
    _timer?.cancel();
    _identifier.dispose();
    _code.dispose();
    super.dispose();
  }

  Future<void> _scan() async {
    final qr = await Navigator.push<String>(
      context,
      MaterialPageRoute(
        builder: (_) => const QrScannerScreen(
          title: 'Scan your pass',
          eyebrow: 'ADD YOUR PASS',
          instructions: 'Fit the QR on your Bio Connect pass inside the frame. You will verify its registered email or mobile next.',
          invalid: 'This is not a Bio Connect admission QR. Scan the QR printed on your pass.',
          fallback: 'Use email or WhatsApp instead',
          cameraHelp: 'Allow camera access in your phone settings, or add your pass using email or WhatsApp.',
          accept: admissionQr,
        ),
      ),
    );
    if (qr != null && mounted) {
      setState(() {
        _qr = qr;
        _error = null;
      });
    }
  }

  Future<void> _requestCode() async {
    if (_busy || _remaining > 0 || !_form.currentState!.validate()) return;
    FocusManager.instance.primaryFocus?.unfocus();
    setState(() {
      _busy = true;
      _error = null;
    });
    try {
      final challenge = await widget.wallet.service.requestCode(
        channel: _channel,
        identifier: _identifier.text,
        qrId: _qr,
        whatsappConsent: _consent,
      );
      if (!mounted) return;
      setState(() {
        _challenge = challenge;
        _verified = null;
        _code.clear();
        _resendAt = DateTime.now().add(
          Duration(seconds: challenge.resendAfterSeconds),
        );
      });
      _timer?.cancel();
      _timer = Timer.periodic(const Duration(seconds: 1), (timer) {
        if (!mounted) {
          timer.cancel();
          return;
        }
        setState(() {});
        if (_remaining == 0) timer.cancel();
      });
      AppFeedback.action();
    } on PassException catch (e) {
      if (mounted) setState(() => _error = e.message);
    } catch (_) {
      if (mounted) {
        setState(
          () => _error = 'The code could not be requested. Please retry.',
        );
      }
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  Future<void> _verify() async {
    if (_busy ||
        (_verified == null &&
            !RegExp(r'^\d{6}$').hasMatch(_code.text.trim()))) {
      if (!_busy) {
        setState(() => _error = 'Enter the six-digit verification code.');
      }
      return;
    }
    FocusManager.instance.primaryFocus?.unfocus();
    setState(() {
      _busy = true;
      _error = null;
    });
    try {
      // Keep the verified session on a failed load, so retry never reuses a consumed OTP.
      _verified ??= await widget.wallet.service.verify(_challenge!, _code.text);
      final access = await widget.wallet.service.refresh(_verified!);
      await widget.wallet.add(access);
      if (!mounted) return;
      AppFeedback.success();
      Navigator.pop(context);
    } on PassException catch (e) {
      if (e.unauthorized && _verified != null) _verified = null;
      if (mounted) setState(() => _error = e.message);
    } catch (_) {
      if (mounted) {
        setState(() => _error = 'Your pass could not be loaded. Please retry.');
      }
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  @override
  Widget build(BuildContext context) => PopScope(
    canPop: !_busy,
    child: Scaffold(
      appBar: AppBar(title: const Text('Add a pass')),
      body: Form(
        key: _form,
        child: ListView(
          padding: const EdgeInsets.all(20),
          keyboardDismissBehavior: ScrollViewKeyboardDismissBehavior.onDrag,
          children: [
            const TitleText('Bring your pass\nwith you.'),
            const SizedBox(height: 12),
            const Text(
              'Use the email or mobile number registered for the attendee. A verification code confirms the pass belongs to you.',
            ),
            const SizedBox(height: 20),
            if (_challenge == null) ...[
              OutlinedButton.icon(
                onPressed: _busy ? null : _scan,
                icon: const Icon(Icons.qr_code_scanner),
                label: Text(
                  _qr.isEmpty
                      ? 'Scan an existing pass QR'
                      : 'Scan a different pass',
                ),
              ),
              if (_qr.isNotEmpty)
                ListTile(
                  contentPadding: EdgeInsets.zero,
                  leading: const Icon(
                    Icons.check_circle_outline,
                    color: forest,
                  ),
                  title: const Text('Pass QR scanned'),
                  subtitle: const Text(
                    'Verify its registered email or mobile below.',
                  ),
                  trailing: IconButton(
                    tooltip: 'Clear scanned pass',
                    onPressed: _busy ? null : () => setState(() => _qr = ''),
                    icon: const Icon(Icons.close),
                  ),
                ),
              const SizedBox(height: 20),
              SegmentedButton<String>(
                segments: const [
                  ButtonSegment(
                    value: 'email',
                    icon: Icon(Icons.email_outlined),
                    label: Text('Email'),
                  ),
                  ButtonSegment(
                    value: 'whatsapp',
                    icon: Icon(Icons.phone_outlined),
                    label: Text('WhatsApp'),
                  ),
                ],
                selected: {_channel},
                onSelectionChanged: _busy
                    ? null
                    : (value) => setState(() {
                        _channel = value.first;
                        _identifier.clear();
                        _consent = false;
                        _error = null;
                      }),
              ),
              const SizedBox(height: 20),
            ],
            TextFormField(
              controller: _identifier,
              readOnly: _busy || _challenge != null,
              keyboardType: _channel == 'email'
                  ? TextInputType.emailAddress
                  : TextInputType.phone,
              autocorrect: false,
              enableSuggestions: false,
              decoration: InputDecoration(
                labelText: _channel == 'email'
                    ? 'Registered email'
                    : 'Registered mobile number',
                hintText: _channel == 'email'
                    ? 'you@example.com'
                    : '+91 98765 43210',
              ),
              validator: (value) {
                final v = value?.trim() ?? '';
                if (_channel == 'email' &&
                    !RegExp(r'^[^\s@]+@[^\s@]+\.[^\s@]+$').hasMatch(v)) {
                  return 'Enter your registered email.';
                }
                if (_channel == 'whatsapp' &&
                    !RegExp(r'^\+?[0-9 ()-]{10,22}$').hasMatch(v)) {
                  return 'Enter your registered mobile number with country code.';
                }
                return null;
              },
            ),
            if (_channel == 'whatsapp' && _challenge == null)
              CheckboxListTile(
                contentPadding: EdgeInsets.zero,
                value: _consent,
                onChanged: _busy
                    ? null
                    : (value) => setState(() => _consent = value ?? false),
                title: const Text('Send my verification code on WhatsApp'),
                subtitle: const Text(
                  'This request allows a one-time verification message to this number.',
                ),
              ),
            const SizedBox(height: 20),
            if (_challenge == null)
              FilledButton(
                onPressed:
                    _busy ||
                        _remaining > 0 ||
                        (_channel == 'whatsapp' && !_consent)
                    ? null
                    : _requestCode,
                child: Text(
                  _remaining > 0
                      ? 'Request code in ${_remaining}s'
                      : 'Send verification code',
                ),
              ),
            if (_challenge != null) ...[
              Text(
                'If an issued pass matches, a code will arrive by ${_channel == 'email' ? 'email' : 'WhatsApp'}. Check spam if needed. The code expires in 10 minutes.',
              ),
              const SizedBox(height: 16),
              if (_verified == null)
                TextField(
                  controller: _code,
                  enabled: !_busy,
                  keyboardType: TextInputType.number,
                  autofillHints: const [AutofillHints.oneTimeCode],
                  inputFormatters: [
                    FilteringTextInputFormatter.digitsOnly,
                    LengthLimitingTextInputFormatter(6),
                  ],
                  decoration: const InputDecoration(
                    labelText: 'Six-digit code',
                  ),
                  onSubmitted: (_) => _verify(),
                ),
              const SizedBox(height: 16),
              FilledButton(
                onPressed: _busy ? null : _verify,
                child: Text(
                  _verified == null ? 'Verify and add pass' : 'Load my pass',
                ),
              ),
              if (_verified == null) ...[
                TextButton(
                  onPressed: _busy || _remaining > 0 ? null : _requestCode,
                  child: Text(
                    _remaining > 0 ? 'Resend in ${_remaining}s' : 'Resend code',
                  ),
                ),
                TextButton(
                  onPressed: _busy
                      ? null
                      : () => setState(() {
                          _challenge = null;
                          _code.clear();
                          _error = null;
                        }),
                  child: const Text('Change email or mobile'),
                ),
              ],
            ],
            if (_busy) ...[
              const SizedBox(height: 20),
              const Center(child: BioLoader(width: 56)),
            ],
            if (_error != null) ...[
              const SizedBox(height: 16),
              Semantics(
                liveRegion: true,
                child: Text(
                  _error!,
                  style: TextStyle(color: Theme.of(context).colorScheme.error),
                ),
              ),
            ],
            const SizedBox(height: 24),
            const Text(
              'Only approved, active passes can be added. Scanning helps find a pass; OTP verifies ownership.',
              style: TextStyle(color: muted, height: 1.5),
            ),
          ],
        ),
      ),
    ),
  );
}
