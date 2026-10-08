import 'package:flutter/cupertino.dart' show CupertinoPageTransitionsBuilder;
import 'package:flutter/foundation.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:provider/provider.dart';
import 'package:upgrader/upgrader.dart';
import 'package:url_launcher/url_launcher.dart';

import 'widgets/destinations.dart';
import 'widgets/detail_header.dart';
import 'widgets/directory.dart';
import 'widgets/interaction.dart';
import 'widgets/motion.dart';
import 'widgets/nav_bar.dart';

import 'models/event_content.dart';
import 'models/event_guide.dart';
import 'providers/agenda.dart';
import 'providers/contact_book.dart';
import 'providers/content_provider.dart';
import 'providers/feedback_book.dart';
import 'providers/pass_wallet.dart';
import 'screens/agenda_screen.dart';
import 'screens/contacts_screen.dart';
import 'screens/delegate_registration_screen.dart';
import 'screens/event_guide_screens.dart';
import 'screens/my_passes_screen.dart';
import 'widgets/saving.dart';
import 'widgets/splash.dart';
import 'services/content_service.dart';
import 'services/registration_service.dart';

const forest = Color(0xFF0B3329);
const deepForest = Color(0xFF051C17);
const cream = Color(0xFFF3F1E9);
const paper = Color(0xFFFBFAF5);
const lime = Color(0xFFB9DC72);
const gold = Color(0xFFE4AD54);
const ink = Color(0xFF10201B);
const muted = Color(0xFF616F69);

/// Input outlines. At least 3:1 against white, paper and cream, so a field's
/// edge stays visible on every surface it sits on.
const fieldEdge = Color(0xFF7F8B85);

const _months = [
  'January',
  'February',
  'March',
  'April',
  'May',
  'June',
  'July',
  'August',
  'September',
  'October',
  'November',
  'December',
];

OutlineInputBorder _fieldBorder(Color color, {double width = 1}) =>
    OutlineInputBorder(
      borderRadius: BorderRadius.circular(14),
      borderSide: BorderSide(color: color, width: width),
    );

String eventDateRange(EventDetails event, {bool uppercase = false}) {
  final sameMonth = event.startDate.month == event.endDate.month;
  final start = sameMonth
      ? event.startDate.day.toString().padLeft(2, '0')
      : '${event.startDate.day.toString().padLeft(2, '0')} ${_months[event.startDate.month - 1]}';
  final value =
      '$start-${event.endDate.day.toString().padLeft(2, '0')} '
      '${_months[event.endDate.month - 1]} ${event.endDate.year}';
  return uppercase ? value.toUpperCase() : value;
}

String directionsUrl(EventDetails event) =>
    Uri.https('www.google.com', '/maps/search/', {
      'api': '1',
      'query': [event.venue, event.city].where((s) => s.isNotEmpty).join(', '),
    }).toString();

void main() {
  WidgetsFlutterBinding.ensureInitialized();
  runApp(
    ChangeNotifierProvider(
      create: (_) => ContentProvider(CurrentContentService())..load(),
      child: const BioConnectApp(showSplash: true),
    ),
  );
}

Future<void> openLink(BuildContext context, String url) async {
  try {
    if (await launchUrl(Uri.parse(url), mode: LaunchMode.externalApplication)) {
      return;
    }
  } catch (_) {
    // The owning app may not be installed, or the platform may reject the URL.
  }
  if (context.mounted) {
    ScaffoldMessenger.of(context).showSnackBar(
      const SnackBar(
        content: Text('Could not open this link. Please try again.'),
      ),
    );
  }
}

final _navigationObserver = AppNavigationObserver();

/// Lets a tapped notification open a page from outside the widget tree.
final appNavigator = GlobalKey<NavigatorState>();

/// Store update checks. Only release builds can match a store listing, so
/// debug and test runs never prompt or touch the network.
final _upgrader = Upgrader(
  // The store listings are published for India.
  countryCode: 'IN',
  durationUntilAlertAgain: const Duration(days: 1),
);

class BioConnectApp extends StatelessWidget {
  const BioConnectApp({
    super.key,
    this.checkForUpdates = kReleaseMode,
    this.showSplash = false,
    this.agenda,
    this.contacts,
    this.feedback,
    this.wallet,
  });
  final bool checkForUpdates;

  /// Plays the animated splash on launch; off in tests.
  final bool showSplash;

  /// The attendee's own data on this phone; tests pass in-memory copies.
  final Agenda? agenda;
  final ContactBook? contacts;
  final FeedbackBook? feedback;

  /// Saved passes, shared by every screen so none keeps a stale copy.
  final PassWallet? wallet;

  @override
  Widget build(BuildContext context) => MultiProvider(
    providers: [
      ChangeNotifierProvider(create: (_) => (agenda ?? Agenda())..load()),
      ChangeNotifierProvider(
        create: (_) => (contacts ?? ContactBook())..load(),
      ),
      ChangeNotifierProvider(
        create: (_) => (feedback ?? FeedbackBook())..load(),
      ),
      ChangeNotifierProvider(create: (_) => wallet ?? PassWallet()),
    ],
    child: _app(context),
  );

  Widget _app(BuildContext context) => MaterialApp(
    title: 'Bio Connect 4.0',
    debugShowCheckedModeBanner: false,
    navigatorKey: appNavigator,
    navigatorObservers: [_navigationObserver],
    theme: ThemeData(
      useMaterial3: true,
      fontFamily: 'DM Sans',
      scaffoldBackgroundColor: paper,
      colorScheme: ColorScheme.fromSeed(
        seedColor: forest,
        primary: forest,
        surface: paper,
      ),
      // Inner pages share the main header's cream bar, Manrope title and
      // forest icons, so the top of every page reads as one app.
      appBarTheme: const AppBarTheme(
        backgroundColor: cream,
        foregroundColor: ink,
        surfaceTintColor: Colors.transparent,
        iconTheme: IconThemeData(color: forest),
        actionsIconTheme: IconThemeData(color: forest),
        centerTitle: false,
        scrolledUnderElevation: 0,
        titleTextStyle: TextStyle(
          fontFamily: 'Manrope',
          fontWeight: FontWeight.w700,
          fontSize: 19,
          color: ink,
        ),
      ),
      pageTransitionsTheme: const PageTransitionsTheme(
        builders: {
          TargetPlatform.android: FadeForwardsPageTransitionsBuilder(
            backgroundColor: paper,
          ),
          TargetPlatform.iOS: CupertinoPageTransitionsBuilder(),
        },
      ),
      cardTheme: CardThemeData(
        elevation: 0,
        clipBehavior: Clip.antiAlias,
        shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(16)),
      ),
      filledButtonTheme: FilledButtonThemeData(
        style: FilledButton.styleFrom(
          minimumSize: const Size(0, 52),
          padding: const EdgeInsets.symmetric(horizontal: 22),
          shape: RoundedRectangleBorder(
            borderRadius: BorderRadius.circular(16),
          ),
          textStyle: const TextStyle(
            fontFamily: 'DM Sans',
            fontSize: 15,
            fontWeight: FontWeight.w700,
          ),
        ),
      ),
      outlinedButtonTheme: OutlinedButtonThemeData(
        style: OutlinedButton.styleFrom(
          minimumSize: const Size(0, 50),
          foregroundColor: forest,
          side: BorderSide(color: forest.withValues(alpha: .28)),
          shape: RoundedRectangleBorder(
            borderRadius: BorderRadius.circular(16),
          ),
          textStyle: const TextStyle(
            fontFamily: 'DM Sans',
            fontSize: 14,
            fontWeight: FontWeight.w700,
          ),
        ),
      ),
      // Matches the chips: a selected segment is forest with a lime check.
      segmentedButtonTheme: SegmentedButtonThemeData(
        style: ButtonStyle(
          minimumSize: const WidgetStatePropertyAll(Size(0, 48)),
          side: const WidgetStatePropertyAll(BorderSide(color: fieldEdge)),
          backgroundColor: WidgetStateProperty.resolveWith(
            (states) =>
                states.contains(WidgetState.selected) ? forest : Colors.white,
          ),
          foregroundColor: WidgetStateProperty.resolveWith(
            (states) => states.contains(WidgetState.disabled)
                ? muted
                : states.contains(WidgetState.selected)
                ? Colors.white
                : ink,
          ),
          iconColor: WidgetStateProperty.resolveWith(
            (states) => states.contains(WidgetState.selected) ? lime : forest,
          ),
          textStyle: const WidgetStatePropertyAll(
            TextStyle(
              fontFamily: 'DM Sans',
              fontSize: 14,
              fontWeight: FontWeight.w700,
            ),
          ),
        ),
      ),
      chipTheme: ChipThemeData(
        backgroundColor: cream,
        selectedColor: forest,
        checkmarkColor: lime,
        side: BorderSide.none,
        shape: const StadiumBorder(),
        labelStyle: TextStyle(
          fontFamily: 'DM Sans',
          fontWeight: FontWeight.w600,
          color: WidgetStateColor.resolveWith(
            (states) =>
                states.contains(WidgetState.selected) ? Colors.white : ink,
          ),
        ),
      ),
      snackBarTheme: SnackBarThemeData(
        behavior: SnackBarBehavior.floating,
        backgroundColor: deepForest,
        actionTextColor: lime,
        contentTextStyle: const TextStyle(
          fontFamily: 'DM Sans',
          color: Colors.white,
        ),
        shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(14)),
      ),
      dialogTheme: DialogThemeData(
        backgroundColor: paper,
        shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(24)),
      ),
      progressIndicatorTheme: const ProgressIndicatorThemeData(
        color: forest,
        linearTrackColor: cream,
      ),
      inputDecorationTheme: InputDecorationTheme(
        filled: true,
        fillColor: Colors.white,
        hintStyle: const TextStyle(color: muted),
        border: _fieldBorder(fieldEdge),
        enabledBorder: _fieldBorder(fieldEdge),
        focusedBorder: _fieldBorder(forest, width: 2),
        disabledBorder: _fieldBorder(fieldEdge.withValues(alpha: .4)),
      ),
      textTheme: Theme.of(context).textTheme
          .apply(fontFamily: 'DM Sans', bodyColor: ink, displayColor: ink),
    ),
    home: showSplash ? SplashGate(child: _home()) : _home(),
  );

  Widget _home() => checkForUpdates
      ? UpgradeAlert(
          upgrader: _upgrader,
          showIgnore: false,
          dialogStyle: defaultTargetPlatform == TargetPlatform.iOS
              ? UpgradeDialogStyle.cupertino
              : UpgradeDialogStyle.material,
          child: const AppShell(),
        )
      : const AppShell();
}

class AppShell extends StatefulWidget {
  const AppShell({super.key});
  @override
  State<AppShell> createState() => _AppShellState();
}

class _AppShellState extends State<AppShell> with WidgetsBindingObserver {
  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addObserver(this);
    _startReminders();
  }

  /// Reminder notifications open their session, including one that launched
  /// the app.
  Future<void> _startReminders() async {
    final reminders = context.read<Agenda>().reminders;
    await reminders.init(_openPayload);
    final launch = await reminders.launchPayload();
    if (launch != null) _openPayload(launch);
  }

  void _openPayload(String payload) {
    if (!payload.startsWith('session:') || !mounted) return;
    final id = payload.substring('session:'.length);
    final provider = context.read<ContentProvider>();
    void open() => appNavigator.currentState?.push(
      MaterialPageRoute<void>(builder: (_) => SessionDetailScreen(id)),
    );
    if (provider.content != null) return open();
    void ready() {
      if (provider.content == null) return;
      provider.removeListener(ready);
      open();
    }

    provider.addListener(ready);
  }

  EventContent? _synced;
  List<Exhibitor>? _syncedExhibitors;

  /// Keeps reminders and saved exhibitors in step with refreshed content.
  void _syncAgenda(ContentProvider state) {
    final content = state.content;
    if (identical(content, _synced) &&
        identical(state.exhibitors, _syncedExhibitors)) {
      return;
    }
    _synced = content;
    _syncedExhibitors = state.exhibitors;
    WidgetsBinding.instance.addPostFrameCallback((_) {
      if (!mounted) return;
      final agenda = context.read<Agenda>();
      if (content != null) agenda.syncReminders(content);
      if (state.exhibitorsLoaded) agenda.refreshExhibitors(state.exhibitors);
    });
  }

  @override
  void dispose() {
    WidgetsBinding.instance.removeObserver(this);
    for (final controller in _tabScroll.values) {
      controller.dispose();
    }
    super.dispose();
  }

  @override
  void didChangeAppLifecycleState(AppLifecycleState state) {
    if (state == AppLifecycleState.resumed) {
      context.read<ContentProvider>().load();
    }
  }

  /// Tabs are kept by key, because staff can hide Sessions or Speakers.
  String selected = 'home';
  final _tabScroll = <String, ScrollController>{};
  bool _exitPromptOpen = false;
  List<String> _tabs = const ['home', 'guide'];

  ScrollController _scroll(String key) =>
      _tabScroll.putIfAbsent(key, ScrollController.new);

  bool _selectKey(String key) {
    if (!_tabs.contains(key)) return false;
    _selectTab(key);
    return true;
  }

  void _selectTab(String index) {
    FocusManager.instance.primaryFocus?.unfocus();
    if (selected == index) {
      final controller = _scroll(index);
      if (controller.hasClients && controller.offset > 0) {
        AppFeedback.selection();
        controller.animateTo(
          0,
          duration: const Duration(milliseconds: 280),
          curve: Curves.easeOutCubic,
        );
      }
      return;
    }
    AppFeedback.selection();
    setState(() => selected = index);
  }

  Future<void> _back() async {
    if (_exitPromptOpen) return;
    if (MediaQuery.viewInsetsOf(context).bottom > 0) {
      FocusManager.instance.primaryFocus?.unfocus();
      return;
    }
    if (selected != 'home') {
      _selectTab('home');
      return;
    }
    // iOS has no app-exit navigation. Keep native detail-page swipe gestures.
    if (Theme.of(context).platform == TargetPlatform.iOS) return;
    _exitPromptOpen = true;
    final exit = await showDialog<bool>(
      context: context,
      builder: (context) => AlertDialog(
        title: const Text('Exit Bio Connect?'),
        content: const Text('You can return to the event guide anytime.'),
        actions: [
          TextButton(
            onPressed: () => Navigator.pop(context, false),
            child: const Text('Stay'),
          ),
          FilledButton(
            onPressed: () => Navigator.pop(context, true),
            child: const Text('Exit'),
          ),
        ],
      ),
    );
    _exitPromptOpen = false;
    if (exit == true && mounted) {
      AppFeedback.action();
      await SystemNavigator.pop();
    }
  }

  @override
  Widget build(BuildContext context) => PopScope<void>(
    canPop: false,
    onPopInvokedWithResult: (didPop, result) {
      if (!didPop) _back();
    },
    child: _buildShell(context),
  );

  Widget _buildShell(BuildContext context) {
    final state = context.watch<ContentProvider>();
    _syncAgenda(state);
    final header = AppBar(
      backgroundColor: cream,
      scrolledUnderElevation: 0,
      toolbarHeight: 64,
      titleSpacing: 20,
      title: Image.asset(
        'assets/images/bio-connect-logo.png',
        width: 150,
        height: 44,
        fit: BoxFit.contain,
      ),
      actions: [
        if (state.content != null &&
            visibleMenu(
              state.content!,
              'guide',
            ).any((e) => e.key == 'contacts'))
          IconButton(
            tooltip: 'Scan a badge',
            color: forest,
            onPressed: () => scanBadge(context),
            icon: const Icon(Icons.qr_code_scanner_rounded),
          ),
        Padding(
          padding: const EdgeInsets.only(right: 12),
          child: IconButton(
            tooltip: 'My passes',
            color: forest,
            onPressed: () => Navigator.push(
              context,
              MaterialPageRoute(builder: (_) => const MyPassesScreen()),
            ),
            icon: const Icon(Icons.confirmation_number_outlined),
          ),
        ),
      ],
    );
    if (state.content == null) {
      return Scaffold(
        appBar: header,
        body: Center(
          child: state.loading
              ? const BioLoader(label: 'Preparing your event guide')
              : StateMessage(
                  state.error ?? 'Event content unavailable.',
                  action: 'Try again',
                  onTap: state.load,
                ),
        ),
      );
    }
    final content = state.content!;
    final tabs = visibleTabs(content);
    _tabs = ['home', for (final tab in tabs) tab.key, 'guide'];
    if (!_tabs.contains(selected)) selected = 'home';
    final pages = <String, Widget>{
      'home': AttendeeHomeScreen(content),
      'sessions': const SessionsScreen(),
      'speakers': SpeakersScreen(content.speakers),
      'agenda': const AgendaScreen(),
      'guide': MoreScreen(content),
    };
    return TabScope(
      select: _selectKey,
      child: Scaffold(
        appBar: header,
        body: SafeArea(
          child: IndexedStack(
            index: _tabs.indexOf(selected),
            children: [
              for (final key in _tabs)
                TickerMode(
                  key: ValueKey(key),
                  enabled: selected == key,
                  child: _TabFade(
                    active: selected == key,
                    child: PrimaryScrollController(
                      controller: _scroll(key),
                      child: pages[key]!,
                    ),
                  ),
                ),
            ],
          ),
        ),
        bottomNavigationBar: BioNavBar(
          selectedIndex: _tabs.indexOf(selected),
          onSelected: (index) => _selectTab(_tabs[index]),
          items: [
            const BioNavItem(Icons.home_outlined, Icons.home_rounded, 'Home'),
            for (final tab in tabs)
              switch (tab.key) {
                'sessions' => BioNavItem(
                  Icons.calendar_month_outlined,
                  Icons.calendar_month_rounded,
                  entryTitle(tab),
                ),
                'agenda' => BioNavItem(
                  Icons.bookmarks_outlined,
                  Icons.bookmarks_rounded,
                  entryTitle(tab),
                ),
                _ => BioNavItem(
                  Icons.people_outline_rounded,
                  Icons.people_alt_rounded,
                  entryTitle(tab),
                ),
              },
            const BioNavItem(
              Icons.grid_view_outlined,
              Icons.grid_view_rounded,
              'Guide',
            ),
          ],
        ),
      ),
    );
  }
}

/// Fades a tab in and lifts it slightly each time it becomes the active tab.
class _TabFade extends StatefulWidget {
  const _TabFade({required this.active, required this.child});
  final bool active;
  final Widget child;
  @override
  State<_TabFade> createState() => _TabFadeState();
}

class _TabFadeState extends State<_TabFade>
    with SingleTickerProviderStateMixin {
  late final _controller = AnimationController(
    vsync: this,
    duration: const Duration(milliseconds: 280),
    value: 1,
  );
  late final _curve = CurvedAnimation(
    parent: _controller,
    curve: Curves.easeOutCubic,
  );

  @override
  void didUpdateWidget(_TabFade old) {
    super.didUpdateWidget(old);
    if (widget.active && !old.active) {
      _controller.forward(from: 0);
    }
  }

  @override
  void dispose() {
    _curve.dispose();
    _controller.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) => FadeTransition(
    opacity: _curve,
    child: SlideTransition(
      position: Tween(
        begin: const Offset(0, .015),
        end: Offset.zero,
      ).animate(_curve),
      child: widget.child,
    ),
  );
}

class ExploreScreen extends StatelessWidget {
  const ExploreScreen({super.key});
  @override
  Widget build(BuildContext context) {
    final state = context.watch<ContentProvider>();
    final content = state.content!;
    return LiveRefresh(onRefresh: state.load, child: _exploreList(content));
  }

  Widget _exploreList(EventContent content) => ListView(
    keyboardDismissBehavior: ScrollViewKeyboardDismissBehavior.onDrag,
    physics: const AlwaysScrollableScrollPhysics(),
    padding: const EdgeInsets.fromLTRB(20, 24, 20, 30),
    children: [
      if (content.themes.isNotEmpty) ...[
        Eyebrow(content.text('explore.eyebrow', 'BIO CONNECT 4.0')),
        const SizedBox(height: 7),
        TitleText(
          content.text('explore.title', 'Explore the ideas\nshaping tomorrow.'),
        ),
        const SizedBox(height: 10),
        Text(
          content.text(
            'explore.intro',
            'Five themes drive the conversations, showcases and connections at Bio Connect 4.0.',
          ),
          style: const TextStyle(color: muted, height: 1.45),
        ),
        const SizedBox(height: 21),
        for (var i = 0; i < content.themes.length; i++)
          Padding(
            padding: const EdgeInsets.only(bottom: 10),
            child: ThemeRow(content.themes[i], i + 1),
          ),
        const SizedBox(height: 22),
      ],
      if (content.programmeHighlights.isNotEmpty) ...[
        Eyebrow(content.text('explore.programme_eyebrow', 'THE PROGRAMME')),
        const SizedBox(height: 7),
        TitleText(
          content.text('explore.programme_title', 'Built for connection.'),
        ),
        const SizedBox(height: 10),
        Text(
          content.text(
            'explore.programme_intro',
            'The event brings science, enterprise and policy together through:',
          ),
          style: const TextStyle(color: muted, height: 1.45),
        ),
        const SizedBox(height: 15),
        Wrap(
          spacing: 8,
          runSpacing: 8,
          children: [
            for (final item in content.programmeHighlights)
              Chip(label: Text(item), backgroundColor: cream),
          ],
        ),
        const SizedBox(height: 24),
      ],
      VisitPanel(content),
    ],
  );
}

class SpeakersScreen extends StatefulWidget {
  const SpeakersScreen(this.speakers, {super.key});
  final List<Speaker> speakers;
  @override
  State<SpeakersScreen> createState() => _SpeakersScreenState();
}

class _SpeakersScreenState extends State<SpeakersScreen> {
  final queryController = TextEditingController();
  String query = '';
  @override
  void dispose() {
    queryController.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final visible = widget.speakers
        .where(
          (s) => '${s.name} ${s.role} ${s.organization}'.toLowerCase().contains(
            query.toLowerCase(),
          ),
        )
        .toList();
    final copy =
        context.watch<ContentProvider>().content?.text ?? (_, text) => text;
    final textScale = MediaQuery.textScalerOf(context);
    return LiveRefresh(
      onRefresh: context.read<ContentProvider>().load,
      child: CustomScrollView(
        keyboardDismissBehavior: ScrollViewKeyboardDismissBehavior.onDrag,
        physics: const AlwaysScrollableScrollPhysics(),
        slivers: [
          SliverPadding(
            padding: const EdgeInsets.fromLTRB(20, 24, 20, 0),
            sliver: SliverList.list(
              children: [
                Eyebrow(copy('speakers.eyebrow', 'CONCLAVE SPEAKERS')),
                const SizedBox(height: 7),
                TitleText(
                  copy('speakers.title', 'The voices\ntaking the stage.'),
                ),
                const SizedBox(height: 16),
                TextField(
                  controller: queryController,
                  onTapOutside: (_) => FocusScope.of(context).unfocus(),
                  textInputAction: TextInputAction.search,
                  onSubmitted: (_) => FocusScope.of(context).unfocus(),
                  onChanged: (v) => setState(() => query = v),
                  decoration: InputDecoration(
                    hintText: 'Search name or organisation',
                    prefixIcon: const Icon(Icons.search),
                    suffixIcon: query.isEmpty
                        ? null
                        : IconButton(
                            tooltip: 'Clear search',
                            onPressed: () {
                              AppFeedback.selection();
                              queryController.clear();
                              setState(() => query = '');
                            },
                            icon: const Icon(Icons.close),
                          ),
                  ),
                ),
                const SizedBox(height: 14),
                Align(
                  alignment: Alignment.centerLeft,
                  child: Container(
                    padding: const EdgeInsets.symmetric(
                      horizontal: 11,
                      vertical: 5,
                    ),
                    decoration: BoxDecoration(
                      color: cream,
                      borderRadius: BorderRadius.circular(99),
                    ),
                    child: Text(
                      '${visible.length} ${visible.length == 1 ? 'speaker' : 'speakers'}',
                      style: const TextStyle(
                        color: forest,
                        fontSize: 12,
                        fontWeight: FontWeight.w700,
                      ),
                    ),
                  ),
                ),
                const SizedBox(height: 14),
                if (visible.isEmpty)
                  const StateMessage(
                    'No speakers match your search. Try another name or organisation.',
                  ),
              ],
            ),
          ),
          SliverPadding(
            padding: const EdgeInsets.fromLTRB(20, 0, 20, 30),
            sliver: SliverLayoutBuilder(
              builder: (context, constraints) {
                final columns = constraints.crossAxisExtent > 560 ? 3 : 2;
                const gap = 12.0;
                final width =
                    (constraints.crossAxisExtent - gap * (columns - 1)) /
                    columns;
                return SliverGrid.builder(
                  itemCount: visible.length,
                  gridDelegate: SliverGridDelegateWithFixedCrossAxisCount(
                    crossAxisCount: columns,
                    mainAxisSpacing: gap,
                    crossAxisSpacing: gap,
                    // Portrait plus up to six lines of text.
                    mainAxisExtent: width * 1.1 + textScale.scale(124),
                  ),
                  // Only the first screenful animates in; scrolling stays calm.
                  itemBuilder: (context, i) => i < 6
                      ? Reveal(
                          key: ValueKey(visible[i].id),
                          order: i,
                          child: SpeakerCard(visible[i]),
                        )
                      : SpeakerCard(visible[i], key: ValueKey(visible[i].id)),
                );
              },
            ),
          ),
        ],
      ),
    );
  }
}

class MoreScreen extends StatelessWidget {
  const MoreScreen(this.content, {super.key});
  final EventContent content;
  @override
  Widget build(BuildContext context) => LiveRefresh(
    onRefresh: context.read<ContentProvider>().load,
    child: _guideList(context),
  );

  Widget _guideList(BuildContext context) => ListView(
    keyboardDismissBehavior: ScrollViewKeyboardDismissBehavior.onDrag,
    physics: const AlwaysScrollableScrollPhysics(),
    padding: const EdgeInsets.fromLTRB(20, 24, 20, 30),
    children: [
      Eyebrow(content.text('guide.eyebrow', 'YOUR EVENT GUIDE')),
      const SizedBox(height: 7),
      TitleText(content.text('guide.title', 'Everything in\none place.')),
      const SizedBox(height: 22),
      for (final entry in visibleMenu(content, 'guide'))
        GuideCard(
          entryIcon(entry),
          entryTitle(entry),
          entrySubtitle(entry, content),
          () => openDestination(context, content, entry),
        ),
      const SizedBox(height: 16),
      VisitPanel(content),
    ],
  );
}

class RegistrationScreen extends StatefulWidget {
  const RegistrationScreen(this.event, {super.key, this.service});
  final EventDetails event;
  final RegistrationService? service;

  @override
  State<RegistrationScreen> createState() => _RegistrationScreenState();
}

class _RegistrationScreenState extends State<RegistrationScreen> {
  late final RegistrationService _service;
  RegistrationOptions? _options;
  Object? _error;

  @override
  void initState() {
    super.initState();
    _service = widget.service ?? RegistrationService();
    _load();
  }

  /// Keeps the options already shown when a refresh fails.
  Future<bool> _load() async {
    setState(() => _error = null);
    try {
      final options = await _service.options();
      if (mounted) setState(() => _options = options);
      return true;
    } catch (error) {
      if (mounted && _options == null) setState(() => _error = error);
      return false;
    }
  }

  @override
  Widget build(BuildContext context) {
    final copy =
        context.watch<ContentProvider?>()?.content?.text ?? (_, text) => text;
    return Scaffold(
      appBar: AppBar(title: const Text('Registration')),
      body: LiveRefresh(
        onRefresh: _load,
        child: ListView(
          keyboardDismissBehavior: ScrollViewKeyboardDismissBehavior.onDrag,
          physics: const AlwaysScrollableScrollPhysics(),
          padding: const EdgeInsets.all(20),
          children: [
            TitleText(
              copy(
                'registration.title',
                widget.event.showExhibitorRegistration
                    ? 'Three ways\nto take part.'
                    : 'Register for\nBio Connect.',
              ),
            ),
            const SizedBox(height: 9),
            Text(
              copy(
                'registration.intro',
                widget.event.showExhibitorRegistration
                    ? 'Register for a delegate pass without leaving the app. Exhibition bookings continue on the secure event portal.'
                    : 'Register for a delegate pass without leaving the app.',
              ),
              style: const TextStyle(color: muted, height: 1.5),
            ),
            const SizedBox(height: 24),
            if (_options == null && _error == null)
              const Padding(
                padding: EdgeInsets.symmetric(vertical: 28),
                child: Center(
                  child: BioLoader(label: 'Loading live pass options'),
                ),
              ),
            if (_error != null)
              StateMessage(
                'Live registration options are unavailable. Please try again.',
                action: 'Try again',
                onTap: _load,
              ),
            if (_options case final options?) ...[
              if (!options.enabled)
                const StateMessage(
                  'Registration is not accepting public submissions right now. Current options are shown below.',
                ),
              const Eyebrow('DELEGATE PASSES'),
              const SizedBox(height: 9),
              for (final category in options.categories.where(
                (category) => category.kind == 'delegate',
              ))
                _CategoryRow(category),
              const SizedBox(height: 8),
              const Text(
                'Prices and availability update live from event registration. Invitation-only categories and private complimentary links are not shown.',
                style: TextStyle(color: muted, fontSize: 11, height: 1.4),
              ),
            ],
            const SizedBox(height: 12),
            FilledButton(
              onPressed:
                  _options?.enabled == true &&
                      _options!.categories.any(
                        (category) =>
                            category.kind == 'delegate' && category.open,
                      )
                  ? () => Navigator.push(
                      context,
                      MaterialPageRoute(
                        builder: (_) => const DelegateRegistrationScreen(),
                      ),
                    )
                  : null,
              child: const Text('Register in the app'),
            ),
            if (widget.event.showExhibitorRegistration) ...[
              const SizedBox(height: 27),
              const Eyebrow('EXHIBITION SPACE'),
              const SizedBox(height: 9),
              if (_options case final options?)
                for (final category in options.categories.where(
                  (category) => category.kind == 'exhibitor',
                ))
                  _CategoryRow(category),
              const SizedBox(height: 12),
              FilledButton(
                onPressed: () => openLink(
                  context,
                  '${RegistrationService.apiBaseUrl}/exhibitors',
                ),
                child: const Text('Book exhibition space'),
              ),
            ],
            if (widget.event.sponsorshipEmail.isNotEmpty) ...[
              const SizedBox(height: 27),
              const Eyebrow('SPONSORSHIP'),
              const SizedBox(height: 9),
              Text(
                copy(
                  'registration.sponsorship',
                  'Sponsorships are arranged with the event team.',
                ),
                style: const TextStyle(color: muted),
              ),
              const SizedBox(height: 12),
              OutlinedButton(
                onPressed: () => openLink(
                  context,
                  'mailto:${widget.event.sponsorshipEmail}?subject=Bio%20Connect%204.0%20-%20Sponsorship%20enquiry',
                ),
                child: Text(
                  copy('sponsors.cta_button', 'Enquire about sponsorship'),
                ),
              ),
            ],
          ],
        ),
      ),
    );
  }
}

class ProductLaunchScreen extends StatelessWidget {
  const ProductLaunchScreen({super.key});

  @override
  Widget build(BuildContext context) {
    final state = context.watch<ContentProvider>();
    return Scaffold(
      appBar: AppBar(title: const Text('Product launch')),
      body: LiveRefresh(
        onRefresh: state.load,
        child: _body(context, state.content!),
      ),
    );
  }

  Widget _body(BuildContext context, EventContent content) {
    final launch = content.productLaunch;
    return ListView(
      keyboardDismissBehavior: ScrollViewKeyboardDismissBehavior.onDrag,
      physics: const AlwaysScrollableScrollPhysics(),
      padding: const EdgeInsets.fromLTRB(20, 12, 20, 32),
      children: [
        if (launch.eyebrow.isNotEmpty) ...[
          Eyebrow(launch.eyebrow.toUpperCase()),
          const SizedBox(height: 8),
        ],
        if (launch.title.isNotEmpty) ...[
          TitleText(launch.title),
          const SizedBox(height: 12),
        ],
        if (launch.description.isNotEmpty)
          Text(
            launch.description,
            style: const TextStyle(color: muted, height: 1.5),
          ),
        if (launch.deadline.isNotEmpty) ...[
          const SizedBox(height: 22),
          Container(
            padding: const EdgeInsets.all(18),
            decoration: BoxDecoration(
              color: forest,
              borderRadius: BorderRadius.circular(18),
            ),
            child: Row(
              children: [
                const Icon(Icons.event_available_outlined, color: lime),
                const SizedBox(width: 13),
                Expanded(
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      const Text(
                        'APPLICATION DEADLINE',
                        style: TextStyle(
                          color: lime,
                          fontSize: 10,
                          fontWeight: FontWeight.w700,
                          letterSpacing: 1.2,
                        ),
                      ),
                      const SizedBox(height: 5),
                      Text(
                        launch.deadline,
                        style: const TextStyle(
                          color: Colors.white,
                          fontFamily: 'Manrope',
                          fontSize: 21,
                        ),
                      ),
                    ],
                  ),
                ),
              ],
            ),
          ),
        ],
        if (launch.eligibility.isNotEmpty) ...[
          const SizedBox(height: 24),
          const Eyebrow('WHO CAN APPLY'),
          const SizedBox(height: 10),
          for (var i = 0; i < launch.eligibility.length; i++)
            _InfoRow(
              i == 0
                  ? Icons.rocket_launch_outlined
                  : Icons.business_center_outlined,
              launch.eligibility[i].title,
              launch.eligibility[i].description,
            ),
        ],
        if (launch.focusAreas.isNotEmpty) ...[
          const SizedBox(height: 20),
          const Eyebrow('FOCUS AREAS'),
          const SizedBox(height: 10),
          Wrap(
            spacing: 8,
            runSpacing: 8,
            children: [
              for (final item in launch.focusAreas)
                Chip(label: Text(item), backgroundColor: cream),
            ],
          ),
        ],
        if (launch.applyUrl.isNotEmpty) ...[
          const SizedBox(height: 28),
          FilledButton.icon(
            onPressed: () => openLink(context, launch.applyUrl),
            icon: const Icon(Icons.open_in_new),
            label: Text(
              launch.applyLabel.isEmpty ? 'Apply now' : launch.applyLabel,
            ),
          ),
        ],
      ],
    );
  }
}

class _InfoRow extends StatelessWidget {
  const _InfoRow(this.icon, this.title, this.description);
  final IconData icon;
  final String title, description;

  @override
  Widget build(BuildContext context) => Card(
    color: Colors.white,
    margin: const EdgeInsets.only(bottom: 9),
    child: ListTile(
      leading: CircleAvatar(
        backgroundColor: cream,
        foregroundColor: forest,
        child: Icon(icon),
      ),
      title: Text(title, style: const TextStyle(fontWeight: FontWeight.w700)),
      subtitle: description.isEmpty
          ? null
          : Text(description, style: const TextStyle(color: muted)),
    ),
  );
}

class SpeakerDetailScreen extends StatelessWidget {
  const SpeakerDetailScreen(this.speaker, {super.key});
  final Speaker speaker;

  @override
  Widget build(BuildContext context) {
    final sessions =
        context.watch<ContentProvider?>()?.content?.sessionsOf(speaker) ??
        const [];
    final agenda = context.watch<Agenda?>();
    final width = MediaQuery.sizeOf(context).width;
    final portrait = (width * .6).clamp(160.0, 240.0);
    return Scaffold(
      body: CustomScrollView(
        slivers: [
          DetailHeader(
            title: 'Speaker',
            childHeight: portrait * 1.25,
            child: Hero(
              tag: 'speaker-photo-${speaker.id}',
              child: HeaderFrame(
                width: portrait,
                child: AspectRatio(
                  aspectRatio: 4 / 5,
                  child: ColoredBox(
                    color: cream,
                    child: SpeakerImage(speaker, fit: BoxFit.contain),
                  ),
                ),
              ),
            ),
          ),
          SliverPadding(
            padding: const EdgeInsets.fromLTRB(24, 26, 24, 36),
            sliver: SliverList.list(
              children: [
                const Reveal(child: Eyebrow('CONCLAVE SPEAKER')),
                const SizedBox(height: 10),
                Reveal(
                  order: 1,
                  child: Text(
                    speaker.name,
                    style: const TextStyle(
                      fontFamily: 'Manrope',
                      fontSize: 30,
                      height: 1.15,
                    ),
                  ),
                ),
                if (speaker.role.isNotEmpty || speaker.organization.isNotEmpty)
                  Reveal(
                    order: 2,
                    child: Padding(
                      padding: const EdgeInsets.only(top: 20),
                      child: DetailPanel(
                        children: [
                          if (speaker.role.isNotEmpty)
                            DetailFact(
                              Icons.work_outline_rounded,
                              'Role',
                              speaker.role,
                            ),
                          if (speaker.role.isNotEmpty &&
                              speaker.organization.isNotEmpty)
                            const Divider(height: 1, indent: 62, color: cream),
                          if (speaker.organization.isNotEmpty)
                            DetailFact(
                              Icons.apartment_rounded,
                              'Organisation',
                              speaker.organization,
                            ),
                        ],
                      ),
                    ),
                  ),
                if (agenda != null) ...[
                  const SizedBox(height: 22),
                  SaveButton(
                    saved: agenda.hasSpeaker(speaker.id),
                    saveLabel: 'Save speaker',
                    onPressed: () {
                      AppFeedback.selection();
                      agenda.toggleSpeaker(speaker.id);
                    },
                  ),
                  if (sessions.isNotEmpty && !agenda.hasSpeaker(speaker.id))
                    const Padding(
                      padding: EdgeInsets.only(top: 6),
                      child: Text(
                        'Their sessions are suggested in your agenda.',
                        style: TextStyle(color: muted, fontSize: 12),
                      ),
                    ),
                ],
                if (sessions.isNotEmpty) ...[
                  const SizedBox(height: 28),
                  const Eyebrow('ON STAGE'),
                  const SizedBox(height: 10),
                  for (final s in sessions)
                    GuideCard(
                      Icons.event_note_outlined,
                      s.title,
                      [
                        if (s.startsAt case final start?)
                          '${sessionDay(start)} · ${sessionTime(start)}',
                        if (s.location.isNotEmpty) s.location,
                      ].join('\n'),
                      () => showGuidePage(context, SessionDetailScreen(s.id)),
                    ),
                ],
                if (speaker.linkedin.isNotEmpty) ...[
                  const SizedBox(height: 24),
                  FilledButton.icon(
                    onPressed: () => openLink(context, speaker.linkedin),
                    icon: const Icon(Icons.open_in_new),
                    label: const Text('LinkedIn profile'),
                  ),
                ],
              ],
            ),
          ),
        ],
      ),
    );
  }
}

class Eyebrow extends StatelessWidget {
  const Eyebrow(this.text, {super.key});
  final String text;
  @override
  Widget build(BuildContext context) => Text(
    text,
    style: const TextStyle(
      color: forest,
      fontSize: 10,
      fontWeight: FontWeight.w700,
      letterSpacing: 1.4,
    ),
  );
}

class TitleText extends StatelessWidget {
  const TitleText(this.text, {super.key});
  final String text;
  @override
  Widget build(BuildContext context) => Text(
    text,
    style: const TextStyle(
      fontFamily: 'Manrope',
      fontSize: 30,
      height: 1.13,
      color: ink,
    ),
  );
}

class IconText extends StatelessWidget {
  const IconText(this.icon, this.text, {super.key});
  final IconData icon;
  final String text;
  @override
  Widget build(BuildContext context) => Row(
    children: [
      Icon(icon, color: gold, size: 17),
      const SizedBox(width: 9),
      Expanded(
        child: Text(
          text,
          style: const TextStyle(
            color: Colors.white,
            fontSize: 12,
            fontWeight: FontWeight.w600,
          ),
        ),
      ),
    ],
  );
}

class SectionHeader extends StatelessWidget {
  const SectionHeader(this.title, this.action, this.onTap, {super.key});
  final String title, action;
  final VoidCallback onTap;
  @override
  Widget build(BuildContext context) => Row(
    children: [
      Expanded(
        child: Text(
          title,
          style: const TextStyle(fontFamily: 'Manrope', fontSize: 21),
        ),
      ),
      TextButton(onPressed: onTap, child: Text('$action  →')),
    ],
  );
}

class Metric extends StatelessWidget {
  const Metric(this.number, this.label, this.icon, {super.key});
  final String number, label;
  final IconData icon;
  @override
  Widget build(BuildContext context) => Container(
    padding: const EdgeInsets.all(13),
    decoration: BoxDecoration(
      color: cream,
      borderRadius: BorderRadius.circular(16),
    ),
    child: Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Icon(icon, color: forest, size: 18),
        const SizedBox(height: 15),
        Text(
          number,
          style: const TextStyle(fontFamily: 'Manrope', fontSize: 25),
        ),
        Text(label, style: const TextStyle(color: muted, fontSize: 11)),
      ],
    ),
  );
}

class ThemeTile extends StatelessWidget {
  const ThemeTile(this.theme, {super.key});
  final EventTheme theme;
  @override
  Widget build(BuildContext context) => ClipRRect(
    borderRadius: BorderRadius.circular(17),
    child: Stack(
      fit: StackFit.expand,
      children: [
        ThemeImage(theme.image),
        const DecoratedBox(
          decoration: BoxDecoration(
            gradient: LinearGradient(
              begin: Alignment.topCenter,
              end: Alignment.bottomCenter,
              colors: [Colors.transparent, Color(0xE3051C17)],
            ),
          ),
        ),
        Positioned(
          left: 12,
          right: 9,
          bottom: 12,
          child: Text(
            theme.title,
            style: const TextStyle(
              color: Colors.white,
              fontWeight: FontWeight.w700,
              fontSize: 13,
            ),
          ),
        ),
      ],
    ),
  );
}

class ThemeRow extends StatelessWidget {
  const ThemeRow(this.theme, this.index, {super.key});
  final EventTheme theme;
  final int index;
  @override
  Widget build(BuildContext context) => Container(
    padding: const EdgeInsets.all(10),
    decoration: BoxDecoration(
      color: Colors.white,
      borderRadius: BorderRadius.circular(17),
    ),
    child: Row(
      children: [
        ClipRRect(
          borderRadius: BorderRadius.circular(11),
          child: ThemeImage(theme.image, width: 88, height: 91),
        ),
        const SizedBox(width: 14),
        Expanded(
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Text(
                index.toString().padLeft(2, '0'),
                style: const TextStyle(
                  color: forest,
                  fontSize: 10,
                  fontWeight: FontWeight.w700,
                ),
              ),
              const SizedBox(height: 4),
              Text(
                theme.title,
                style: const TextStyle(fontFamily: 'Manrope', fontSize: 17),
              ),
              if (theme.description.isNotEmpty) ...[
                const SizedBox(height: 5),
                Text(
                  theme.description,
                  style: const TextStyle(
                    color: muted,
                    fontSize: 11,
                    height: 1.3,
                  ),
                ),
              ],
            ],
          ),
        ),
      ],
    ),
  );
}

/// A theme picture shipped with the app, or one staff host online.
class ThemeImage extends StatelessWidget {
  const ThemeImage(this.source, {super.key, this.width, this.height});
  final String source;
  final double? width, height;
  @override
  Widget build(BuildContext context) {
    final fallback = SizedBox(
      width: width,
      height: height,
      child: const ColoredBox(
        color: cream,
        child: Icon(Icons.biotech_outlined, color: forest),
      ),
    );
    if (source.isEmpty) return fallback;
    return source.startsWith('https://')
        ? Image.network(
            source,
            width: width,
            height: height,
            fit: BoxFit.cover,
            errorBuilder: (_, _, _) => fallback,
          )
        : Image.asset(
            source,
            width: width,
            height: height,
            fit: BoxFit.cover,
            errorBuilder: (_, _, _) => fallback,
          );
  }
}

class SpeakerTile extends StatelessWidget {
  const SpeakerTile(this.speaker, {super.key});
  final Speaker speaker;
  @override
  Widget build(BuildContext context) => InkWell(
    onTap: () => Navigator.push(
      context,
      MaterialPageRoute(builder: (_) => SpeakerDetailScreen(speaker)),
    ),
    child: Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Expanded(
          child: ClipRRect(
            borderRadius: BorderRadius.circular(13),
            child: SpeakerImage(speaker, width: double.infinity),
          ),
        ),
        const SizedBox(height: 7),
        Text(
          speaker.name,
          maxLines: 2,
          overflow: TextOverflow.ellipsis,
          style: const TextStyle(fontWeight: FontWeight.w700, fontSize: 11),
        ),
      ],
    ),
  );
}

class SpeakerImage extends StatelessWidget {
  const SpeakerImage(
    this.speaker, {
    super.key,
    this.width,
    this.fit = BoxFit.cover,
  });
  final Speaker speaker;
  final double? width;
  final BoxFit fit;
  @override
  Widget build(BuildContext context) {
    final fallback = _SpeakerFallback(speaker, fit: fit);
    if (speaker.image.isEmpty) return SizedBox(width: width, child: fallback);
    if (speaker.image.startsWith('http')) {
      return CachedPicture(
        speaker.image,
        width: width,
        fit: fit,
        alignment: Alignment.topCenter,
        decodeWidth: width,
        fallback: fallback,
      );
    }
    return Image.asset(
      speaker.image,
      width: width,
      fit: fit,
      alignment: Alignment.topCenter,
      errorBuilder: (_, _, _) => fallback,
    );
  }
}

class _SpeakerFallback extends StatelessWidget {
  const _SpeakerFallback(this.speaker, {required this.fit});
  final BoxFit fit;
  final Speaker speaker;

  @override
  Widget build(BuildContext context) => Image.asset(
    'assets/images/speakers/${speaker.id}.webp',
    fit: fit,
    alignment: Alignment.topCenter,
    errorBuilder: (_, _, _) => ColoredBox(
      color: cream,
      child: Center(
        child: Text(
          speaker.name
              .split(' ')
              .where((part) => part.isNotEmpty)
              .take(2)
              .map((part) => part[0])
              .join(),
          style: const TextStyle(
            color: forest,
            fontFamily: 'Manrope',
            fontSize: 24,
          ),
        ),
      ),
    ),
  );
}

/// A speaker in the directory grid: portrait first, then name and affiliation.
class SpeakerCard extends StatelessWidget {
  const SpeakerCard(this.speaker, {super.key});
  final Speaker speaker;
  @override
  Widget build(BuildContext context) => Pressable(
    child: Material(
      color: Colors.white,
      borderRadius: BorderRadius.circular(20),
      clipBehavior: Clip.antiAlias,
      child: InkWell(
        onTap: () => Navigator.push(
          context,
          MaterialPageRoute(builder: (_) => SpeakerDetailScreen(speaker)),
        ),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.stretch,
          children: [
            AspectRatio(
              aspectRatio: 1 / 1.1,
              child: Hero(
                tag: 'speaker-photo-${speaker.id}',
                child: ColoredBox(
                  color: cream,
                  child: SpeakerImage(speaker, width: double.infinity),
                ),
              ),
            ),
            Expanded(
              child: Padding(
                padding: const EdgeInsets.fromLTRB(12, 10, 12, 10),
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Text(
                      speaker.name,
                      maxLines: 2,
                      overflow: TextOverflow.ellipsis,
                      style: const TextStyle(
                        fontSize: 13.5,
                        height: 1.25,
                        fontWeight: FontWeight.w700,
                      ),
                    ),
                    if (speaker.role.isNotEmpty) ...[
                      const SizedBox(height: 3),
                      Text(
                        speaker.role,
                        maxLines: 2,
                        overflow: TextOverflow.ellipsis,
                        style: const TextStyle(
                          color: muted,
                          fontSize: 11.5,
                          height: 1.3,
                        ),
                      ),
                    ],
                    const Spacer(),
                    if (speaker.organization.isNotEmpty)
                      Text(
                        speaker.organization,
                        maxLines: 2,
                        overflow: TextOverflow.ellipsis,
                        style: const TextStyle(
                          color: forest,
                          fontSize: 11,
                          height: 1.3,
                          fontWeight: FontWeight.w700,
                        ),
                      ),
                  ],
                ),
              ),
            ),
          ],
        ),
      ),
    ),
  );
}

class GuideCard extends StatelessWidget {
  const GuideCard(
    this.icon,
    this.title,
    this.subtitle,
    this.onTap, {
    super.key,
  });
  final IconData icon;
  final String title, subtitle;
  final VoidCallback onTap;
  @override
  Widget build(BuildContext context) => Pressable(
    child: Card(
      color: Colors.white,
      margin: const EdgeInsets.only(bottom: 9),
      child: InkWell(
        onTap: onTap,
        child: Padding(
          padding: const EdgeInsets.all(14),
          child: Row(
            children: [
              Container(
                width: 44,
                height: 44,
                decoration: BoxDecoration(
                  color: cream,
                  borderRadius: BorderRadius.circular(14),
                ),
                child: Icon(icon, color: forest, size: 22),
              ),
              const SizedBox(width: 13),
              Expanded(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Text(
                      title,
                      style: const TextStyle(
                        fontWeight: FontWeight.w700,
                        fontSize: 13,
                      ),
                    ),
                    if (subtitle.isNotEmpty) ...[
                      const SizedBox(height: 4),
                      Text(
                        subtitle,
                        style: const TextStyle(color: muted, fontSize: 11),
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

class VisitPanel extends StatelessWidget {
  const VisitPanel(this.content, {super.key});
  final EventContent content;
  @override
  Widget build(BuildContext context) {
    final event = content.event;
    final city = event.city.split(',').first.trim();
    final place = event.venue.isNotEmpty || event.city.isNotEmpty;
    return Container(
      padding: const EdgeInsets.all(20),
      decoration: BoxDecoration(
        color: forest,
        borderRadius: BorderRadius.circular(18),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(
            content.text('visit.eyebrow', 'PLAN YOUR VISIT'),
            style: const TextStyle(
              color: lime,
              fontSize: 10,
              fontWeight: FontWeight.w700,
              letterSpacing: 1.3,
            ),
          ),
          const SizedBox(height: 9),
          Text(
            content.text(
              'visit.title',
              city.isEmpty ? 'See you there.' : 'See you in\n$city.',
            ),
            style: const TextStyle(
              color: Colors.white,
              fontFamily: 'Manrope',
              fontSize: 24,
            ),
          ),
          const SizedBox(height: 13),
          Text(
            [
              eventDateRange(event),
              if (event.venue.isNotEmpty) event.venue,
            ].join(' · '),
            style: const TextStyle(color: Colors.white70, fontSize: 11),
          ),
          if (place) ...[
            const SizedBox(height: 9),
            TextButton.icon(
              onPressed: () => openLink(context, directionsUrl(event)),
              icon: const Icon(Icons.arrow_outward, size: 16),
              label: const Text('Get directions'),
              style: TextButton.styleFrom(foregroundColor: lime),
            ),
          ],
        ],
      ),
    );
  }
}

class _CategoryRow extends StatelessWidget {
  const _CategoryRow(this.category);
  final RegistrationCategory category;

  String _money(int paise) {
    final digits = (paise ~/ 100).toString();
    if (digits.length <= 3) return '₹$digits';
    final tail = digits.substring(digits.length - 3);
    var head = digits.substring(0, digits.length - 3);
    final groups = <String>[];
    while (head.length > 2) {
      groups.insert(0, head.substring(head.length - 2));
      head = head.substring(0, head.length - 2);
    }
    if (head.isNotEmpty) groups.insert(0, head);
    return '₹${groups.join(',')},$tail';
  }

  @override
  Widget build(BuildContext context) => Container(
    margin: const EdgeInsets.only(bottom: 7),
    padding: const EdgeInsets.symmetric(horizontal: 14, vertical: 13),
    decoration: BoxDecoration(
      color: Colors.white,
      borderRadius: BorderRadius.circular(12),
    ),
    child: Row(
      children: [
        Expanded(
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Text(
                category.label,
                style: const TextStyle(
                  fontWeight: FontWeight.w600,
                  fontSize: 12,
                ),
              ),
              Text(
                '${category.rosterCount} ${category.rosterCount == 1 ? 'pass' : 'passes'}${category.open ? '' : ' · Closed'}',
                style: TextStyle(
                  color: category.open ? muted : Colors.red.shade700,
                  fontSize: 10,
                ),
              ),
            ],
          ),
        ),
        Column(
          crossAxisAlignment: CrossAxisAlignment.end,
          children: [
            Text(
              _money(category.payablePaise),
              style: TextStyle(
                fontWeight: FontWeight.w700,
                color: category.open ? forest : muted,
              ),
            ),
            if (category.earlyPaise != category.regularPaise)
              Text(
                'Early ${_money(category.earlyPaise)} · Regular ${_money(category.regularPaise)}',
                style: const TextStyle(fontSize: 10, color: muted),
              ),
          ],
        ),
      ],
    ),
  );
}

class StateMessage extends StatelessWidget {
  const StateMessage(this.message, {super.key, this.action, this.onTap});
  final String message;
  final String? action;
  final VoidCallback? onTap;
  @override
  Widget build(BuildContext context) => Padding(
    padding: const EdgeInsets.all(24),
    child: Column(
      mainAxisSize: MainAxisSize.min,
      children: [
        const Icon(Icons.info_outline, color: forest),
        const SizedBox(height: 10),
        Text(
          message,
          textAlign: TextAlign.center,
          style: const TextStyle(color: muted, height: 1.5),
        ),
        if (action != null) TextButton(onPressed: onTap, child: Text(action!)),
      ],
    ),
  );
}
