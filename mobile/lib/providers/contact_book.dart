import 'dart:convert';

import 'package:flutter/foundation.dart';

import '../services/attendee_service.dart';
import '../services/local_store.dart';

/// Someone the attendee met, saved by scanning their badge. [notes] and
/// [tags] are private to this phone.
class SavedContact {
  const SavedContact({
    required this.qrId,
    this.name = '',
    this.designation = '',
    this.institution = '',
    this.category = '',
    this.email = '',
    this.phone = '',
    this.notes = '',
    this.tags = const [],
    required this.savedAt,
    this.pending = false,
  });
  final String qrId, name, designation, institution, category, email, phone;
  final String notes;
  final List<String> tags;
  final DateTime savedAt;

  /// Scanned without a connection; the profile loads when back online.
  final bool pending;

  bool get sharedDetails => email.isNotEmpty || phone.isNotEmpty;
  String get displayName => name.isNotEmpty ? name : 'Badge scanned offline';

  SavedContact copyWith({
    String? notes,
    List<String>? tags,
    BadgeProfile? profile,
  }) => SavedContact(
    qrId: qrId,
    name: profile?.name ?? name,
    designation: profile?.designation ?? designation,
    institution: profile?.institution ?? institution,
    category: profile?.category ?? category,
    email: profile?.email ?? email,
    phone: profile?.phone ?? phone,
    notes: notes ?? this.notes,
    tags: tags ?? this.tags,
    savedAt: savedAt,
    pending: profile == null && pending,
  );

  factory SavedContact.fromJson(Map<String, dynamic> j) => SavedContact(
    qrId: j['qr_id'] as String,
    name: j['name'] as String? ?? '',
    designation: j['designation'] as String? ?? '',
    institution: j['institution'] as String? ?? '',
    category: j['category'] as String? ?? '',
    email: j['email'] as String? ?? '',
    phone: j['phone'] as String? ?? '',
    notes: j['notes'] as String? ?? '',
    tags: (j['tags'] as List? ?? const []).whereType<String>().toList(),
    savedAt:
        DateTime.tryParse(j['saved_at'] as String? ?? '') ?? DateTime(2026),
    pending: j['pending'] == true,
  );
  Map<String, dynamic> toJson() => {
    'qr_id': qrId,
    'name': name,
    'designation': designation,
    'institution': institution,
    'category': category,
    'email': email,
    'phone': phone,
    'notes': notes,
    'tags': tags,
    'saved_at': savedAt.toIso8601String(),
    'pending': pending,
  };
}

enum ScanOutcome { added, updated, savedOffline }

class ContactBook extends ChangeNotifier {
  ContactBook({
    LocalStore? store,
    AttendeeService? service,
    DateTime Function()? now,
  }) : _store = store ?? SecureStore(),
       service = service ?? AttendeeService(),
       _now = now ?? DateTime.now;

  static const _key = 'contacts_v1';
  static const suggestedTags = [
    'Follow up',
    'Investor',
    'Supplier',
    'Partner',
    'Customer',
    'Collaborator',
    'Hiring',
  ];

  final LocalStore _store;
  final AttendeeService service;
  final DateTime Function() _now;
  final List<SavedContact> _contacts = [];
  bool loaded = false;
  String? error;

  /// Newest first.
  List<SavedContact> get contacts => List.unmodifiable(_contacts);
  List<String> get usedTags {
    final tags = <String>{for (final c in _contacts) ...c.tags};
    return [
      ...suggestedTags.where(tags.contains),
      ...(tags.difference(suggestedTags.toSet()).toList()..sort()),
    ];
  }

  SavedContact? byQr(String qr) {
    for (final c in _contacts) {
      if (c.qrId == qr) return c;
    }
    return null;
  }

  Future<void> load() async {
    try {
      final raw = await _store.read(_key);
      if (raw != null) {
        _contacts
          ..clear()
          ..addAll(
            (jsonDecode(raw) as List).whereType<Map>().map(
              (m) => SavedContact.fromJson(m.cast<String, dynamic>()),
            ),
          );
      }
    } catch (e) {
      error = 'Saved contacts could not be opened.';
      debugPrint('Contacts could not be read: $e');
    }
    loaded = true;
    notifyListeners();
    await resolvePending();
  }

  Future<void> _save() async {
    notifyListeners();
    try {
      await _store.write(
        _key,
        jsonEncode(_contacts.map((c) => c.toJson()).toList()),
      );
      error = null;
    } catch (e) {
      error = 'Contacts could not be saved on this phone. Try again.';
      notifyListeners();
    }
  }

  /// Saves the badge with this admission code. Throws [AttendeeException]
  /// when the badge is not active.
  Future<ScanOutcome> addScan(String qr) async {
    final existing = byQr(qr);
    try {
      final profile = await service.badge(qr);
      if (existing != null) {
        _replace(existing.copyWith(profile: profile));
      } else {
        _contacts.insert(
          0,
          SavedContact(qrId: qr, savedAt: _now()).copyWith(profile: profile),
        );
      }
      await _save();
      return existing == null ? ScanOutcome.added : ScanOutcome.updated;
    } on AttendeeException catch (e) {
      if (!e.offline) rethrow;
      if (existing == null) {
        _contacts.insert(
          0,
          SavedContact(qrId: qr, savedAt: _now(), pending: true),
        );
        await _save();
      }
      return ScanOutcome.savedOffline;
    }
  }

  /// Loads badges scanned offline, and with [all] refreshes what everyone
  /// else shares, so withdrawn consent is honoured. False when offline.
  Future<bool> resolvePending({bool all = false}) async {
    for (final c in [..._contacts]) {
      if (!all && !c.pending) continue;
      try {
        _replace(c.copyWith(profile: await service.badge(c.qrId)));
      } on AttendeeException catch (e) {
        if (e.offline) {
          await _save();
          return false;
        }
        // A badge revoked before it could load keeps its notes and tags.
        if (c.pending) {
          _replace(
            c.copyWith(
              profile: BadgeProfile(qrId: c.qrId, name: 'Inactive badge'),
            ),
          );
        }
      }
    }
    await _save();
    return true;
  }

  void _replace(SavedContact next) {
    final i = _contacts.indexWhere((c) => c.qrId == next.qrId);
    if (i >= 0) _contacts[i] = next;
  }

  Future<void> update(
    SavedContact contact, {
    String? notes,
    List<String>? tags,
  }) {
    _replace(contact.copyWith(notes: notes, tags: tags));
    return _save();
  }

  Future<void> remove(SavedContact contact) {
    _contacts.removeWhere((c) => c.qrId == contact.qrId);
    return _save();
  }
}

/// Saved contacts as CSV, ready for a spreadsheet or CRM import.
String contactsCsv(List<SavedContact> contacts) {
  String cell(String v) {
    // Neutralise spreadsheet formulas in values other people typed. A
    // phone number such as +91 98470 00000 cannot carry one, so stays as is.
    final formula =
        RegExp(r'^[=+\-@\t\r]').hasMatch(v) &&
        !RegExp(r'^\+[0-9 ()\-]+$').hasMatch(v);
    final safe = formula ? "'$v" : v;
    return '"${safe.replaceAll('"', '""')}"';
  }

  final rows = [
    [
      'Name',
      'Designation',
      'Organisation',
      'Category',
      'Email',
      'Phone',
      'Tags',
      'Notes',
      'Saved',
    ],
    for (final c in contacts)
      [
        c.name,
        c.designation,
        c.institution,
        c.category,
        c.email,
        c.phone,
        c.tags.join('; '),
        c.notes,
        c.savedAt.toIso8601String(),
      ],
  ];
  return '${rows.map((r) => r.map(cell).join(',')).join('\r\n')}\r\n';
}

/// Saved contacts as vCards, which phones and email apps import directly.
String contactsVcf(List<SavedContact> contacts) {
  String esc(String v) => v
      .replaceAll(r'\', r'\\')
      .replaceAll('\n', r'\n')
      .replaceAll(',', r'\,')
      .replaceAll(';', r'\;');
  final out = StringBuffer();
  for (final c in contacts.where((c) => c.name.isNotEmpty)) {
    final note = [
      'Met at Bio Connect 4.0',
      if (c.tags.isNotEmpty) 'Tags: ${c.tags.join(', ')}',
      if (c.notes.isNotEmpty) c.notes,
    ].join('\n');
    out
      ..write('BEGIN:VCARD\r\nVERSION:3.0\r\n')
      ..write('FN:${esc(c.name)}\r\nN:${esc(c.name)};;;;\r\n')
      ..write(c.institution.isEmpty ? '' : 'ORG:${esc(c.institution)}\r\n')
      ..write(c.designation.isEmpty ? '' : 'TITLE:${esc(c.designation)}\r\n')
      ..write(c.email.isEmpty ? '' : 'EMAIL;TYPE=INTERNET:${esc(c.email)}\r\n')
      ..write(c.phone.isEmpty ? '' : 'TEL;TYPE=CELL:${esc(c.phone)}\r\n')
      ..write(
        c.tags.isEmpty ? '' : 'CATEGORIES:${c.tags.map(esc).join(',')}\r\n',
      )
      ..write('NOTE:${esc(note)}\r\nEND:VCARD\r\n');
  }
  return out.toString();
}
