import 'package:flutter/material.dart';
import 'package:provider/provider.dart';
import 'package:url_launcher/url_launcher.dart';

import 'models/event_content.dart';
import 'providers/content_provider.dart';
import 'screens/delegate_registration_screen.dart';
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

String directionsUrl(EventDetails event) => Uri.https(
  'www.google.com',
  '/maps/search/',
  {'api': '1', 'query': '${event.venue}, ${event.city}'},
).toString();

void main() {
  WidgetsFlutterBinding.ensureInitialized();
  runApp(
    ChangeNotifierProvider(
      create: (_) => ContentProvider(CurrentContentService())..load(),
      child: const BioConnectApp(),
    ),
  );
}

Future<void> openLink(BuildContext context, String url) async {
  if (!await launchUrl(Uri.parse(url), mode: LaunchMode.externalApplication) &&
      context.mounted) {
    ScaffoldMessenger.of(context).showSnackBar(
      const SnackBar(
        content: Text('Could not open this link. Please try again.'),
      ),
    );
  }
}

class BioConnectApp extends StatelessWidget {
  const BioConnectApp({super.key});
  @override
  Widget build(BuildContext context) => MaterialApp(
    title: 'Bio Connect 4.0',
    debugShowCheckedModeBanner: false,
    theme: ThemeData(
      useMaterial3: true,
      fontFamily: 'DM Sans',
      scaffoldBackgroundColor: paper,
      colorScheme: ColorScheme.fromSeed(
        seedColor: forest,
        primary: forest,
        surface: paper,
      ),
      appBarTheme: const AppBarTheme(
        backgroundColor: paper,
        foregroundColor: ink,
        centerTitle: false,
        scrolledUnderElevation: 0,
      ),
      cardTheme: CardThemeData(
        elevation: 0,
        shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(16)),
      ),
      inputDecorationTheme: InputDecorationTheme(
        filled: true,
        fillColor: Colors.white,
        border: OutlineInputBorder(
          borderRadius: BorderRadius.circular(14),
          borderSide: BorderSide.none,
        ),
      ),
      textTheme: Theme.of(context).textTheme
          .apply(bodyColor: ink, displayColor: ink),
    ),
    home: const AppShell(),
  );
}

class AppShell extends StatefulWidget {
  const AppShell({super.key});
  @override
  State<AppShell> createState() => _AppShellState();
}

class _AppShellState extends State<AppShell> {
  int selected = 0;
  @override
  Widget build(BuildContext context) {
    final state = context.watch<ContentProvider>();
    if (state.content == null) {
      return Scaffold(
        body: Center(
          child: state.loading
              ? const CircularProgressIndicator()
              : StateMessage(
                  state.error ?? 'Event content unavailable.',
                  action: 'Try again',
                  onTap: state.load,
                ),
        ),
      );
    }
    final content = state.content!;
    return Scaffold(
      body: SafeArea(
        child: IndexedStack(
          index: selected,
          children: [
            HomeScreen(
              content,
              explore: () => setState(() => selected = 1),
              speakers: () => setState(() => selected = 2),
            ),
            ExploreScreen(content),
            SpeakersScreen(content.speakers),
            MoreScreen(content),
          ],
        ),
      ),
      bottomNavigationBar: NavigationBar(
        selectedIndex: selected,
        onDestinationSelected: (index) => setState(() => selected = index),
        backgroundColor: paper,
        indicatorColor: lime.withValues(alpha: .42),
        destinations: const [
          NavigationDestination(
            icon: Icon(Icons.home_outlined),
            selectedIcon: Icon(Icons.home),
            label: 'Home',
          ),
          NavigationDestination(
            icon: Icon(Icons.explore_outlined),
            selectedIcon: Icon(Icons.explore),
            label: 'Explore',
          ),
          NavigationDestination(
            icon: Icon(Icons.people_outline),
            selectedIcon: Icon(Icons.people),
            label: 'Speakers',
          ),
          NavigationDestination(
            icon: Icon(Icons.grid_view_outlined),
            selectedIcon: Icon(Icons.grid_view),
            label: 'More',
          ),
        ],
      ),
    );
  }
}

class HomeScreen extends StatelessWidget {
  const HomeScreen(
    this.content, {
    super.key,
    required this.explore,
    required this.speakers,
  });
  final EventContent content;
  final VoidCallback explore, speakers;
  @override
  Widget build(BuildContext context) => ListView(
    children: [
      Stack(
        children: [
          Positioned.fill(
            child: Image.asset(
              'assets/images/hero-biotech.webp',
              fit: BoxFit.cover,
            ),
          ),
          Container(
            constraints: const BoxConstraints(minHeight: 470),
            padding: const EdgeInsets.fromLTRB(24, 26, 24, 30),
            decoration: const BoxDecoration(
              gradient: LinearGradient(
                begin: Alignment.topLeft,
                end: Alignment.bottomRight,
                colors: [
                  Color(0xF0051C17),
                  Color(0xB00B3329),
                  Color(0xE9051C17),
                ],
              ),
            ),
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Row(
                  children: [
                    Expanded(
                      child: Image.asset(
                        'assets/images/bio-connect-logo.png',
                        height: 50,
                        alignment: Alignment.centerLeft,
                      ),
                    ),
                    const Text(
                      'KERALA 2026',
                      style: TextStyle(
                        color: lime,
                        fontSize: 10,
                        letterSpacing: 1.4,
                        fontWeight: FontWeight.w700,
                      ),
                    ),
                  ],
                ),
                const SizedBox(height: 49),
                const Text(
                  'THE INTERNATIONAL LIFE SCIENCES CONCLAVE & EXPO',
                  style: TextStyle(
                    color: lime,
                    fontSize: 10,
                    letterSpacing: 1.2,
                    fontWeight: FontWeight.w700,
                  ),
                ),
                const SizedBox(height: 13),
                Text(
                  content.event.heroTitle,
                  style: const TextStyle(
                    color: Colors.white,
                    fontFamily: 'Manrope',
                    fontSize: 37,
                    height: 1.09,
                  ),
                ),
                const SizedBox(height: 15),
                Text(
                  content.event.description,
                  style: const TextStyle(
                    color: Colors.white,
                    fontSize: 13,
                    height: 1.5,
                  ),
                ),
                const SizedBox(height: 23),
                IconText(
                  Icons.calendar_month_outlined,
                  eventDateRange(content.event, uppercase: true),
                ),
                const SizedBox(height: 8),
                IconText(Icons.place_outlined, content.event.venue),
                const SizedBox(height: 23),
                FilledButton.icon(
                  onPressed: () => Navigator.push(
                    context,
                    MaterialPageRoute(
                      builder: (_) => const DelegateRegistrationScreen(),
                    ),
                  ),
                  style: FilledButton.styleFrom(
                    backgroundColor: gold,
                    foregroundColor: ink,
                    minimumSize: const Size(double.infinity, 52),
                  ),
                  icon: const Icon(Icons.arrow_outward, size: 18),
                  label: const Text('Register as a delegate'),
                ),
              ],
            ),
          ),
        ],
      ),
      Padding(
        padding: const EdgeInsets.fromLTRB(20, 26, 20, 30),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            const Eyebrow('01 / THE CONCLAVE'),
            const SizedBox(height: 8),
            const TitleText('A place for discovery\nand connection.'),
            const SizedBox(height: 18),
            Row(
              children: [
                const Expanded(
                  child: Metric('2', 'days', Icons.calendar_today_outlined),
                ),
                const SizedBox(width: 9),
                Expanded(
                  child: Metric(
                    '${content.themes.length}',
                    'themes',
                    Icons.science_outlined,
                  ),
                ),
                const SizedBox(width: 9),
                Expanded(
                  child: Metric(
                    '${content.speakers.length}',
                    'speakers',
                    Icons.record_voice_over_outlined,
                  ),
                ),
              ],
            ),
            const SizedBox(height: 30),
            SectionHeader('Five themes. One future.', 'Explore all', explore),
            const SizedBox(height: 12),
            SizedBox(
              height: 182,
              child: ListView.separated(
                scrollDirection: Axis.horizontal,
                itemCount: content.themes.length,
                separatorBuilder: (_, _) => const SizedBox(width: 10),
                itemBuilder: (context, i) => GestureDetector(
                  onTap: explore,
                  child: SizedBox(
                    width: 150,
                    child: ThemeTile(content.themes[i]),
                  ),
                ),
              ),
            ),
            const SizedBox(height: 30),
            SectionHeader('Voices on stage', 'Meet all', speakers),
            const SizedBox(height: 12),
            SizedBox(
              height: 200,
              child: ListView.separated(
                scrollDirection: Axis.horizontal,
                itemCount: content.speakers.length.clamp(0, 8),
                separatorBuilder: (_, _) => const SizedBox(width: 10),
                itemBuilder: (context, i) => SizedBox(
                  width: 135,
                  child: SpeakerTile(content.speakers[i]),
                ),
              ),
            ),
            const SizedBox(height: 30),
            VisitPanel(content.event),
          ],
        ),
      ),
    ],
  );
}

class ExploreScreen extends StatelessWidget {
  const ExploreScreen(this.content, {super.key});
  final EventContent content;
  @override
  Widget build(BuildContext context) => ListView(
    padding: const EdgeInsets.fromLTRB(20, 24, 20, 30),
    children: [
      const Eyebrow('BIO CONNECT 4.0'),
      const SizedBox(height: 7),
      const TitleText('Explore the ideas\nshaping tomorrow.'),
      const SizedBox(height: 10),
      const Text(
        'Five themes drive the conversations, showcases and connections at Bio Connect 4.0.',
        style: TextStyle(color: muted, height: 1.45),
      ),
      const SizedBox(height: 21),
      for (var i = 0; i < content.themes.length; i++)
        Padding(
          padding: const EdgeInsets.only(bottom: 10),
          child: ThemeRow(content.themes[i], i + 1),
        ),
      const SizedBox(height: 22),
      const Eyebrow('THE PROGRAMME'),
      const SizedBox(height: 7),
      const TitleText('Built for connection.'),
      const SizedBox(height: 10),
      const Text(
        'The event brings science, enterprise and policy together through:',
        style: TextStyle(color: muted, height: 1.45),
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
      VisitPanel(content.event),
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
    return ListView(
      padding: const EdgeInsets.fromLTRB(20, 24, 20, 30),
      children: [
        const Eyebrow('CONCLAVE SPEAKERS'),
        const SizedBox(height: 7),
        const TitleText('The voices\ntaking the stage.'),
        const SizedBox(height: 16),
        TextField(
          controller: queryController,
          onChanged: (v) => setState(() => query = v),
          decoration: InputDecoration(
            hintText: 'Search name or organisation',
            prefixIcon: const Icon(Icons.search),
            suffixIcon: query.isEmpty
                ? null
                : IconButton(
                    tooltip: 'Clear search',
                    onPressed: () {
                      queryController.clear();
                      setState(() => query = '');
                    },
                    icon: const Icon(Icons.close),
                  ),
            filled: true,
            fillColor: Colors.white,
            border: OutlineInputBorder(
              borderRadius: BorderRadius.circular(15),
              borderSide: BorderSide.none,
            ),
          ),
        ),
        const SizedBox(height: 12),
        Text(
          '${visible.length} speakers',
          style: const TextStyle(color: muted, fontSize: 12),
        ),
        const SizedBox(height: 14),
        if (visible.isEmpty)
          const StateMessage(
            'No speakers match your search. Try another name or organisation.',
          ),
        for (final s in visible) SpeakerRow(s),
      ],
    );
  }
}

class MoreScreen extends StatelessWidget {
  const MoreScreen(this.content, {super.key});
  final EventContent content;
  @override
  Widget build(BuildContext context) => ListView(
    padding: const EdgeInsets.fromLTRB(20, 24, 20, 30),
    children: [
      const Eyebrow('YOUR EVENT GUIDE'),
      const SizedBox(height: 7),
      const TitleText('Everything in\none place.'),
      const SizedBox(height: 22),
      GuideCard(
        Icons.storefront_outlined,
        'Exhibitors',
        'Confirmed expo line-up',
        () {
          context.read<ContentProvider>().loadExhibitors();
          Navigator.push(
            context,
            MaterialPageRoute(builder: (_) => const ExhibitorsScreen()),
          );
        },
      ),
      GuideCard(
        Icons.confirmation_number_outlined,
        'Registration & passes',
        'Delegate and exhibition options',
        () => Navigator.push(
          context,
          MaterialPageRoute(builder: (_) => RegistrationScreen(content.event)),
        ),
      ),
      GuideCard(
        Icons.place_outlined,
        'Venue & directions',
        content.event.venue,
        () => openLink(context, directionsUrl(content.event)),
      ),
      GuideCard(
        Icons.article_outlined,
        'Event brochure',
        'View the programme overview',
        () => openLink(context, content.event.brochureUrl),
      ),
      GuideCard(
        Icons.rocket_launch_outlined,
        'Product launch',
        'Kerala Startup Mission showcase',
        () => Navigator.push(
          context,
          MaterialPageRoute(builder: (_) => ProductLaunchScreen(content)),
        ),
      ),
      GuideCard(
        Icons.groups_outlined,
        'Leadership & sponsors',
        'People and partners behind the event',
        () => Navigator.push(
          context,
          MaterialPageRoute(builder: (_) => PartnersScreen(content)),
        ),
      ),
      const SizedBox(height: 16),
      VisitPanel(content.event),
    ],
  );
}

class ExhibitorsScreen extends StatefulWidget {
  const ExhibitorsScreen({super.key});
  @override
  State<ExhibitorsScreen> createState() => _ExhibitorsScreenState();
}

class _ExhibitorsScreenState extends State<ExhibitorsScreen> {
  String query = '';
  @override
  Widget build(BuildContext context) {
    final state = context.watch<ContentProvider>();
    final visible = state.exhibitors
        .where(
          (e) => '${e.name} ${e.description}'.toLowerCase().contains(
            query.toLowerCase(),
          ),
        )
        .toList();
    return Scaffold(
      appBar: AppBar(title: const Text('Exhibitors')),
      body: ListView(
        padding: const EdgeInsets.all(20),
        children: [
          const TitleText('Meet your next\ncollaborator.'),
          const SizedBox(height: 18),
          if (state.exhibitorsLoading)
            const Center(
              child: Padding(
                padding: EdgeInsets.all(32),
                child: CircularProgressIndicator(),
              ),
            ),
          if (state.exhibitorsError != null)
            StateMessage(
              state.exhibitorsError!,
              action: 'Try again',
              onTap: state.loadExhibitors,
            ),
          if (!state.exhibitorsLoading && state.exhibitorsError == null) ...[
            TextField(
              onChanged: (v) => setState(() => query = v),
              decoration: InputDecoration(
                hintText: 'Search organisations or expertise',
                prefixIcon: const Icon(Icons.search),
                filled: true,
                fillColor: Colors.white,
                border: OutlineInputBorder(
                  borderRadius: BorderRadius.circular(15),
                  borderSide: BorderSide.none,
                ),
              ),
            ),
            const SizedBox(height: 12),
            Text(
              '${visible.length} confirmed exhibitors',
              style: const TextStyle(color: muted, fontSize: 12),
            ),
            const SizedBox(height: 14),
            if (visible.isEmpty)
              const StateMessage(
                'No exhibitors match yet. Try another search or check back soon.',
              ),
            for (final e in visible)
              Card(
                color: Colors.white,
                margin: const EdgeInsets.only(bottom: 10),
                child: Padding(
                  padding: const EdgeInsets.all(16),
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      if (e.logoUrl.isNotEmpty) ...[
                        Align(
                          alignment: Alignment.centerLeft,
                          child: Image.network(
                            e.logoUrl,
                            height: 52,
                            errorBuilder: (_, _, _) => const SizedBox.shrink(),
                          ),
                        ),
                        const SizedBox(height: 12),
                      ],
                      Text(
                        e.name,
                        style: const TextStyle(
                          fontSize: 16,
                          fontWeight: FontWeight.w700,
                        ),
                      ),
                      const SizedBox(height: 7),
                      Text(
                        e.description,
                        style: const TextStyle(color: muted, height: 1.45),
                      ),
                    ],
                  ),
                ),
              ),
          ],
        ],
      ),
    );
  }
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

  Future<void> _load() async {
    setState(() => _error = null);
    try {
      final options = await _service.options();
      if (mounted) setState(() => _options = options);
    } catch (error) {
      if (mounted) setState(() => _error = error);
    }
  }

  @override
  Widget build(BuildContext context) => Scaffold(
    appBar: AppBar(title: const Text('Registration')),
    body: ListView(
      padding: const EdgeInsets.all(20),
      children: [
        const TitleText('Three ways\nto take part.'),
        const SizedBox(height: 9),
        const Text(
          'Register for a delegate pass without leaving the app. Exhibition bookings continue on the secure event portal.',
          style: TextStyle(color: muted, height: 1.5),
        ),
        const SizedBox(height: 24),
        if (_options == null && _error == null)
          const Center(child: CircularProgressIndicator()),
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
                    (category) => category.kind == 'delegate' && category.open,
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
          onPressed: () =>
              openLink(context, '${RegistrationService.apiBaseUrl}/exhibitors'),
          child: const Text('Book exhibition space'),
        ),
        const SizedBox(height: 27),
        const Eyebrow('SPONSORSHIP'),
        const SizedBox(height: 9),
        const Text(
          'Sponsorships are arranged with the event team.',
          style: TextStyle(color: muted),
        ),
        const SizedBox(height: 12),
        OutlinedButton(
          onPressed: () => openLink(
            context,
            'mailto:${widget.event.sponsorshipEmail}?subject=Bio%20Connect%204.0%20-%20Sponsorship%20enquiry',
          ),
          child: const Text('Enquire about sponsorship'),
        ),
      ],
    ),
  );
}

class PartnersScreen extends StatelessWidget {
  const PartnersScreen(this.content, {super.key});
  final EventContent content;
  @override
  Widget build(BuildContext context) => Scaffold(
    appBar: AppBar(title: const Text('People & partners')),
    body: ListView(
      padding: const EdgeInsets.all(20),
      children: [
        const TitleText('A shared vision\nfor life sciences.'),
        const SizedBox(height: 18),
        Text(
          content.leadership.intro,
          style: TextStyle(color: muted, height: 1.5),
        ),
        const SizedBox(height: 24),
        const Eyebrow('STATE LEADERSHIP'),
        const SizedBox(height: 10),
        for (final person in content.leadership.people)
          _PersonCard(
            name: person.name,
            role: person.role,
            badge: person.badge,
          ),
        const SizedBox(height: 20),
        const Eyebrow('ADVISORY COMMITTEE'),
        const SizedBox(height: 10),
        Text(
          content.leadership.advisoryNote,
          style: TextStyle(color: muted, height: 1.5),
        ),
        const SizedBox(height: 24),
        const Eyebrow('SPONSOR'),
        const SizedBox(height: 10),
        Card(
          color: Colors.white,
          child: Padding(
            padding: const EdgeInsets.all(18),
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Image.network(
                  content.sponsor.logoUrl,
                  height: 66,
                  errorBuilder: (_, _, _) => Image.asset(
                    'assets/images/kerala-rubber-logo.png',
                    height: 66,
                  ),
                ),
                const SizedBox(height: 12),
                Text(
                  content.sponsor.name,
                  style: TextStyle(fontSize: 17, fontWeight: FontWeight.w700),
                ),
                const SizedBox(height: 8),
                Text(
                  content.sponsor.description,
                  style: TextStyle(color: muted, height: 1.45),
                ),
              ],
            ),
          ),
        ),
        const SizedBox(height: 12),
        OutlinedButton.icon(
          onPressed: () => openLink(context, content.sponsor.websiteUrl),
          icon: const Icon(Icons.open_in_new),
          label: const Text('Visit sponsor website'),
        ),
        const SizedBox(height: 28),
        const Eyebrow('ECOSYSTEM PARTNERS'),
        const SizedBox(height: 10),
        for (final partner in content.ecosystemPartners)
          Card(
            color: Colors.white,
            margin: const EdgeInsets.only(bottom: 9),
            child: Padding(
              padding: const EdgeInsets.all(16),
              child: Row(
                children: [
                  SizedBox(
                    width: 68,
                    height: 44,
                    child: Image.network(
                      partner.logoUrl,
                      fit: BoxFit.contain,
                      errorBuilder: (_, _, _) => const Icon(
                        Icons.account_balance_outlined,
                        color: forest,
                      ),
                    ),
                  ),
                  const SizedBox(width: 14),
                  Expanded(
                    child: Text(
                      partner.name,
                      style: const TextStyle(fontWeight: FontWeight.w700),
                    ),
                  ),
                ],
              ),
            ),
          ),
      ],
    ),
  );
}

class ProductLaunchScreen extends StatelessWidget {
  const ProductLaunchScreen(this.content, {super.key});
  final EventContent content;

  @override
  Widget build(BuildContext context) => Scaffold(
    appBar: AppBar(title: const Text('Product launch')),
    body: ListView(
      padding: const EdgeInsets.fromLTRB(20, 12, 20, 32),
      children: [
        Eyebrow(content.productLaunch.eyebrow.toUpperCase()),
        const SizedBox(height: 8),
        TitleText(content.productLaunch.title),
        const SizedBox(height: 12),
        Text(
          content.productLaunch.description,
          style: TextStyle(color: muted, height: 1.5),
        ),
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
                      content.productLaunch.deadline,
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
        const SizedBox(height: 24),
        const Eyebrow('WHO CAN APPLY'),
        const SizedBox(height: 10),
        for (var i = 0; i < content.productLaunch.eligibility.length; i++)
          _InfoRow(
            i == 0
                ? Icons.rocket_launch_outlined
                : Icons.business_center_outlined,
            content.productLaunch.eligibility[i].title,
            content.productLaunch.eligibility[i].description,
          ),
        const SizedBox(height: 20),
        const Eyebrow('FOCUS AREAS'),
        const SizedBox(height: 10),
        Wrap(
          spacing: 8,
          runSpacing: 8,
          children: [
            for (final item in content.productLaunch.focusAreas)
              Chip(label: Text(item), backgroundColor: cream),
          ],
        ),
        const SizedBox(height: 28),
        FilledButton.icon(
          onPressed: () => openLink(context, content.productLaunch.applyUrl),
          icon: const Icon(Icons.open_in_new),
          label: const Text('Apply via Kerala Startup Mission'),
        ),
      ],
    ),
  );
}

class _PersonCard extends StatelessWidget {
  const _PersonCard({
    required this.name,
    required this.role,
    required this.badge,
  });
  final String name, role, badge;

  @override
  Widget build(BuildContext context) => Card(
    color: Colors.white,
    margin: const EdgeInsets.only(bottom: 9),
    child: Padding(
      padding: const EdgeInsets.all(16),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          const CircleAvatar(
            backgroundColor: cream,
            foregroundColor: forest,
            child: Icon(Icons.account_balance_outlined),
          ),
          const SizedBox(width: 13),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  badge.toUpperCase(),
                  style: const TextStyle(
                    color: forest,
                    fontSize: 9,
                    fontWeight: FontWeight.w700,
                    letterSpacing: 1,
                  ),
                ),
                const SizedBox(height: 5),
                Text(
                  name,
                  style: const TextStyle(fontFamily: 'Manrope', fontSize: 17),
                ),
                const SizedBox(height: 4),
                Text(
                  role,
                  style: const TextStyle(
                    color: muted,
                    fontSize: 12,
                    height: 1.4,
                  ),
                ),
              ],
            ),
          ),
        ],
      ),
    ),
  );
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
      subtitle: Text(description, style: const TextStyle(color: muted)),
    ),
  );
}

class SpeakerDetailScreen extends StatelessWidget {
  const SpeakerDetailScreen(this.speaker, {super.key});
  final Speaker speaker;
  @override
  Widget build(BuildContext context) => Scaffold(
    appBar: AppBar(title: const Text('Speaker')),
    body: ListView(
      children: [
        AspectRatio(
          aspectRatio: 1.4,
          child: SpeakerImage(speaker, width: double.infinity),
        ),
        Padding(
          padding: const EdgeInsets.all(24),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              const Eyebrow('CONCLAVE SPEAKER'),
              const SizedBox(height: 10),
              Text(
                speaker.name,
                style: const TextStyle(fontFamily: 'Manrope', fontSize: 30),
              ),
              const SizedBox(height: 12),
              Text(
                speaker.role,
                style: const TextStyle(
                  fontWeight: FontWeight.w700,
                  fontSize: 16,
                ),
              ),
              const SizedBox(height: 5),
              Text(
                speaker.organization,
                style: const TextStyle(color: muted, fontSize: 15, height: 1.4),
              ),
              if (speaker.linkedin.isNotEmpty) ...[
                const SizedBox(height: 23),
                OutlinedButton.icon(
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
      Text(
        text,
        style: const TextStyle(
          color: Colors.white,
          fontSize: 12,
          fontWeight: FontWeight.w600,
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
        Image.asset(theme.image, fit: BoxFit.cover),
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
          child: Image.asset(
            theme.image,
            width: 88,
            height: 91,
            fit: BoxFit.cover,
          ),
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
              const SizedBox(height: 5),
              Text(
                theme.description,
                style: const TextStyle(color: muted, fontSize: 11, height: 1.3),
              ),
            ],
          ),
        ),
      ],
    ),
  );
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
  const SpeakerImage(this.speaker, {super.key, this.width});
  final Speaker speaker;
  final double? width;
  @override
  Widget build(BuildContext context) {
    final fallback = _SpeakerFallback(speaker);
    if (speaker.image.isEmpty) return SizedBox(width: width, child: fallback);
    if (speaker.image.startsWith('http')) {
      return Image.network(
        speaker.image,
        width: width,
        fit: BoxFit.cover,
        alignment: Alignment.topCenter,
        errorBuilder: (_, _, _) => fallback,
      );
    }
    return Image.asset(
      speaker.image,
      width: width,
      fit: BoxFit.cover,
      alignment: Alignment.topCenter,
      errorBuilder: (_, _, _) => fallback,
    );
  }
}

class _SpeakerFallback extends StatelessWidget {
  const _SpeakerFallback(this.speaker);
  final Speaker speaker;

  @override
  Widget build(BuildContext context) => Image.asset(
    'assets/images/speakers/${speaker.id}.webp',
    fit: BoxFit.cover,
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

class SpeakerRow extends StatelessWidget {
  const SpeakerRow(this.speaker, {super.key});
  final Speaker speaker;
  @override
  Widget build(BuildContext context) => Card(
    color: Colors.white,
    margin: const EdgeInsets.only(bottom: 9),
    child: InkWell(
      onTap: () => Navigator.push(
        context,
        MaterialPageRoute(builder: (_) => SpeakerDetailScreen(speaker)),
      ),
      child: Padding(
        padding: const EdgeInsets.all(9),
        child: Row(
          children: [
            ClipRRect(
              borderRadius: BorderRadius.circular(9),
              child: SizedBox(
                width: 70,
                height: 76,
                child: SpeakerImage(speaker, width: 70),
              ),
            ),
            const SizedBox(width: 12),
            Expanded(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text(
                    speaker.name,
                    style: const TextStyle(
                      fontSize: 13,
                      fontWeight: FontWeight.w700,
                    ),
                  ),
                  const SizedBox(height: 4),
                  Text(
                    speaker.role,
                    maxLines: 2,
                    overflow: TextOverflow.ellipsis,
                    style: const TextStyle(color: muted, fontSize: 11),
                  ),
                  const SizedBox(height: 3),
                  Text(
                    speaker.organization,
                    maxLines: 2,
                    overflow: TextOverflow.ellipsis,
                    style: const TextStyle(color: muted, fontSize: 11),
                  ),
                ],
              ),
            ),
            const Icon(Icons.chevron_right, color: forest),
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
  Widget build(BuildContext context) => Card(
    color: Colors.white,
    margin: const EdgeInsets.only(bottom: 9),
    child: InkWell(
      onTap: onTap,
      child: Padding(
        padding: const EdgeInsets.all(14),
        child: Row(
          children: [
            CircleAvatar(
              backgroundColor: cream,
              foregroundColor: forest,
              child: Icon(icon),
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
                  const SizedBox(height: 4),
                  Text(
                    subtitle,
                    style: const TextStyle(color: muted, fontSize: 11),
                  ),
                ],
              ),
            ),
            const Icon(Icons.chevron_right, color: forest),
          ],
        ),
      ),
    ),
  );
}

class VisitPanel extends StatelessWidget {
  const VisitPanel(this.event, {super.key});
  final EventDetails event;
  @override
  Widget build(BuildContext context) => Container(
    padding: const EdgeInsets.all(20),
    decoration: BoxDecoration(
      color: forest,
      borderRadius: BorderRadius.circular(18),
    ),
    child: Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        const Text(
          'PLAN YOUR VISIT',
          style: TextStyle(
            color: lime,
            fontSize: 10,
            fontWeight: FontWeight.w700,
            letterSpacing: 1.3,
          ),
        ),
        const SizedBox(height: 9),
        Text(
          'See you in\n${event.city.split(',').first}.',
          style: const TextStyle(
            color: Colors.white,
            fontFamily: 'Manrope',
            fontSize: 24,
          ),
        ),
        const SizedBox(height: 13),
        Text(
          '${eventDateRange(event)} · ${event.venue}',
          style: const TextStyle(color: Colors.white70, fontSize: 11),
        ),
        const SizedBox(height: 9),
        TextButton.icon(
          onPressed: () => openLink(context, directionsUrl(event)),
          icon: const Icon(Icons.arrow_outward, size: 16),
          label: const Text('Get directions'),
          style: TextButton.styleFrom(foregroundColor: lime),
        ),
      ],
    ),
  );
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
