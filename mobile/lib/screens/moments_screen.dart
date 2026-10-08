import 'dart:async';
import 'dart:io';

import 'package:cached_network_image/cached_network_image.dart';
import 'package:camera/camera.dart';
import 'package:flutter/foundation.dart';
import 'package:flutter/material.dart';
import 'package:photo_view/photo_view.dart';
import 'package:photo_view/photo_view_gallery.dart';
import 'package:provider/provider.dart';

import '../main.dart';
import '../providers/pass_wallet.dart';
import '../services/moments_photo_actions.dart';
import '../services/moments_service.dart';
import '../services/pass_service.dart';
import '../widgets/directory.dart';
import '../widgets/interaction.dart';
import '../widgets/motion.dart';
import 'my_passes_screen.dart';

class MomentsScreen extends StatefulWidget {
  const MomentsScreen({super.key, this.wallet, this.service});
  final PassWallet? wallet;
  final MomentsService? service;

  @override
  State<MomentsScreen> createState() => _MomentsScreenState();
}

class _MomentsScreenState extends State<MomentsScreen>
    with WidgetsBindingObserver {
  late final PassWallet _wallet;
  PassWallet? _ownWallet;
  late final MomentsService _service = widget.service ?? MomentsService();
  MomentsCredential? _credential;
  MomentsStatus? _status;
  String? _error;
  bool _loading = true, _working = false;
  bool _refreshing = false;
  bool _previewMode = false;
  Timer? _poll;

  @override
  void initState() {
    super.initState();
    _wallet =
        widget.wallet ??
        context.read<PassWallet?>() ??
        (_ownWallet = PassWallet());
    WidgetsBinding.instance.addObserver(this);
    _load();
  }

  Future<void> _load({bool reloadWallet = false}) async {
    if (_refreshing) return;
    _refreshing = true;
    _poll?.cancel();
    setState(() {
      _loading = true;
      _error = null;
    });
    try {
      if (!_wallet.loaded || reloadWallet) {
        if (reloadWallet && _wallet.loaded) {
          await _wallet.refresh();
        } else {
          await _wallet.load();
        }
      }
      final credentials = _wallet.momentsCredentials;
      if (credentials.isEmpty) {
        if (kDebugMode) {
          _previewMode = true;
          _credential = const MomentsCredential(
            token: 'debug-preview',
            pass: AdmissionPass(
              id: 'debug-preview',
              name: 'Preview attendee',
              institution: 'Bio Connect',
              designation: 'Delegate',
              category: 'Preview',
              number: 'SAMPLE',
              qrId: '',
              downloadUrl: '',
            ),
          );
          _status = const MomentsStatus(
            albumReady: true,
            selfieUploaded: false,
            similarPhotosCount: 0,
            photosReady: false,
            processingStatus: 'pending',
          );
          return;
        }
        _credential = null;
        _status = null;
        return;
      }
      _previewMode = false;
      final selectedID = _credential?.pass.id;
      _credential = credentials.cast<MomentsCredential?>().firstWhere(
        (item) => item?.pass.id == selectedID,
        orElse: () => credentials.first,
      );
      _status = await _service.status(_credential!);
    } on MomentsException catch (error) {
      _error = error.message;
    } catch (_) {
      _error = 'Could not load Moments. Check your connection and try again.';
    } finally {
      _refreshing = false;
      if (mounted) {
        setState(() => _loading = false);
        _schedulePoll();
      }
    }
  }

  void _schedulePoll() {
    _poll?.cancel();
    if (_status?.isProcessing == true &&
        WidgetsBinding.instance.lifecycleState == AppLifecycleState.resumed) {
      _poll = Timer(const Duration(seconds: 10), () {
        if (mounted && ModalRoute.of(context)?.isCurrent == true) _load();
      });
    }
  }

  @override
  void didChangeAppLifecycleState(AppLifecycleState state) {
    if (state == AppLifecycleState.resumed) {
      _load(reloadWallet: true);
    } else {
      _poll?.cancel();
    }
  }

  Future<void> _openPasses() async {
    await Navigator.of(context)
        .push(MaterialPageRoute<void>(builder: (_) => const MyPassesScreen()));
    if (mounted) await _load(reloadWallet: true);
  }

  Future<void> _select(MomentsCredential? credential) async {
    if (credential == null || credential.pass.id == _credential?.pass.id) {
      return;
    }
    setState(() => _credential = credential);
    await _load();
  }

  Future<void> _takeSelfie() async {
    final credential = _credential;
    if (_working || credential == null) return;
    final consent = await showDialog<bool>(
      context: context,
      builder: (context) => AlertDialog(
        title: const Text('Find your event photos?'),
        content: const Text(
          'Your selfie will be sent securely to the event photo service only to match photos of you. You can remove it from Moments at any time.',
        ),
        actions: [
          TextButton(
            onPressed: () => Navigator.pop(context, false),
            child: const Text('Not now'),
          ),
          FilledButton(
            onPressed: () => Navigator.pop(context, true),
            child: const Text('Continue'),
          ),
        ],
      ),
    );
    if (consent != true || !mounted) return;
    setState(() => _working = true);
    try {
      final cameras = await availableCameras();
      if (cameras.isEmpty) {
        throw const MomentsException('Camera unavailable on this device.');
      }
      final camera = cameras.firstWhere(
        (item) => item.lensDirection == CameraLensDirection.front,
        orElse: () => cameras.first,
      );
      if (!mounted) return;
      final uploaded = await Navigator.of(context).push<bool>(
        MaterialPageRoute(
          builder: (_) => MomentsCameraScreen(
            camera: camera,
            credential: credential,
            service: _service,
          ),
        ),
      );
      if (uploaded == true && mounted) {
        _snack('Selfie uploaded. Finding your photos.');
        await _load();
      }
    } on MomentsException catch (error) {
      if (mounted) _snack(error.message);
    } catch (_) {
      if (mounted) _snack('Could not open the camera. Please try again.');
    } finally {
      if (mounted) setState(() => _working = false);
    }
  }

  Future<void> _viewPhotos() async {
    final credential = _credential;
    if (_working || credential == null) return;
    setState(() => _working = true);
    try {
      final album = await _service.photos(credential);
      if (!mounted) return;
      if (album.photos.isEmpty) {
        _snack('No matching photos are available yet.');
      } else {
        await Navigator.of(context).push(
          MaterialPageRoute<void>(
            builder: (_) => MomentsAlbumScreen(photos: album.photos),
          ),
        );
      }
    } on MomentsException catch (error) {
      if (mounted) _snack(error.message);
    } finally {
      if (mounted) setState(() => _working = false);
    }
  }

  Future<void> _removeSelfie() async {
    final credential = _credential;
    if (_working || credential == null) return;
    final remove = await showDialog<bool>(
      context: context,
      builder: (context) => AlertDialog(
        title: const Text('Remove your selfie?'),
        content: const Text(
          'Your matched album will no longer be available. Event photographs themselves are not deleted.',
        ),
        actions: [
          TextButton(
            onPressed: () => Navigator.pop(context, false),
            child: const Text('Keep selfie'),
          ),
          FilledButton(
            onPressed: () => Navigator.pop(context, true),
            child: const Text('Remove'),
          ),
        ],
      ),
    );
    if (remove != true || !mounted) return;
    setState(() => _working = true);
    try {
      await _service.removeSelfie(credential);
      if (mounted) {
        AppFeedback.success();
        _snack('Selfie removed. You can add one again later.');
        await _load();
      }
    } on MomentsException catch (error) {
      if (mounted) _snack(error.message);
    } finally {
      if (mounted) setState(() => _working = false);
    }
  }

  void _snack(String message) {
    ScaffoldMessenger.of(context)
      ..hideCurrentSnackBar()
      ..showSnackBar(SnackBar(content: Text(message)));
  }

  @override
  void dispose() {
    _poll?.cancel();
    WidgetsBinding.instance.removeObserver(this);
    _ownWallet?.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) => Scaffold(
    appBar: AppBar(title: const Text('Moments album')),
    body: SafeArea(child: _body()),
  );

  Widget _body() {
    if (_loading) {
      return const Center(child: BioLoader(label: 'Loading Moments'));
    }
    if (_credential == null) {
      return ListView(
        padding: const EdgeInsets.all(24),
        children: [
          const PageIntro(
            eyebrow: 'YOUR EVENT PHOTOS',
            title: 'Find your\nMoments.',
            lede: 'Verify and save your Bio Connect pass first. Your pass securely links the album to you.',
          ),
          const SizedBox(height: 28),
          _MomentsPanel(
            icon: Icons.confirmation_number_outlined,
            title: 'Add your pass to continue',
            message:
                'Use the same email or mobile number used for registration.',
            action: FilledButton.icon(
              onPressed: _openPasses,
              icon: const Icon(Icons.add),
              label: const Text('Add my pass'),
            ),
          ),
        ],
      );
    }
    if (_error != null) {
      return Center(
        child: StateMessage(_error!, action: 'Try again', onTap: _load),
      );
    }
    final status = _status!;
    final credentials = _wallet.momentsCredentials;
    final (icon, title, message) = switch (status) {
      MomentsStatus(selfieUploaded: false) => (
        Icons.face_retouching_natural_outlined,
        'Take a selfie',
        'Keep one face clearly visible. We will use it to find your event photos.',
      ),
      MomentsStatus(processingFailed: true) => (
        Icons.error_outline,
        'Selfie could not be processed',
        'Retake it in good lighting with one face clearly visible.',
      ),
      MomentsStatus(isProcessing: true) => (
        Icons.auto_awesome_outlined,
        'Finding your photos',
        'Your selfie is being processed. This page updates automatically.',
      ),
      MomentsStatus(albumReady: false) => (
        Icons.hourglass_top_rounded,
        'Waiting for event photos',
        'Your selfie is saved. Photos will appear when the organizer adds them.',
      ),
      MomentsStatus(photosReady: true) => (
        Icons.photo_library_outlined,
        'Your photos are ready',
        '${status.similarPhotosCount} matching ${status.similarPhotosCount == 1 ? 'photo' : 'photos'} from Bio Connect.',
      ),
      _ => (
        Icons.image_search_outlined,
        'No matching photos yet',
        'Check again as more event photos are added.',
      ),
    };
    return RefreshIndicator(
      onRefresh: _load,
      child: ListView(
        physics: const AlwaysScrollableScrollPhysics(),
        padding: const EdgeInsets.all(24),
        children: [
          const PageIntro(
            eyebrow: 'YOUR EVENT PHOTOS',
            title: 'Find your\nMoments.',
            lede:
                'A private album matched from the official event photographs.',
          ),
          const SizedBox(height: 24),
          if (_previewMode) ...[
            const Align(
              alignment: Alignment.centerLeft,
              child: Chip(
                avatar: Icon(Icons.visibility_outlined, size: 17),
                label: Text('DEBUG PREVIEW'),
              ),
            ),
            const SizedBox(height: 12),
          ],
          if (credentials.length > 1) ...[
            DropdownButtonFormField<MomentsCredential>(
              initialValue: _credential,
              decoration: const InputDecoration(labelText: 'Pass holder'),
              items: [
                for (final item in credentials)
                  DropdownMenuItem(
                    value: item,
                    child: Text('${item.pass.name} · ${item.pass.number}'),
                  ),
              ],
              onChanged: _working ? null : _select,
            ),
            const SizedBox(height: 16),
          ],
          _MomentsPanel(
            icon: icon,
            title: title,
            message: message,
            imageUrl: status.uploadedPhotoUrl,
            busy: _working || status.isProcessing,
            action: Column(
              crossAxisAlignment: CrossAxisAlignment.stretch,
              children: [
                if (status.photosReady && !status.isProcessing)
                  FilledButton.icon(
                    onPressed: _working ? null : _viewPhotos,
                    icon: const Icon(Icons.photo_library_outlined),
                    label: const Text('View my photos'),
                  ),
                if (status.photosReady && !status.isProcessing)
                  const SizedBox(height: 10),
                OutlinedButton.icon(
                  onPressed: _working ? null : _takeSelfie,
                  icon: const Icon(Icons.camera_alt_outlined),
                  label: Text(
                    status.selfieUploaded ? 'Retake selfie' : 'Take a selfie',
                  ),
                ),
                if (status.selfieUploaded) ...[
                  TextButton.icon(
                    onPressed: _working ? null : _load,
                    icon: const Icon(Icons.refresh),
                    label: const Text('Check again'),
                  ),
                  TextButton.icon(
                    onPressed: _working ? null : _removeSelfie,
                    icon: const Icon(Icons.delete_outline),
                    label: const Text('Remove selfie'),
                  ),
                ],
              ],
            ),
          ),
          const SizedBox(height: 18),
          const Text(
            'Privacy: your selfie is used only to match event photos. You can remove it here at any time.',
            style: TextStyle(color: muted, height: 1.5, fontSize: 12),
          ),
        ],
      ),
    );
  }
}

class _MomentsPanel extends StatelessWidget {
  const _MomentsPanel({
    required this.icon,
    required this.title,
    required this.message,
    required this.action,
    this.imageUrl = '',
    this.busy = false,
  });
  final IconData icon;
  final String title, message, imageUrl;
  final Widget action;
  final bool busy;

  @override
  Widget build(BuildContext context) => Container(
    padding: const EdgeInsets.all(22),
    decoration: BoxDecoration(
      color: Colors.white,
      borderRadius: BorderRadius.circular(22),
      border: Border.all(color: forest.withValues(alpha: .1)),
    ),
    child: Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        if (imageUrl.isNotEmpty) ...[
          ClipRRect(
            borderRadius: BorderRadius.circular(16),
            child: CachedNetworkImage(
              imageUrl: imageUrl,
              height: 220,
              fit: BoxFit.contain,
              errorWidget: (_, _, _) => Icon(icon, color: forest, size: 54),
            ),
          ),
          const SizedBox(height: 20),
        ] else ...[
          ClipRRect(
            borderRadius: BorderRadius.circular(16),
            child: Image.asset(
              'assets/images/moments-hero.png',
              height: 205,
              fit: BoxFit.cover,
            ),
          ),
          const SizedBox(height: 18),
          Icon(icon, color: forest, size: 42),
        ],
        const SizedBox(height: 14),
        Text(
          title,
          textAlign: TextAlign.center,
          style: const TextStyle(fontFamily: 'Manrope', fontSize: 22),
        ),
        const SizedBox(height: 8),
        Text(
          message,
          textAlign: TextAlign.center,
          style: const TextStyle(color: muted, height: 1.5),
        ),
        if (busy) ...[
          const SizedBox(height: 18),
          const Center(child: BioLoader(width: 54)),
        ],
        const SizedBox(height: 20),
        action,
      ],
    ),
  );
}

class MomentsCameraScreen extends StatefulWidget {
  const MomentsCameraScreen({
    super.key,
    required this.camera,
    required this.credential,
    required this.service,
  });
  final CameraDescription camera;
  final MomentsCredential credential;
  final MomentsService service;

  @override
  State<MomentsCameraScreen> createState() => _MomentsCameraScreenState();
}

class _MomentsCameraScreenState extends State<MomentsCameraScreen>
    with WidgetsBindingObserver {
  CameraController? _controller;
  String? _photo, _error;
  bool _busy = false;

  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addObserver(this);
    _start();
  }

  Future<void> _start() async {
    final previous = _controller;
    _controller = null;
    await previous?.dispose();
    if (!mounted || _photo != null) return;
    setState(() => _error = null);
    final controller = CameraController(
      widget.camera,
      ResolutionPreset.high,
      enableAudio: false,
    );
    try {
      await controller.initialize();
      if (!mounted) {
        await controller.dispose();
        return;
      }
      setState(() => _controller = controller);
    } on CameraException catch (error) {
      await controller.dispose();
      if (mounted) {
        setState(() {
          _error = error.code.startsWith('CameraAccess')
              ? 'Camera permission is required. Allow it in device settings, then try again.'
              : 'Could not start the camera. Please try again.';
        });
      }
    }
  }

  @override
  void didChangeAppLifecycleState(AppLifecycleState state) {
    if (state == AppLifecycleState.resumed) {
      _start();
    } else {
      final controller = _controller;
      _controller = null;
      controller?.dispose();
    }
  }

  Future<void> _capture() async {
    final controller = _controller;
    if (_busy || controller == null || !controller.value.isInitialized) return;
    setState(() => _busy = true);
    try {
      final image = await controller.takePicture();
      await controller.dispose();
      if (mounted) setState(() => _photo = image.path);
    } catch (_) {
      if (mounted) {
        setState(() => _error = 'Could not take the selfie. Try again.');
      }
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  Future<void> _upload() async {
    if (_busy || _photo == null) return;
    setState(() => _busy = true);
    try {
      await widget.service.uploadSelfie(widget.credential, _photo!);
      if (mounted) Navigator.pop(context, true);
    } on MomentsException catch (error) {
      if (mounted) {
        ScaffoldMessenger.of(context)
            .showSnackBar(SnackBar(content: Text(error.message)));
      }
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  Future<void> _retake() async {
    if (_busy) return;
    setState(() => _photo = null);
    await _start();
  }

  @override
  void dispose() {
    WidgetsBinding.instance.removeObserver(this);
    _controller?.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) => PopScope<void>(
    canPop: !_busy,
    child: Scaffold(
      appBar: AppBar(
        title: Text(_photo == null ? 'Take a selfie' : 'Confirm your selfie'),
        automaticallyImplyLeading: !_busy,
      ),
      body: SafeArea(
        child: Padding(
          padding: const EdgeInsets.all(20),
          child: Column(
            children: [
              Expanded(child: _preview()),
              const SizedBox(height: 18),
              const Text(
                'Keep one face clearly visible in good lighting.',
                textAlign: TextAlign.center,
                style: TextStyle(color: muted),
              ),
              const SizedBox(height: 16),
              if (_busy)
                const BioLoader(width: 52, label: 'Please wait')
              else if (_photo == null)
                FilledButton.icon(
                  onPressed: _controller?.value.isInitialized == true
                      ? _capture
                      : null,
                  icon: const Icon(Icons.camera_alt_outlined),
                  label: const Text('Take selfie'),
                )
              else
                Row(
                  children: [
                    Expanded(
                      child: OutlinedButton(
                        onPressed: _retake,
                        child: const Text('Retake'),
                      ),
                    ),
                    const SizedBox(width: 12),
                    Expanded(
                      child: FilledButton(
                        onPressed: _upload,
                        child: const Text('Use selfie'),
                      ),
                    ),
                  ],
                ),
            ],
          ),
        ),
      ),
    ),
  );

  Widget _preview() {
    if (_error != null) {
      return StateMessage(_error!, action: 'Try again', onTap: _start);
    }
    if (_photo != null) {
      return ClipRRect(
        borderRadius: BorderRadius.circular(22),
        child: Image.file(File(_photo!), fit: BoxFit.contain),
      );
    }
    final controller = _controller;
    if (controller == null || !controller.value.isInitialized) {
      return const Center(child: BioLoader(label: 'Starting camera'));
    }
    return Center(
      child: ClipRRect(
        borderRadius: BorderRadius.circular(22),
        child: CameraPreview(controller),
      ),
    );
  }
}

class MomentsAlbumScreen extends StatelessWidget {
  const MomentsAlbumScreen({super.key, required this.photos});
  final List<MomentPhoto> photos;

  @override
  Widget build(BuildContext context) => Scaffold(
    appBar: AppBar(
      title: Text(
        '${photos.length} ${photos.length == 1 ? 'photo' : 'photos'}',
      ),
    ),
    body: LayoutBuilder(
      builder: (context, constraints) {
        final columns = constraints.maxWidth >= 720
            ? 4
            : constraints.maxWidth >= 420
            ? 3
            : 2;
        return GridView.builder(
          padding: const EdgeInsets.all(12),
          gridDelegate: SliverGridDelegateWithFixedCrossAxisCount(
            crossAxisCount: columns,
            crossAxisSpacing: 8,
            mainAxisSpacing: 8,
          ),
          itemCount: photos.length,
          itemBuilder: (context, index) {
            final photo = photos[index];
            return Semantics(
              button: true,
              label: 'View moment ${index + 1}',
              child: InkWell(
                onTap: () => Navigator.of(context).push(
                  MaterialPageRoute<void>(
                    builder: (_) =>
                        MomentsPhotoViewer(photos: photos, initialIndex: index),
                  ),
                ),
                child: ClipRRect(
                  borderRadius: BorderRadius.circular(14),
                  child: CachedNetworkImage(
                    imageUrl: photo.thumbnailUrl.isNotEmpty
                        ? photo.thumbnailUrl
                        : photo.fileUrl,
                    fit: BoxFit.cover,
                    placeholder: (_, _) =>
                        const Center(child: BioLoader(width: 42)),
                    errorWidget: (_, _, _) => const ColoredBox(
                      color: cream,
                      child: Icon(Icons.broken_image_outlined),
                    ),
                  ),
                ),
              ),
            );
          },
        );
      },
    ),
  );
}

class MomentsPhotoViewer extends StatefulWidget {
  const MomentsPhotoViewer({
    super.key,
    required this.photos,
    required this.initialIndex,
    this.actions,
  });
  final List<MomentPhoto> photos;
  final int initialIndex;
  final MomentsPhotoActions? actions;

  @override
  State<MomentsPhotoViewer> createState() => _MomentsPhotoViewerState();
}

class _MomentsPhotoViewerState extends State<MomentsPhotoViewer> {
  late final PageController _controller = PageController(
    initialPage: widget.initialIndex,
  );
  late final MomentsPhotoActions _actions =
      widget.actions ?? MomentsPhotoActions();
  late int _index = widget.initialIndex;
  bool _busy = false;

  Future<void> _save() async {
    if (_busy) return;
    setState(() => _busy = true);
    try {
      await _actions.save(widget.photos[_index].fileUrl);
      if (mounted) _snack('Photo saved to your gallery.');
    } on MomentsException catch (error) {
      if (mounted) _snack(error.message);
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  Future<void> _share(BuildContext buttonContext) async {
    if (_busy) return;
    final box = buttonContext.findRenderObject() as RenderBox;
    final origin = box.localToGlobal(Offset.zero) & box.size;
    setState(() => _busy = true);
    try {
      await _actions.share(widget.photos[_index].fileUrl, origin: origin);
    } on MomentsException catch (error) {
      if (mounted) _snack(error.message);
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  void _snack(String message) =>
      ScaffoldMessenger.of(context)
          .showSnackBar(SnackBar(content: Text(message)));

  @override
  void dispose() {
    _controller.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) => Scaffold(
    backgroundColor: Colors.black,
    body: Stack(
      children: [
        PhotoViewGallery.builder(
          itemCount: widget.photos.length,
          pageController: _controller,
          onPageChanged: (value) => setState(() => _index = value),
          builder: (_, index) => PhotoViewGalleryPageOptions(
            imageProvider: CachedNetworkImageProvider(
              widget.photos[index].fileUrl,
            ),
            initialScale: PhotoViewComputedScale.contained,
            minScale: PhotoViewComputedScale.contained,
            maxScale: PhotoViewComputedScale.covered * 4,
          ),
          loadingBuilder: (_, _) => const Center(
            child: BioLoader(label: 'Loading photo', onDark: true),
          ),
          backgroundDecoration: const BoxDecoration(color: Colors.black),
        ),
        SafeArea(
          child: Padding(
            padding: const EdgeInsets.all(12),
            child: Row(
              mainAxisAlignment: MainAxisAlignment.spaceBetween,
              children: [
                IconButton.filled(
                  tooltip: 'Back',
                  onPressed: () => Navigator.pop(context),
                  icon: const Icon(Icons.arrow_back),
                ),
                Text(
                  '${_index + 1} / ${widget.photos.length}',
                  style: const TextStyle(color: Colors.white),
                ),
              ],
            ),
          ),
        ),
        Align(
          alignment: Alignment.bottomRight,
          child: SafeArea(
            child: Padding(
              padding: const EdgeInsets.all(18),
              child: Row(
                mainAxisSize: MainAxisSize.min,
                children: [
                  if (_busy)
                    const BioLoader(width: 48, onDark: true)
                  else ...[
                    IconButton.filled(
                      tooltip: 'Save photo',
                      onPressed: _save,
                      icon: const Icon(Icons.download),
                    ),
                    const SizedBox(width: 12),
                    Builder(
                      builder: (buttonContext) => IconButton.filled(
                        tooltip: 'Share photo',
                        onPressed: () => _share(buttonContext),
                        icon: const Icon(Icons.share),
                      ),
                    ),
                  ],
                ],
              ),
            ),
          ),
        ),
      ],
    ),
  );
}
