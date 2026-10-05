import 'package:bio_connect_app/main.dart';
import 'package:bio_connect_app/models/event_content.dart';
import 'package:bio_connect_app/providers/content_provider.dart';
import 'package:bio_connect_app/screens/exhibitors_screen.dart';
import 'package:bio_connect_app/services/content_service.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:provider/provider.dart';

const _expo = [
  Exhibitor(
    name: 'Zeta Diagnostics',
    description: 'Point-of-care tests',
    logoUrl: '',
    stallNumber: 'A-10',
    stallType: 'Premium stall',
  ),
  Exhibitor(
    name: 'Alpha Bio',
    description: 'Enzymes',
    logoUrl: '',
    stallType: 'Table space',
  ),
  Exhibitor(
    name: 'Mid Labs',
    description: 'Genomics services',
    logoUrl: '',
    stallNumber: 'A-2',
    stallType: 'Standard stall',
  ),
];

/// Bundled event content with a controllable exhibitor directory.
class _FakeContent implements ContentService {
  _FakeContent(this.exhibitors);
  List<Exhibitor>? exhibitors;
  final _bundle = CurrentContentService(remoteContent: false);
  @override
  Future<EventContent> loadEvent() => _bundle.loadEvent();
  @override
  Future<EventContent> fetchEvent(EventContent current) async => current;
  @override
  Future<EventContent> refreshEvent(EventContent current) async => current;
  @override
  Future<List<Exhibitor>> loadExhibitors() async {
    final items = exhibitors;
    if (items == null) throw StateError('offline');
    return items;
  }
}

Future<ContentProvider> _pump(WidgetTester tester, _FakeContent service) async {
  final provider = ContentProvider(service);
  await provider.load();
  await tester.pumpWidget(
    ChangeNotifierProvider.value(
      value: provider,
      child: const MaterialApp(home: ExhibitorsScreen()),
    ),
  );
  await tester.pumpAndSettle();
  return provider;
}

void main() {
  // Asset futures cached in one test's fake-async zone never complete in the next.
  setUp(() => rootBundle.clear());
  test('stall numbers sort in floor-plan order', () {
    final stalls = ['B-1', 'A-10', 'a-2', 'A-2B', '12', '3'];
    stalls.sort(compareStallNumbers);
    expect(stalls, ['3', '12', 'a-2', 'A-2B', 'A-10', 'B-1']);
    expect(sortExhibitors(_expo, ExhibitorSort.stall).map((e) => e.name), [
      'Mid Labs',
      'Zeta Diagnostics',
      'Alpha Bio',
    ]);
    expect(sortExhibitors(_expo, ExhibitorSort.name).map((e) => e.name), [
      'Alpha Bio',
      'Mid Labs',
      'Zeta Diagnostics',
    ]);
  });

  test('search matches stall numbers with or without separators', () {
    final mid = _expo[2];
    expect(exhibitorMatches(mid, 'a2'), isTrue);
    expect(exhibitorMatches(mid, 'A-2'), isTrue);
    expect(exhibitorMatches(mid, 'genomics'), isTrue);
    expect(exhibitorMatches(mid, 'standard'), isTrue);
    expect(exhibitorMatches(mid, 'b2'), isFalse);
  });

  testWidgets('lists stalls, filters by type and opens a stall detail', (
    tester,
  ) async {
    await _pump(tester, _FakeContent(_expo));
    expect(find.text('3 exhibitors · 2 stalls allocated'), findsOneWidget);
    // Unallocated exhibitors show no stall placeholder.
    expect(find.text('TBA'), findsNothing);
    expect(find.text('STALL'), findsNWidgets(2));
    final first = tester.getTopLeft(find.text('Mid Labs')).dy;
    final second = tester.getTopLeft(find.text('Zeta Diagnostics')).dy;
    expect(first, lessThan(second));

    await tester.tap(find.widgetWithText(ChoiceChip, 'Premium stall'));
    await tester.pumpAndSettle();
    expect(find.text('1 of 3 exhibitors'), findsOneWidget);
    expect(find.text('Mid Labs'), findsNothing);

    await tester.tap(find.text('Zeta Diagnostics'));
    await tester.pumpAndSettle();
    expect(find.text('FIND THEM AT STALL'), findsOneWidget);
    expect(find.text('A-10'), findsOneWidget);
    expect(find.text('Point-of-care tests'), findsOneWidget);
  });

  testWidgets('hides stall features until any stall is allocated', (
    tester,
  ) async {
    final unallocated = [
      for (final e in _expo)
        Exhibitor(
          name: e.name,
          description: e.description,
          logoUrl: e.logoUrl,
          stallType: e.stallType,
        ),
    ];
    await _pump(tester, _FakeContent(unallocated));
    expect(find.text('3 exhibitors'), findsOneWidget);
    expect(find.text('STALL'), findsNothing);
    expect(find.byTooltip('Sort exhibitors'), findsNothing);
    expect(find.text('Search name or expertise'), findsOneWidget);
    await tester.tap(find.text('Alpha Bio'));
    await tester.pumpAndSettle();
    expect(find.text('FIND THEM AT STALL'), findsNothing);
    expect(find.text('Open venue floor plan'), findsNothing);
    expect(find.text('Table space'), findsOneWidget);
  });

  testWidgets('shows a retry state, then keeps the list when offline', (
    tester,
  ) async {
    final service = _FakeContent(null);
    final provider = await _pump(tester, service);
    expect(find.text('Try again'), findsOneWidget);

    service.exhibitors = _expo;
    await tester.tap(find.text('Try again'));
    await tester.pumpAndSettle();
    expect(find.text('Mid Labs'), findsOneWidget);

    service.exhibitors = null;
    await tester.fling(find.byType(ListView), const Offset(0, 400), 1000);
    await tester.pumpAndSettle();
    expect(provider.exhibitorsError, isNotNull);
    expect(find.text('Mid Labs'), findsOneWidget);
    expect(find.text('Showing the last loaded directory.'), findsOneWidget);
    expect(
      find.text('Could not reach Bio Connect. Showing saved information.'),
      findsOneWidget,
    );
  });

  testWidgets('guide opens separate sponsors and leadership pages', (
    tester,
  ) async {
    await tester.pumpWidget(
      ChangeNotifierProvider(
        create: (_) => ContentProvider(_FakeContent(_expo))..load(),
        child: const BioConnectApp(),
      ),
    );
    await tester.pumpAndSettle();
    await tester.tap(find.text('Guide').last);
    await tester.pumpAndSettle();

    await tester.scrollUntilVisible(find.text('Sponsors'), 200);
    await tester.ensureVisible(find.text('Sponsors'));
    await tester.pumpAndSettle();
    await tester.tap(find.text('Sponsors'));
    await tester.pumpAndSettle();
    expect(find.text('9 sponsors'), findsOneWidget);
    expect(find.text('State Bank of India'), findsOneWidget);
    expect(find.text('sbi.co.in'), findsOneWidget);
    await tester.pageBack();
    await tester.pumpAndSettle();

    await tester.scrollUntilVisible(find.text('Leadership'), 200);
    await tester.ensureVisible(find.text('Leadership'));
    await tester.pumpAndSettle();
    await tester.tap(find.text('Leadership'));
    await tester.pumpAndSettle();
    expect(find.text('V. D. Satheesan'), findsOneWidget);
    await tester.scrollUntilVisible(find.text('Mr. Aju Jacob'), 300);
    expect(find.text('14 members'), findsOneWidget);
    expect(find.text('Convenor'), findsOneWidget);
  });
}
