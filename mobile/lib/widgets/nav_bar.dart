import 'package:flutter/material.dart';

import '../main.dart';

class BioNavItem {
  const BioNavItem(this.icon, this.selectedIcon, this.label);
  final IconData icon, selectedIcon;
  final String label;
}

/// A floating forest-green tab bar. A lime pill glides to the selected tab and
/// its icon lifts in.
class BioNavBar extends StatelessWidget {
  const BioNavBar({
    super.key,
    required this.items,
    required this.selectedIndex,
    required this.onSelected,
  });
  final List<BioNavItem> items;
  final int selectedIndex;
  final ValueChanged<int> onSelected;

  static const _pill = Size(58, 32);

  @override
  Widget build(BuildContext context) => ColoredBox(
    color: paper,
    child: SafeArea(
      top: false,
      minimum: const EdgeInsets.only(bottom: 8),
      child: Padding(
        padding: const EdgeInsets.fromLTRB(14, 6, 14, 4),
        child: DecoratedBox(
          decoration: BoxDecoration(
            gradient: const LinearGradient(
              begin: Alignment.topLeft,
              end: Alignment.bottomRight,
              colors: [forest, deepForest],
            ),
            borderRadius: BorderRadius.circular(26),
            boxShadow: [
              BoxShadow(
                color: deepForest.withValues(alpha: .22),
                blurRadius: 22,
                offset: const Offset(0, 8),
              ),
            ],
          ),
          child: SizedBox(
            height: 70,
            child: LayoutBuilder(
              builder: (context, box) {
                final cell = box.maxWidth / items.length;
                return Stack(
                  children: [
                    AnimatedPositioned(
                      duration: const Duration(milliseconds: 420),
                      curve: Curves.easeOutBack,
                      top: 10,
                      left: cell * selectedIndex + (cell - _pill.width) / 2,
                      width: _pill.width,
                      height: _pill.height,
                      child: const DecoratedBox(
                        decoration: BoxDecoration(
                          color: lime,
                          borderRadius: BorderRadius.all(Radius.circular(16)),
                        ),
                      ),
                    ),
                    Row(
                      children: [
                        for (var i = 0; i < items.length; i++)
                          Expanded(
                            child: _NavButton(
                              item: items[i],
                              selected: i == selectedIndex,
                              onTap: () => onSelected(i),
                            ),
                          ),
                      ],
                    ),
                  ],
                );
              },
            ),
          ),
        ),
      ),
    ),
  );
}

class _NavButton extends StatelessWidget {
  const _NavButton({
    required this.item,
    required this.selected,
    required this.onTap,
  });
  final BioNavItem item;
  final bool selected;
  final VoidCallback onTap;

  @override
  Widget build(BuildContext context) => Semantics(
    button: true,
    selected: selected,
    label: item.label,
    excludeSemantics: true,
    child: InkResponse(
      onTap: onTap,
      radius: 36,
      highlightColor: Colors.transparent,
      splashColor: lime.withValues(alpha: .16),
      child: Column(
        mainAxisAlignment: MainAxisAlignment.center,
        children: [
          SizedBox(
            height: BioNavBar._pill.height,
            child: Center(
              child: AnimatedScale(
                scale: selected ? 1.08 : 1,
                duration: const Duration(milliseconds: 320),
                curve: Curves.easeOutBack,
                child: AnimatedSwitcher(
                  duration: const Duration(milliseconds: 220),
                  transitionBuilder: (child, animation) => ScaleTransition(
                    scale: Tween(begin: .6, end: 1.0).animate(animation),
                    child: FadeTransition(opacity: animation, child: child),
                  ),
                  child: Icon(
                    selected ? item.selectedIcon : item.icon,
                    key: ValueKey(selected),
                    size: 22,
                    color: selected ? forest : Colors.white70,
                  ),
                ),
              ),
            ),
          ),
          const SizedBox(height: 4),
          AnimatedDefaultTextStyle(
            duration: const Duration(milliseconds: 220),
            style: TextStyle(
              fontFamily: 'DM Sans',
              fontSize: 11,
              fontWeight: selected ? FontWeight.w700 : FontWeight.w500,
              color: selected ? Colors.white : Colors.white60,
              letterSpacing: .2,
            ),
            child: Text(
              item.label,
              maxLines: 1,
              overflow: TextOverflow.ellipsis,
            ),
          ),
        ],
      ),
    ),
  );
}
