import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:mobile_scanner/mobile_scanner.dart';
import 'package:qr_flutter/qr_flutter.dart';

import '../main.dart';
import '../providers/pass_wallet.dart';
import '../services/pass_service.dart';
import '../widgets/interaction.dart';

class MyPassesScreen extends StatefulWidget {
  const MyPassesScreen({super.key, this.wallet});
  final PassWallet? wallet;
  @override
  State<MyPassesScreen> createState() => _MyPassesScreenState();
}

class _MyPassesScreenState extends State<MyPassesScreen>
    with WidgetsBindingObserver {
  late final PassWallet _wallet;
  @override
  void initState() {
    super.initState();
    _wallet = widget.wallet ?? PassWallet();
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
    if (widget.wallet == null) _wallet.dispose();
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

  @override
  Widget build(BuildContext context) => Scaffold(
    appBar: AppBar(
      title: const Text('My passes'),
      actions: [
        IconButton(
          tooltip: 'Remove saved passes',
          onPressed: _wallet.busy ? null : _forget,
          icon: const Icon(Icons.delete_outline),
        ),
      ],
    ),
    body: RefreshIndicator(
      onRefresh: () =>
          AppFeedback.refresh(_wallet.loaded ? _wallet.refresh : _wallet.load),
      child: ListView(
        padding: const EdgeInsets.all(20),
        physics: const AlwaysScrollableScrollPhysics(),
        children: [
          const Eyebrow('READY FOR BIO CONNECT'),
          const SizedBox(height: 8),
          const TitleText('Your pass.\nOn your phone.'),
          const SizedBox(height: 12),
          const Text(
            'Add an issued pass using your registered email or mobile number. Show its QR at the event entrance.',
          ),
          const SizedBox(height: 20),
          FilledButton.icon(
            onPressed: _wallet.busy || !_wallet.loaded
                ? null
                : () => Navigator.push(
                    context,
                    MaterialPageRoute(
                      builder: (_) => AddPassScreen(wallet: _wallet),
                    ),
                  ),
            icon: const Icon(Icons.add),
            label: const Text('Add a pass'),
          ),
          if (_wallet.busy) ...[
            const SizedBox(height: 16),
            const LinearProgressIndicator(),
          ],
          if (_wallet.error != null) ...[
            const SizedBox(height: 16),
            Text(_wallet.error!, style: const TextStyle(color: muted)),
            TextButton.icon(
              onPressed: _wallet.busy
                  ? null
                  : (_wallet.loaded ? _wallet.refresh : _wallet.load),
              icon: const Icon(Icons.refresh),
              label: const Text('Try again'),
            ),
          ],
          const SizedBox(height: 20),
          if (_wallet.passes.isEmpty && !_wallet.busy && _wallet.loaded)
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
          for (final pass in _wallet.passes) ...[
            _PassCard(
              pass: pass,
              checkedAt: _wallet.checkedAt(pass.id),
              offline: _wallet.offline,
            ),
            const SizedBox(height: 18),
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

class _PassCard extends StatelessWidget {
  const _PassCard({
    required this.pass,
    required this.checkedAt,
    required this.offline,
  });
  final AdmissionPass pass;
  final DateTime? checkedAt;
  final bool offline;
  @override
  Widget build(BuildContext context) => Card(
    color: Colors.white,
    clipBehavior: Clip.antiAlias,
    child: Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        Container(
          color: forest,
          padding: const EdgeInsets.all(20),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Text(
                pass.category,
                style: const TextStyle(
                  color: lime,
                  fontWeight: FontWeight.w700,
                ),
              ),
              const SizedBox(height: 8),
              Text(
                pass.name,
                style: const TextStyle(
                  color: Colors.white,
                  fontSize: 24,
                  fontFamily: 'Manrope',
                  fontWeight: FontWeight.w700,
                ),
              ),
              const SizedBox(height: 8),
              Text(
                pass.institution,
                style: const TextStyle(color: Colors.white),
              ),
              if (pass.designation.isNotEmpty)
                Text(
                  pass.designation,
                  style: const TextStyle(color: Colors.white70),
                ),
            ],
          ),
        ),
        Padding(
          padding: const EdgeInsets.all(20),
          child: Column(
            children: [
              const Text(
                'ADMISSION PASS',
                style: TextStyle(color: muted, letterSpacing: 1.5),
              ),
              const SizedBox(height: 12),
              Semantics(
                label:
                    'Admission QR for ${pass.name}. Show this code at the entrance.',
                child: QrImageView(
                  data: pass.qrId,
                  size: 240,
                  padding: const EdgeInsets.all(16),
                  backgroundColor: Colors.white,
                ),
              ),
              SelectableText(
                pass.number,
                style: const TextStyle(
                  fontFamily: 'Manrope',
                  fontWeight: FontWeight.w700,
                  fontSize: 18,
                ),
              ),
              const SizedBox(height: 12),
              if (checkedAt != null)
                Text(
                  'Last checked ${_checkedTime(checkedAt!)}${offline ? ' · Saved copy' : ''}',
                  textAlign: TextAlign.center,
                  style: const TextStyle(color: muted, fontSize: 12),
                ),
              const SizedBox(height: 12),
              OutlinedButton.icon(
                onPressed: () => openLink(context, pass.downloadUrl),
                icon: const Icon(Icons.picture_as_pdf_outlined),
                label: const Text('Open official PDF'),
              ),
            ],
          ),
        ),
      ],
    ),
  );
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
      MaterialPageRoute(builder: (_) => const PassScannerScreen()),
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
              const SizedBox(height: 16),
              const LinearProgressIndicator(),
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

class PassScannerScreen extends StatefulWidget {
  const PassScannerScreen({super.key});
  @override
  State<PassScannerScreen> createState() => _PassScannerScreenState();
}

class _PassScannerScreenState extends State<PassScannerScreen> {
  bool _done = false;
  String? _error;

  @override
  Widget build(BuildContext context) => Scaffold(
    appBar: AppBar(title: const Text('Scan your pass')),
    body: Column(
      children: [
        const Padding(
          padding: EdgeInsets.all(20),
          child: Text(
            'Point the camera at the admission QR on your Bio Connect pass. You will verify its registered email or mobile next.',
          ),
        ),
        Expanded(
          child: MobileScanner(
            errorBuilder: (context, error) => Center(
              child: Padding(
                padding: const EdgeInsets.all(24),
                child: Column(
                  mainAxisSize: MainAxisSize.min,
                  children: [
                    const Icon(Icons.no_photography_outlined, size: 40),
                    const SizedBox(height: 12),
                    const Text(
                      'Camera unavailable. Allow camera access in your phone settings, or add your pass using email or WhatsApp.',
                      textAlign: TextAlign.center,
                    ),
                    TextButton(
                      onPressed: () => Navigator.pop(context),
                      child: const Text('Use email or WhatsApp'),
                    ),
                  ],
                ),
              ),
            ),
            onDetect: (capture) {
              if (_done) return;
              for (final barcode in capture.barcodes) {
                final qr = admissionQr(barcode.rawValue ?? '');
                if (qr != null) {
                  _done = true;
                  AppFeedback.success();
                  Navigator.pop(context, qr);
                  return;
                }
              }
              if (mounted) {
                setState(
                  () => _error = 'This is not a Bio Connect admission QR. Scan the QR printed on your pass.',
                );
              }
            },
          ),
        ),
        if (_error != null)
          Padding(padding: const EdgeInsets.all(16), child: Text(_error!)),
      ],
    ),
  );
}
