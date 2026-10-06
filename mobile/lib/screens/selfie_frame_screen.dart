import 'dart:async';
import 'dart:io';
import 'dart:typed_data';
import 'dart:ui' as ui;

import 'package:camera/camera.dart';
import 'package:flutter/material.dart';
import 'package:flutter/rendering.dart';
import 'package:image_picker/image_picker.dart';
import 'package:provider/provider.dart';

import '../main.dart';
import '../models/event_content.dart';
import '../providers/content_provider.dart';
import '../services/photo_export.dart';
import '../widgets/interaction.dart';
import '../widgets/motion.dart';

enum FrameDesign {
  spotlight('Spotlight'),
  arch('Arch'),
  editorial('Editorial');

  const FrameDesign(this.label);
  final String label;
}

/// The shared picture's shape. The frame is laid out at this logical size and
/// exported three times larger: 1080 x 1350 for a feed post, 1080 x 1920 for a
/// story or status.
enum FrameFormat {
  post('Post', Size(360, 450)),
  story('Story', Size(360, 640));

  const FrameFormat(this.label, this.size);
  final String label;
  final Size size;
}

const frameCaptions = [
  "I'm attending",
  'See you at',
  "I'm speaking at",
  "I'm exhibiting at",
];

/// Exported width in pixels for every format.
const frameExportWidth = 1080.0;

/// Take a selfie or pick a photo, place it in an event frame and share it.
class SelfieFrameScreen extends StatefulWidget {
  const SelfieFrameScreen(
    this.event, {
    super.key,
    this.title = 'Selfie frame',
    this.export = const PhotoExport(),
    this.cameras,
    this.pickImage,
  });
  final EventDetails event;
  final String title;
  final PhotoExport export;

  /// Tests replace the device cameras and the gallery picker.
  final Future<List<CameraDescription>> Function()? cameras;
  final Future<String?> Function()? pickImage;

  @override
  State<SelfieFrameScreen> createState() => _SelfieFrameScreenState();
}

class _SelfieFrameScreenState extends State<SelfieFrameScreen>
    with WidgetsBindingObserver {
  final _frame = GlobalKey();
  final _framing = TransformationController();
  List<CameraDescription> _cameras = const [];
  int _camera = 0;
  CameraController? _controller;
  String? _photo, _cameraError;
  bool _busy = false;
  var _design = FrameDesign.spotlight;
  var _format = FrameFormat.post;
  var _caption = frameCaptions.first;

  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addObserver(this);
    _openCameras();
  }

  Future<void> _openCameras() async {
    try {
      final cameras = await (widget.cameras ?? availableCameras)();
      if (!mounted) return;
      final front = cameras.indexWhere(
        (camera) => camera.lensDirection == CameraLensDirection.front,
      );
      setState(() {
        _cameras = cameras;
        _camera = front < 0 ? 0 : front;
      });
    } catch (_) {
      // Handled below as a device without a camera.
    }
    if (_cameras.isEmpty) {
      if (mounted) {
        setState(
          () => _cameraError = 'No camera is available. Choose a photo from your gallery instead.',
        );
      }
      return;
    }
    await _start();
  }

  Future<void> _start() async {
    await _stop();
    if (!mounted || _photo != null || _cameras.isEmpty) return;
    setState(() => _cameraError = null);
    final controller = CameraController(
      _cameras[_camera],
      ResolutionPreset.veryHigh,
      enableAudio: false,
    );
    try {
      await controller.initialize();
      if (!mounted || _photo != null) {
        await controller.dispose();
        return;
      }
      setState(() => _controller = controller);
    } on CameraException catch (error) {
      await controller.dispose();
      if (mounted) {
        setState(() {
          _cameraError = error.code.startsWith('CameraAccess')
              ? 'Allow camera access in Settings, or choose a photo from your gallery.'
              : 'Could not start the camera. Try again or choose a photo.';
        });
      }
    }
  }

  Future<void> _stop() async {
    final controller = _controller;
    if (controller == null) return;
    if (mounted) setState(() => _controller = null);
    await controller.dispose();
  }

  @override
  void didChangeAppLifecycleState(AppLifecycleState state) {
    if (state == AppLifecycleState.resumed) {
      if (_controller == null && _photo == null) _start();
    } else if (state == AppLifecycleState.paused ||
        state == AppLifecycleState.inactive) {
      _stop();
    }
  }

  Future<void> _flip() async {
    if (_busy || _cameras.length < 2) return;
    AppFeedback.selection();
    final direction = _cameras[_camera].lensDirection;
    final next = _cameras.indexWhere((c) => c.lensDirection != direction);
    _camera = next < 0 ? (_camera + 1) % _cameras.length : next;
    await _start();
  }

  Future<void> _capture() async {
    final controller = _controller;
    if (_busy || controller == null || !controller.value.isInitialized) return;
    setState(() => _busy = true);
    try {
      final image = await controller.takePicture();
      AppFeedback.success();
      await _usePhoto(image.path);
    } catch (_) {
      if (mounted) _snack('Could not take the photo. Please try again.');
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  Future<void> _choose() async {
    if (_busy) return;
    setState(() => _busy = true);
    try {
      final path = await (widget.pickImage ?? _pickFromGallery)();
      if (path != null) await _usePhoto(path);
    } catch (_) {
      if (mounted) _snack('Could not open your photos. Please try again.');
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  static Future<String?> _pickFromGallery() async {
    final image = await ImagePicker().pickImage(
      source: ImageSource.gallery,
      maxWidth: 2400,
      maxHeight: 2400,
      imageQuality: 92,
    );
    return image?.path;
  }

  Future<void> _usePhoto(String path) async {
    if (!mounted) return;
    // Decode before showing, so the frame never exports a blank photo.
    await precacheImage(FileImage(File(path)), context);
    if (!mounted) return;
    _framing.value = Matrix4.identity();
    setState(() => _photo = path);
    await _stop();
  }

  Future<void> _retake() async {
    if (_busy) return;
    setState(() => _photo = null);
    if (_cameras.isNotEmpty) await _start();
  }

  Future<Uint8List> _render() async {
    await WidgetsBinding.instance.endOfFrame;
    final boundary =
        _frame.currentContext!.findRenderObject()! as RenderRepaintBoundary;
    final image = await boundary.toImage(
      pixelRatio: frameExportWidth / boundary.size.width,
    );
    try {
      final data = await image.toByteData(format: ui.ImageByteFormat.png);
      return data!.buffer.asUint8List();
    } finally {
      image.dispose();
    }
  }

  String get _fileName =>
      'bio-connect-${_format.name}-${DateTime.now().millisecondsSinceEpoch}.png';

  Future<void> _save() async {
    if (_busy) return;
    setState(() => _busy = true);
    try {
      await widget.export.save(await _render(), _fileName);
      AppFeedback.success();
      if (mounted) _snack('Saved to your gallery.');
    } on PhotoExportException catch (error) {
      if (mounted) _snack(error.message);
    } catch (_) {
      if (mounted) _snack('Could not save the photo. Please try again.');
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  Future<void> _share(BuildContext button, EventDetails event) async {
    if (_busy) return;
    final box = button.findRenderObject()! as RenderBox;
    final origin = box.localToGlobal(Offset.zero) & box.size;
    final copy = context.read<ContentProvider?>()?.content?.override;
    setState(() => _busy = true);
    try {
      await widget.export.share(
        await _render(),
        _fileName,
        origin: origin,
        text: copy?.call('frame.share_text') ?? shareText(_caption, event),
      );
    } catch (_) {
      if (mounted) _snack('Could not share the photo. Please try again.');
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  void _snack(String message) {
    ScaffoldMessenger.of(context)
      ..hideCurrentSnackBar()
      ..showSnackBar(SnackBar(content: Text(message)));
  }

  @override
  void dispose() {
    WidgetsBinding.instance.removeObserver(this);
    _controller?.dispose();
    _framing.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final event = widget.event;
    return PopScope<void>(
      canPop: !_busy,
      child: Scaffold(
        appBar: AppBar(
          title: Text(widget.title),
          automaticallyImplyLeading: !_busy,
        ),
        body: SafeArea(
          child: Column(
            children: [
              Expanded(
                child: Padding(
                  padding: const EdgeInsets.fromLTRB(20, 4, 20, 12),
                  child: Center(
                    child: AspectRatio(
                      aspectRatio: _format.size.aspectRatio,
                      child: DecoratedBox(
                        decoration: BoxDecoration(
                          borderRadius: BorderRadius.circular(6),
                          boxShadow: [
                            BoxShadow(
                              color: deepForest.withValues(alpha: .18),
                              blurRadius: 24,
                              offset: const Offset(0, 10),
                            ),
                          ],
                        ),
                        child: FittedBox(
                          child: RepaintBoundary(
                            key: _frame,
                            child: SelfieFrame(
                              design: _design,
                              format: _format,
                              caption: _caption,
                              event: event,
                              photo: _photoSlot(),
                            ),
                          ),
                        ),
                      ),
                    ),
                  ),
                ),
              ),
              _options(),
              Padding(
                padding: const EdgeInsets.fromLTRB(20, 12, 20, 16),
                child: _actions(event),
              ),
            ],
          ),
        ),
      ),
    );
  }

  Widget _photoSlot() {
    if (_photo case final photo?) {
      return InteractiveViewer(
        transformationController: _framing,
        minScale: 1,
        maxScale: 4,
        child: SizedBox.expand(
          child: Image.file(File(photo), fit: BoxFit.cover),
        ),
      );
    }
    final controller = _controller;
    if (controller != null && controller.value.isInitialized) {
      final size = controller.value.previewSize;
      return ClipRect(
        child: SizedBox.expand(
          child: FittedBox(
            fit: BoxFit.cover,
            child: SizedBox(
              // The preview size is reported in landscape.
              width: size?.height ?? 300,
              height: size?.width ?? 400,
              child: CameraPreview(controller),
            ),
          ),
        ),
      );
    }
    return ColoredBox(
      color: deepForest,
      child: Center(
        child: _cameraError == null
            ? const BioLoader(width: 48, onDark: true)
            : Padding(
                padding: const EdgeInsets.all(24),
                child: Column(
                  mainAxisSize: MainAxisSize.min,
                  children: [
                    const Icon(
                      Icons.no_photography_outlined,
                      color: lime,
                      size: 30,
                    ),
                    const SizedBox(height: 10),
                    Text(
                      _cameraError!,
                      textAlign: TextAlign.center,
                      style: const TextStyle(
                        color: Colors.white,
                        fontSize: 12,
                        height: 1.4,
                      ),
                    ),
                  ],
                ),
              ),
      ),
    );
  }

  Widget _options() => Column(
    children: [
      _ChipRow(
        children: [
          for (final caption in frameCaptions)
            ChoiceChip(
              label: Text(caption),
              selected: _caption == caption,
              onSelected: _busy
                  ? null
                  : (_) {
                      AppFeedback.selection();
                      setState(() => _caption = caption);
                    },
            ),
        ],
      ),
      const SizedBox(height: 8),
      _ChipRow(
        children: [
          for (final design in FrameDesign.values)
            ChoiceChip(
              label: Text(design.label),
              selected: _design == design,
              onSelected: _busy
                  ? null
                  : (_) {
                      AppFeedback.selection();
                      setState(() => _design = design);
                    },
            ),
          const SizedBox(
            height: 24,
            child: VerticalDivider(width: 12, color: fieldEdge),
          ),
          for (final format in FrameFormat.values)
            ChoiceChip(
              avatar: Icon(
                format == FrameFormat.post
                    ? Icons.crop_portrait_rounded
                    : Icons.smartphone_rounded,
                size: 17,
                color: _format == format ? lime : forest,
              ),
              showCheckmark: false,
              label: Text(format.label),
              selected: _format == format,
              onSelected: _busy
                  ? null
                  : (_) {
                      AppFeedback.selection();
                      _framing.value = Matrix4.identity();
                      setState(() => _format = format);
                    },
            ),
        ],
      ),
    ],
  );

  Widget _actions(EventDetails event) {
    if (_busy) {
      return const SizedBox(
        height: 52,
        child: Center(child: BioLoader(width: 52)),
      );
    }
    if (_photo != null) {
      return Row(
        children: [
          OutlinedButton.icon(
            onPressed: _retake,
            icon: const Icon(Icons.replay_rounded),
            label: const Text('Retake'),
          ),
          const SizedBox(width: 10),
          IconButton.outlined(
            tooltip: 'Save to gallery',
            onPressed: _save,
            style: IconButton.styleFrom(
              minimumSize: const Size(50, 50),
              foregroundColor: forest,
              side: BorderSide(color: forest.withValues(alpha: .28)),
            ),
            icon: const Icon(Icons.download_rounded),
          ),
          const SizedBox(width: 10),
          Expanded(
            child: Builder(
              builder: (button) => FilledButton.icon(
                onPressed: () => _share(button, event),
                icon: const Icon(Icons.ios_share_rounded),
                label: const Text('Share'),
              ),
            ),
          ),
        ],
      );
    }
    final ready = _controller?.value.isInitialized == true;
    final square = IconButton.styleFrom(
      minimumSize: const Size(52, 52),
      foregroundColor: forest,
      side: BorderSide(color: forest.withValues(alpha: .28)),
    );
    return Row(
      children: [
        IconButton.outlined(
          tooltip: 'Choose from gallery',
          onPressed: _choose,
          style: square,
          icon: const Icon(Icons.photo_library_outlined),
        ),
        const SizedBox(width: 10),
        Expanded(
          child: FilledButton.icon(
            onPressed: ready ? _capture : null,
            icon: const Icon(Icons.camera_alt_outlined),
            label: const Text('Take photo'),
          ),
        ),
        if (_cameras.length > 1) ...[
          const SizedBox(width: 10),
          IconButton.outlined(
            tooltip: 'Switch camera',
            onPressed: ready ? _flip : null,
            style: square,
            icon: const Icon(Icons.cameraswitch_outlined),
          ),
        ],
      ],
    );
  }
}

String shareText(String caption, EventDetails event) {
  final place = event.venue.isEmpty ? '' : ' at ${event.venue}';
  return '$caption ${event.title}, ${eventDateRange(event)}$place.';
}

class _ChipRow extends StatelessWidget {
  const _ChipRow({required this.children});
  final List<Widget> children;

  @override
  Widget build(BuildContext context) => SingleChildScrollView(
    scrollDirection: Axis.horizontal,
    padding: const EdgeInsets.symmetric(horizontal: 20),
    child: Row(
      children: [
        for (final (i, child) in children.indexed) ...[
          if (i > 0) const SizedBox(width: 8),
          child,
        ],
      ],
    ),
  );
}

/// The shareable picture: a photo inside one of the event frame designs, laid
/// out at the format's logical size.
class SelfieFrame extends StatelessWidget {
  const SelfieFrame({
    super.key,
    required this.design,
    required this.format,
    required this.caption,
    required this.event,
    required this.photo,
  });
  final FrameDesign design;
  final FrameFormat format;
  final String caption;
  final EventDetails event;
  final Widget photo;

  String get _details => [
    eventDateRange(event),
    if (event.venue.isNotEmpty) event.venue,
  ].join('  ·  ');

  @override
  Widget build(BuildContext context) => MediaQuery.withNoTextScaling(
    child: SizedBox.fromSize(
      size: format.size,
      child: DefaultTextStyle(
        style: const TextStyle(fontFamily: 'DM Sans', color: ink),
        child: switch (design) {
          FrameDesign.spotlight => _spotlight(),
          FrameDesign.arch => _arch(),
          FrameDesign.editorial => _editorial(),
        },
      ),
    ),
  );

  /// Full-bleed photo fading into forest behind the event details.
  Widget _spotlight() => ColoredBox(
    color: forest,
    child: Stack(
      fit: StackFit.expand,
      children: [
        photo,
        const IgnorePointer(
          child: DecoratedBox(
            decoration: BoxDecoration(
              gradient: LinearGradient(
                begin: Alignment.topCenter,
                end: Alignment.bottomCenter,
                colors: [Color(0x000B3329), Color(0xD90B3329), forest],
                stops: [.42, .74, 1],
              ),
            ),
          ),
        ),
        Positioned(
          left: 22,
          right: 22,
          bottom: 22,
          child: IgnorePointer(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Row(
                  children: [
                    Container(width: 20, height: 2, color: lime),
                    const SizedBox(width: 8),
                    _Caption(caption, color: lime),
                  ],
                ),
                const SizedBox(height: 6),
                _Title(event.title, color: Colors.white, size: 32),
                const SizedBox(height: 9),
                _Details(_details, color: Colors.white70),
              ],
            ),
          ),
        ),
        const Positioned(top: 14, left: 14, child: _LogoChip(height: 20)),
      ],
    ),
  );

  /// The photo in an arched window inside botanical line art.
  Widget _arch() => Stack(
    fit: StackFit.expand,
    children: [
      const _Art('assets/images/frame-forest.webp', color: forest),
      Padding(
        padding: EdgeInsets.fromLTRB(
          40,
          format == FrameFormat.story ? 56 : 30,
          40,
          22,
        ),
        child: Column(
          children: [
            Expanded(
              child: Container(
                padding: const EdgeInsets.all(5),
                decoration: BoxDecoration(
                  border: Border.all(color: gold, width: 1.4),
                  borderRadius: const BorderRadius.vertical(
                    top: Radius.circular(150),
                    bottom: Radius.circular(18),
                  ),
                ),
                child: ClipRRect(
                  borderRadius: const BorderRadius.vertical(
                    top: Radius.circular(145),
                    bottom: Radius.circular(13),
                  ),
                  child: photo,
                ),
              ),
            ),
            const SizedBox(height: 16),
            _Caption(caption, color: gold, center: true),
            const SizedBox(height: 4),
            _Title(event.title, color: Colors.white, size: 30, center: true),
            const SizedBox(height: 7),
            _Details(_details, color: Colors.white70, center: true),
            const SizedBox(height: 12),
            const _LogoChip(height: 17),
          ],
        ),
      ),
    ],
  );

  /// A light, magazine-style layout on cream paper.
  Widget _editorial() => Stack(
    fit: StackFit.expand,
    children: [
      const _Art('assets/images/frame-cream.webp', color: cream),
      Padding(
        padding: const EdgeInsets.fromLTRB(24, 22, 24, 22),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Image.asset(
              'assets/images/bio-connect-logo.png',
              height: 26,
              fit: BoxFit.contain,
            ),
            const SizedBox(height: 16),
            Expanded(
              child: Stack(
                fit: StackFit.expand,
                children: [
                  Positioned.fill(
                    left: 9,
                    top: 9,
                    child: DecoratedBox(
                      decoration: BoxDecoration(
                        color: lime,
                        borderRadius: BorderRadius.circular(22),
                      ),
                    ),
                  ),
                  Padding(
                    padding: const EdgeInsets.only(right: 9, bottom: 9),
                    child: ClipRRect(
                      borderRadius: BorderRadius.circular(22),
                      child: photo,
                    ),
                  ),
                ],
              ),
            ),
            const SizedBox(height: 18),
            // Right-aligned, clear of the artwork's lower-left corner.
            Align(
              alignment: Alignment.centerRight,
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.end,
                children: [
                  Row(
                    mainAxisSize: MainAxisSize.min,
                    children: [
                      _Caption(caption, color: forest),
                      const SizedBox(width: 8),
                      Container(
                        width: 7,
                        height: 7,
                        decoration: const BoxDecoration(
                          color: gold,
                          shape: BoxShape.circle,
                        ),
                      ),
                    ],
                  ),
                  const SizedBox(height: 5),
                  _Title(event.title, color: ink, size: 30, end: true),
                  const SizedBox(height: 7),
                  _Details(eventDateRange(event), color: muted, end: true),
                  if (event.venue.isNotEmpty)
                    _Details(event.venue, color: muted, end: true),
                ],
              ),
            ),
          ],
        ),
      ),
    ],
  );
}

TextAlign _align({required bool center, required bool end}) => center
    ? TextAlign.center
    : end
    ? TextAlign.end
    : TextAlign.start;

class _Art extends StatelessWidget {
  const _Art(this.asset, {required this.color});
  final String asset;
  final Color color;

  @override
  Widget build(BuildContext context) => ColoredBox(
    color: color,
    child: Image.asset(
      asset,
      fit: BoxFit.cover,
      errorBuilder: (_, _, _) => const SizedBox.shrink(),
    ),
  );
}

class _Caption extends StatelessWidget {
  const _Caption(this.text, {required this.color, this.center = false});
  final String text;
  final Color color;
  final bool center;

  @override
  Widget build(BuildContext context) => Text(
    text.toUpperCase(),
    textAlign: center ? TextAlign.center : TextAlign.start,
    style: TextStyle(
      color: color,
      fontSize: 11,
      fontWeight: FontWeight.w700,
      letterSpacing: 2,
    ),
  );
}

class _Title extends StatelessWidget {
  const _Title(
    this.text, {
    required this.color,
    required this.size,
    this.center = false,
    this.end = false,
  });
  final String text;
  final Color color;
  final double size;
  final bool center, end;

  @override
  Widget build(BuildContext context) => Text(
    text,
    maxLines: 1,
    softWrap: false,
    overflow: TextOverflow.fade,
    textAlign: _align(center: center, end: end),
    style: TextStyle(
      fontFamily: 'Manrope',
      fontWeight: FontWeight.w700,
      color: color,
      fontSize: size,
      height: 1.05,
      letterSpacing: -.4,
    ),
  );
}

class _Details extends StatelessWidget {
  const _Details(
    this.text, {
    required this.color,
    this.center = false,
    this.end = false,
  });
  final String text;
  final Color color;
  final bool center, end;

  @override
  Widget build(BuildContext context) => Text(
    text,
    maxLines: 1,
    overflow: TextOverflow.ellipsis,
    textAlign: _align(center: center, end: end),
    style: TextStyle(color: color, fontSize: 10.5, letterSpacing: .2),
  );
}

/// The event logo on white, so its colours read on any background.
class _LogoChip extends StatelessWidget {
  const _LogoChip({required this.height});
  final double height;

  @override
  Widget build(BuildContext context) => Container(
    padding: EdgeInsets.symmetric(
      horizontal: height * .5,
      vertical: height * .3,
    ),
    decoration: BoxDecoration(
      color: Colors.white.withValues(alpha: .94),
      borderRadius: BorderRadius.circular(height * .45),
    ),
    child: Image.asset(
      'assets/images/bio-connect-logo.png',
      height: height,
      fit: BoxFit.contain,
    ),
  );
}
