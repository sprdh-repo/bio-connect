import 'package:flutter/material.dart';
import 'package:flutter/services.dart';

/// Discrete feedback only. Native platform settings control vibration support.
abstract final class AppFeedback {
  static void selection() {
    HapticFeedback.selectionClick();
  }

  static void action() {
    HapticFeedback.lightImpact();
  }

  static void success() {
    HapticFeedback.mediumImpact();
  }

  static Future<void> refresh(Future<void> Function() load) async {
    selection();
    await load();
  }
}

/// Covers every pushed detail page without changing native back/swipe behavior.
class AppNavigationObserver extends NavigatorObserver {
  @override
  void didPush(Route<dynamic> route, Route<dynamic>? previousRoute) {
    if (previousRoute != null && route is PageRoute) {
      FocusManager.instance.primaryFocus?.unfocus();
      AppFeedback.action();
    }
  }

  @override
  void didPop(Route<dynamic> route, Route<dynamic>? previousRoute) {
    if (route is PageRoute && previousRoute != null) {
      AppFeedback.selection();
    }
  }
}

class GuideSearchField extends StatefulWidget {
  const GuideSearchField({
    super.key,
    required this.label,
    required this.onChanged,
  });
  final String label;
  final ValueChanged<String> onChanged;
  @override
  State<GuideSearchField> createState() => _GuideSearchFieldState();
}

class _GuideSearchFieldState extends State<GuideSearchField> {
  final _controller = TextEditingController();
  @override
  void dispose() {
    _controller.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) => TextField(
    controller: _controller,
    textInputAction: TextInputAction.search,
    onSubmitted: (_) => FocusScope.of(context).unfocus(),
    onTapOutside: (_) => FocusScope.of(context).unfocus(),
    onChanged: (value) {
      setState(() {});
      widget.onChanged(value);
    },
    decoration: InputDecoration(
      labelText: widget.label,
      prefixIcon: const Icon(Icons.search),
      suffixIcon: _controller.text.isEmpty
          ? null
          : IconButton(
              tooltip: 'Clear search',
              icon: const Icon(Icons.close),
              onPressed: () {
                AppFeedback.selection();
                _controller.clear();
                setState(() {});
                widget.onChanged('');
              },
            ),
    ),
  );
}
