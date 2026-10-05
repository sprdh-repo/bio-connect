import 'package:bio_connect_app/providers/contact_book.dart';
import 'package:bio_connect_app/providers/feedback_book.dart';
import 'package:bio_connect_app/services/attendee_service.dart';
import 'package:bio_connect_app/services/local_store.dart';
import 'package:dio/dio.dart';
import 'package:flutter_test/flutter_test.dart';

const qr = 'aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa';

class FakeAttendees extends AttendeeService {
  FakeAttendees() : super(dio: Dio());
  bool online = true;
  final profiles = <String, BadgeProfile>{
    qr: const BadgeProfile(
      qrId: qr,
      name: 'Asha Nair',
      designation: 'CTO',
      institution: 'Helix Labs',
      category: 'Industry',
      email: 'asha@helix.example',
    ),
  };
  final sent = <Map<String, Object>>[];

  @override
  Future<BadgeProfile> badge(String qrId) async {
    if (!online) {
      throw const AttendeeException('offline', offline: true);
    }
    return profiles[qrId] ??
        (throw const AttendeeException('not active', notFound: true));
  }

  @override
  Future<void> sendFeedback({
    required String deviceId,
    required String sessionId,
    required int rating,
    required String comment,
  }) async {
    if (!online) throw const AttendeeException('offline', offline: true);
    sent.add({
      'device': deviceId,
      'session': sessionId,
      'rating': rating,
      'comment': comment,
    });
  }
}

void main() {
  test(
    'a scanned badge is saved with its shared details, notes and tags',
    () async {
      final store = MemoryStore();
      final service = FakeAttendees();
      final book = ContactBook(store: store, service: service);
      await book.load();

      expect(await book.addScan(qr), ScanOutcome.added);
      final saved = book.byQr(qr)!;
      expect(
        (saved.name, saved.institution, saved.email, saved.phone),
        ('Asha Nair', 'Helix Labs', 'asha@helix.example', ''),
      );
      await book.update(saved, notes: 'Wants a demo', tags: ['Investor']);
      expect(await book.addScan(qr), ScanOutcome.updated);
      expect(book.contacts, hasLength(1));
      expect(
        book.byQr(qr)!.notes,
        'Wants a demo',
        reason: 'a rescan keeps notes',
      );

      // Withdrawn consent is honoured on the next refresh; notes stay.
      service.profiles[qr] = const BadgeProfile(qrId: qr, name: 'Asha Nair');
      expect(await book.resolvePending(all: true), isTrue);
      expect(book.byQr(qr)!.email, isEmpty);
      expect(book.byQr(qr)!.tags, ['Investor']);

      final reopened = ContactBook(store: store, service: service);
      await reopened.load();
      expect(reopened.byQr(qr)!.notes, 'Wants a demo');
      expect(reopened.usedTags, ['Investor']);

      await expectLater(
        book.addScan('b' * 43),
        throwsA(
          isA<AttendeeException>().having((e) => e.notFound, 'notFound', true),
        ),
      );
      expect(book.contacts, hasLength(1));
    },
  );

  test('a badge scanned offline loads once back online', () async {
    final service = FakeAttendees()..online = false;
    final book = ContactBook(store: MemoryStore(), service: service);
    await book.load();
    expect(await book.addScan(qr), ScanOutcome.savedOffline);
    expect(book.byQr(qr)!.pending, isTrue);
    expect(await book.resolvePending(), isFalse);

    service.online = true;
    expect(await book.resolvePending(), isTrue);
    expect(book.byQr(qr)!.pending, isFalse);
    expect(book.byQr(qr)!.name, 'Asha Nair');
  });

  test('exports neutralise formulas and escape vCard text', () {
    final contacts = [
      SavedContact(
        qrId: qr,
        name: '=HYPERLINK("x")',
        institution: 'Labs, Inc; "R&D"',
        phone: '+91 98470 00000',
        notes: 'Line one\nline two',
        tags: const ['Follow up', 'Supplier'],
        savedAt: DateTime.utc(2026, 10, 8, 4),
      ),
    ];
    final csv = contactsCsv(contacts);
    expect(csv, contains('"\'=HYPERLINK(""x"")"'));
    expect(
      csv,
      contains('"+91 98470 00000"'),
      reason: 'a leading plus in a phone is data',
    );
    expect(csv, contains('"Follow up; Supplier"'));

    final vcf = contactsVcf(contacts);
    expect(vcf, contains(r'ORG:Labs\, Inc\; "R&D"'));
    expect(vcf, contains('TEL;TYPE=CELL:+91 98470 00000'));
    expect(vcf, contains('CATEGORIES:Follow up,Supplier'));
    expect(
      vcf,
      contains(
        r'NOTE:Met at Bio Connect 4.0\nTags: Follow up\, Supplier\nLine one\nline two',
      ),
    );
    expect(
      contactsVcf([SavedContact(qrId: qr, savedAt: DateTime(2026))]),
      isEmpty,
    );
  });

  test(
    'feedback is sent once per target and remembered on this phone',
    () async {
      final store = MemoryStore();
      final service = FakeAttendees();
      final book = FeedbackBook(store: store, service: service);
      await book.load();
      await book.submit('', 4, '  Great event  ');
      await book.submit('s1', 5, '');
      await book.submit('', 5, 'Even better');
      expect(service.sent.map((s) => s['device']).toSet(), hasLength(1));
      expect(service.sent.first['comment'], '  Great event  ');
      expect(book.answer('')!.rating, 5);

      final reopened = FeedbackBook(store: store, service: service);
      await reopened.load();
      expect(reopened.answer('s1')!.rating, 5);
      expect(reopened.answer('')!.comment, 'Even better');
      await reopened.submit('s2', 3, '');
      expect(service.sent.last['device'], service.sent.first['device']);

      service.online = false;
      await expectLater(
        book.submit('s3', 2, ''),
        throwsA(isA<AttendeeException>()),
      );
      expect(
        book.answer('s3'),
        isNull,
        reason: 'unsent answers are not shown as sent',
      );
    },
  );
}
