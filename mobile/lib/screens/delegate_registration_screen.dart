import 'package:flutter/material.dart';
import 'package:url_launcher/url_launcher.dart';

import '../services/registration_service.dart';

const _forest = Color(0xFF0B3329);
const _cream = Color(0xFFF3F1E9);
const _ink = Color(0xFF10201B);
const _muted = Color(0xFF616F69);

class DelegateRegistrationScreen extends StatefulWidget {
  const DelegateRegistrationScreen({super.key, this.service});
  final RegistrationService? service;

  @override
  State<DelegateRegistrationScreen> createState() =>
      _DelegateRegistrationScreenState();
}

class _DelegateRegistrationScreenState
    extends State<DelegateRegistrationScreen> {
  final _formKey = GlobalKey<FormState>();
  late final RegistrationService _service;
  final _institution = TextEditingController();
  final _name = TextEditingController();
  final _designation = TextEditingController();
  final _email = TextEditingController();
  final _phone = TextEditingController(text: '+91');
  List<RegistrationCategory>? _categories;
  RegistrationCategory? _category;
  String? _error;
  bool _whatsapp = false;
  bool _privacyAccepted = false;
  bool _submitting = false;

  @override
  void initState() {
    super.initState();
    _service = widget.service ?? RegistrationService();
    _loadCategories();
  }

  @override
  void dispose() {
    for (final controller in [
      _institution,
      _name,
      _designation,
      _email,
      _phone,
    ]) {
      controller.dispose();
    }
    super.dispose();
  }

  Future<void> _loadCategories() async {
    try {
      final categories = await _service.delegateCategories();
      if (!mounted) return;
      setState(() {
        _categories = categories;
        _category = categories.isEmpty ? null : categories.first;
        for (final category in categories) {
          if (category.id == 'industry') _category = category;
        }
        _error = categories.isEmpty
            ? 'Delegate registration is not open.'
            : null;
      });
    } catch (_) {
      if (mounted) {
        setState(() => _error = 'Pass options could not be loaded. Try again.');
      }
    }
  }

  String? _required(String? value) =>
      value == null || value.trim().isEmpty ? 'This field is required' : null;

  Future<void> _submit() async {
    if (!_formKey.currentState!.validate() || _category == null) return;
    setState(() {
      _submitting = true;
      _error = null;
    });
    try {
      final result = await _service.registerDelegate(
        categoryID: _category!.id,
        institution: _institution.text.trim(),
        name: _name.text.trim(),
        designation: _designation.text.trim(),
        email: _email.text.trim(),
        phone: _phone.text.trim(),
        whatsappConsent: _whatsapp,
      );
      if (!mounted) return;
      await showModalBottomSheet<void>(
        context: context,
        isDismissible: false,
        enableDrag: false,
        builder: (context) => SafeArea(
          child: Padding(
            padding: const EdgeInsets.all(24),
            child: Column(
              mainAxisSize: MainAxisSize.min,
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                const CircleAvatar(
                  backgroundColor: _cream,
                  foregroundColor: _forest,
                  child: Icon(Icons.check_rounded),
                ),
                const SizedBox(height: 16),
                const Text(
                  'Registration saved',
                  style: TextStyle(
                    fontFamily: 'Manrope',
                    fontSize: 25,
                    color: _ink,
                  ),
                ),
                const SizedBox(height: 8),
                const Text(
                  'Your private payment and registration link has also been emailed to you.',
                  style: TextStyle(color: _muted, height: 1.45),
                ),
                const SizedBox(height: 20),
                FilledButton.icon(
                  onPressed: () => launchUrl(
                    Uri.parse(result.referenceUrl),
                    mode: LaunchMode.externalApplication,
                  ),
                  icon: const Icon(Icons.lock_outline),
                  label: const Text('Continue securely to payment'),
                ),
                TextButton(
                  onPressed: () {
                    Navigator.pop(context);
                    Navigator.pop(context);
                  },
                  child: const Text('Finish for now'),
                ),
              ],
            ),
          ),
        ),
      );
    } catch (error) {
      if (mounted) setState(() => _error = error.toString());
    } finally {
      if (mounted) setState(() => _submitting = false);
    }
  }

  @override
  Widget build(BuildContext context) => Scaffold(
    appBar: AppBar(title: const Text('Delegate registration')),
    body: _categories == null && _error == null
        ? const Center(child: CircularProgressIndicator())
        : Form(
            key: _formKey,
            child: ListView(
              padding: const EdgeInsets.fromLTRB(20, 12, 20, 32),
              children: [
                const Text(
                  'Join the room where\nlife sciences moves forward.',
                  style: TextStyle(
                    fontFamily: 'Manrope',
                    fontSize: 28,
                    height: 1.15,
                    color: _ink,
                  ),
                ),
                const SizedBox(height: 10),
                const Text(
                  'Choose your pass and enter the attendee details. It takes about two minutes.',
                  style: TextStyle(color: _muted, height: 1.45),
                ),
                const SizedBox(height: 24),
                if (_categories != null && _categories!.isNotEmpty) ...[
                  const _SectionLabel('CHOOSE YOUR PASS'),
                  const SizedBox(height: 10),
                  for (final category in _categories!)
                    _PassOption(
                      category: category,
                      selected: category == _category,
                      onTap: () => setState(() => _category = category),
                    ),
                  const SizedBox(height: 22),
                  const _SectionLabel('ATTENDEE DETAILS'),
                  const SizedBox(height: 10),
                  _Field(
                    controller: _name,
                    label: 'Full name',
                    icon: Icons.person_outline,
                    validator: _required,
                    textInputAction: TextInputAction.next,
                  ),
                  _Field(
                    controller: _designation,
                    label: 'Designation',
                    icon: Icons.badge_outlined,
                    validator: _required,
                    textInputAction: TextInputAction.next,
                  ),
                  _Field(
                    controller: _institution,
                    label: 'Institution / organisation',
                    icon: Icons.business_outlined,
                    validator: _required,
                    textInputAction: TextInputAction.next,
                  ),
                  _Field(
                    controller: _email,
                    label: 'Email',
                    icon: Icons.mail_outline,
                    keyboardType: TextInputType.emailAddress,
                    validator: (value) {
                      final error = _required(value);
                      if (error != null) return error;
                      return value!.contains('@')
                          ? null
                          : 'Enter a valid email';
                    },
                    textInputAction: TextInputAction.next,
                  ),
                  _Field(
                    controller: _phone,
                    label: 'Phone with country code',
                    icon: Icons.phone_outlined,
                    keyboardType: TextInputType.phone,
                    validator: (value) =>
                        RegExp(r'^\+[1-9][0-9]{7,14}$')
                            .hasMatch(value?.trim() ?? '')
                        ? null
                        : 'Use international format, e.g. +919876543210',
                    textInputAction: TextInputAction.done,
                  ),
                  CheckboxListTile(
                    value: _whatsapp,
                    onChanged: (value) =>
                        setState(() => _whatsapp = value ?? false),
                    contentPadding: EdgeInsets.zero,
                    controlAffinity: ListTileControlAffinity.leading,
                    title: const Text(
                      'Send my pass and event updates on WhatsApp',
                      style: TextStyle(fontSize: 13),
                    ),
                    subtitle: const Text(
                      'Optional. Your pass is always sent by email.',
                      style: TextStyle(fontSize: 11, color: _muted),
                    ),
                  ),
                  FormField<bool>(
                    initialValue: _privacyAccepted,
                    validator: (value) => value == true
                        ? null
                        : 'Confirm permission to register this attendee.',
                    builder: (field) => Column(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: [
                        CheckboxListTile(
                          value: _privacyAccepted,
                          onChanged: (value) {
                            final accepted = value ?? false;
                            setState(() => _privacyAccepted = accepted);
                            field.didChange(accepted);
                          },
                          contentPadding: EdgeInsets.zero,
                          controlAffinity: ListTileControlAffinity.leading,
                          title: const Text(
                            'I have checked these details and have permission to provide them for registration and pass delivery.',
                            style: TextStyle(fontSize: 13),
                          ),
                        ),
                        if (field.hasError)
                          Padding(
                            padding: const EdgeInsets.only(left: 12),
                            child: Text(
                              field.errorText!,
                              style: TextStyle(
                                color: Theme.of(context).colorScheme.error,
                                fontSize: 12,
                              ),
                            ),
                          ),
                      ],
                    ),
                  ),
                ],
                if (_error != null)
                  Container(
                    margin: const EdgeInsets.only(top: 12),
                    padding: const EdgeInsets.all(14),
                    decoration: BoxDecoration(
                      color: const Color(0xFFFFECE8),
                      borderRadius: BorderRadius.circular(12),
                    ),
                    child: Text(_error!, style: const TextStyle(height: 1.4)),
                  ),
                const SizedBox(height: 18),
                if (_categories != null && _categories!.isNotEmpty)
                  FilledButton(
                    onPressed: _submitting ? null : _submit,
                    style: FilledButton.styleFrom(
                      minimumSize: const Size.fromHeight(54),
                    ),
                    child: _submitting
                        ? const SizedBox.square(
                            dimension: 20,
                            child: CircularProgressIndicator(strokeWidth: 2),
                          )
                        : const Text('Save registration'),
                  )
                else
                  OutlinedButton(
                    onPressed: _loadCategories,
                    child: const Text('Try again'),
                  ),
              ],
            ),
          ),
  );
}

class _SectionLabel extends StatelessWidget {
  const _SectionLabel(this.text);
  final String text;
  @override
  Widget build(BuildContext context) => Text(
    text,
    style: const TextStyle(
      color: _forest,
      fontSize: 10,
      fontWeight: FontWeight.w700,
      letterSpacing: 1.35,
    ),
  );
}

class _PassOption extends StatelessWidget {
  const _PassOption({
    required this.category,
    required this.selected,
    required this.onTap,
  });
  final RegistrationCategory category;
  final bool selected;
  final VoidCallback onTap;

  @override
  Widget build(BuildContext context) => Padding(
    padding: const EdgeInsets.only(bottom: 8),
    child: Material(
      color: selected ? _cream : Colors.white,
      borderRadius: BorderRadius.circular(14),
      child: InkWell(
        onTap: onTap,
        borderRadius: BorderRadius.circular(14),
        child: Padding(
          padding: const EdgeInsets.all(15),
          child: Row(
            children: [
              Icon(
                selected ? Icons.radio_button_checked : Icons.radio_button_off,
                color: _forest,
              ),
              const SizedBox(width: 12),
              Expanded(
                child: Text(
                  category.label,
                  style: const TextStyle(fontWeight: FontWeight.w600),
                ),
              ),
              Text(
                _formatRupees(category.payablePaise),
                style: const TextStyle(
                  color: _forest,
                  fontWeight: FontWeight.w700,
                ),
              ),
            ],
          ),
        ),
      ),
    ),
  );
}

String _formatRupees(int paise) {
  final digits = (paise ~/ 100).toString();
  if (digits.length <= 3) return '₹$digits';
  final lastThree = digits.substring(digits.length - 3);
  final leading = digits.substring(0, digits.length - 3);
  final groups = <String>[];
  for (var end = leading.length; end > 0; end -= 2) {
    groups.insert(0, leading.substring(end > 2 ? end - 2 : 0, end));
  }
  return '₹${groups.join(',')},$lastThree';
}

class _Field extends StatelessWidget {
  const _Field({
    required this.controller,
    required this.label,
    required this.icon,
    required this.validator,
    required this.textInputAction,
    this.keyboardType,
  });
  final TextEditingController controller;
  final String label;
  final IconData icon;
  final FormFieldValidator<String> validator;
  final TextInputAction textInputAction;
  final TextInputType? keyboardType;

  @override
  Widget build(BuildContext context) => Padding(
    padding: const EdgeInsets.only(bottom: 12),
    child: TextFormField(
      controller: controller,
      validator: validator,
      keyboardType: keyboardType,
      textInputAction: textInputAction,
      autofillHints: switch (label) {
        'Full name' => const [AutofillHints.name],
        'Email' => const [AutofillHints.email],
        'Phone with country code' => const [AutofillHints.telephoneNumber],
        'Institution / organisation' => const [AutofillHints.organizationName],
        _ => null,
      },
      decoration: InputDecoration(
        labelText: label,
        prefixIcon: Icon(icon),
        filled: true,
        fillColor: Colors.white,
        border: OutlineInputBorder(
          borderRadius: BorderRadius.circular(14),
          borderSide: BorderSide.none,
        ),
      ),
    ),
  );
}
