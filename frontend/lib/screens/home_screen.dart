import 'dart:async'; // Added for Timer
import 'dart:convert';
import 'package:flutter/material.dart';
import 'package:provider/provider.dart';
import 'package:google_fonts/google_fonts.dart';
import 'package:video_player/video_player.dart';
import '../providers/weather_provider.dart';
import '../providers/theme_provider.dart';
import '../widgets/preset_drawer.dart';

class HomeScreen extends StatefulWidget {
  const HomeScreen({super.key});

  @override
  State<HomeScreen> createState() => _HomeScreenState();
}

class _HomeScreenState extends State<HomeScreen> {
  final TextEditingController _controller = TextEditingController();
  bool _isInitializing = true;
  VideoPlayerController? _videoController;
  String? _currentVideoUrl;
  
  // Status Cycling
  Timer? _statusTimer;
  String _currentPhrase = "this may take a minute...";
  final List<String> _loadingPhrases = [
    "this may take a minute...",
    "peeling back the layers of reality...",
    "summoning the banana spirits...",
    "rendering atmospheric vibes...",
    "teaching pixels to dance...",
    "consulting the cloud oracles...",
  ];

  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addPostFrameCallback((_) async {
      await Provider.of<WeatherProvider>(context, listen: false).fetchCurrentLocation();
      if (mounted) {
        setState(() {
          _isInitializing = false;
        });
      }
    });
  }

  void _startCycling() {
    if (_statusTimer != null && _statusTimer!.isActive) return;
    _statusTimer = Timer.periodic(const Duration(seconds: 4), (timer) {
      if (mounted) {
        setState(() {
          _currentPhrase = _loadingPhrases[(timer.tick) % _loadingPhrases.length];
        });
      }
    });
  }

  void _stopCycling() {
    _statusTimer?.cancel();
    _statusTimer = null;
  }

  @override
  void dispose() {
    _controller.dispose();
    _disposeVideo();
    _stopCycling();
    super.dispose();
  }

  Future<void> _initializeVideo(String url) async {
    // Hack: If URL is gs://, we can't play it.
    if (!url.startsWith('http')) return;

    // Dispose previous controller if any
    _disposeVideo();

    // Use .network (String) instead of .networkUrl (Uri) to avoid Web "Unimplemented" issues
    // ignore: deprecated_member_use
    final controller = VideoPlayerController.network(url);
    
    try {
      await controller.initialize();
      await controller.setVolume(0.0); // Mute to allow auto-play on web
      await controller.setLooping(true);
      await controller.play();
      if (mounted) {
        setState(() {
          _videoController = controller;
          _currentVideoUrl = url;
        });
      }
    } catch (e) {
      print("Video initialization failed: $e");
    }
  }

  void _disposeVideo() {
    _videoController?.dispose();
    _videoController = null;
    _currentVideoUrl = null;
  }

  @override
  Widget build(BuildContext context) {
    final weatherProvider = Provider.of<WeatherProvider>(context);
    final themeProvider = Provider.of<ThemeProvider>(context);
    final colorScheme = Theme.of(context).colorScheme;

    // Check for video update
    if (weatherProvider.videoUrl == null && _videoController != null) {
       _disposeVideo();
    }
    if (weatherProvider.videoUrl != null && weatherProvider.videoUrl != _currentVideoUrl) {
      _currentVideoUrl = weatherProvider.videoUrl;
      WidgetsBinding.instance.addPostFrameCallback((_) {
        _initializeVideo(weatherProvider.videoUrl!);
      });
    }

    return Scaffold(
      appBar: AppBar(
        centerTitle: true,
        title: Text(
          "Banana Weather 🍌🌤️",
          style: GoogleFonts.lato(
            fontWeight: FontWeight.bold,
            fontSize: 24,
          ),
        ),
        bottom: null,
        actions: [
          IconButton(
            icon: Icon(
              themeProvider.isDarkMode ? Icons.light_mode : Icons.dark_mode,
            ),
            onPressed: () {
              themeProvider.toggleTheme();
            },
          ),
        ],
      ),
      drawer: const PresetDrawer(),
      body: Center(
        child: AspectRatio(
          aspectRatio: 9 / 16,
          child: Container(
            decoration: BoxDecoration(
              color: Colors.black,
              borderRadius: BorderRadius.circular(16),
              boxShadow: [
                BoxShadow(
                  color: Colors.black.withOpacity(0.5),
                  blurRadius: 20,
                  spreadRadius: 5,
                ),
              ],
            ),
            clipBehavior: Clip.hardEdge,
            child: Stack(
              children: [
                // Image Layer
                if (weatherProvider.imageBase64 != null)
                  Positioned.fill(
                    child: Image.memory(
                      base64Decode(weatherProvider.imageBase64!),
                      fit: BoxFit.cover,
                    ),
                  )
                else if (weatherProvider.imageUrl != null)
                  Positioned.fill(
                    child: Image.network(
                      weatherProvider.imageUrl!,
                      fit: BoxFit.cover,
                    ),
                  )
                else
                  Positioned.fill(
                    child: Image.asset(
                      'assets/images/placeholder_vertical.png',
                      fit: BoxFit.cover,
                    ),
                  ),

                // Video Layer
                if (_videoController != null && _videoController!.value.isInitialized)
                  Positioned.fill(
                    child: FittedBox(
                      fit: BoxFit.cover,
                      child: SizedBox(
                        width: _videoController!.value.size.width,
                        height: _videoController!.value.size.height,
                        child: VideoPlayer(_videoController!),
                      ),
                    ),
                  ),

                // Unified Status Pill
                if (weatherProvider.isLoading || weatherProvider.statusMessage != null)
                  Positioned(
                    bottom: 30,
                    left: 0,
                    right: 0,
                    child: Center(
                      child: Container(
                        padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 8),
                        decoration: BoxDecoration(
                          color: Colors.black.withOpacity(0.6),
                          borderRadius: BorderRadius.circular(20),
                          border: Border.all(color: Colors.white10),
                        ),
                        child: Row(
                          mainAxisSize: MainAxisSize.min,
                          children: [
                            const SizedBox(
                              width: 16, 
                              height: 16, 
                              child: CircularProgressIndicator(
                                color: Colors.yellowAccent, 
                                strokeWidth: 2,
                              ),
                            ),
                            const SizedBox(width: 10),
                            Text(
                              (weatherProvider.statusMessage != null && weatherProvider.statusMessage!.contains("Animating"))
                                  ? "Animating (Veo 3.1)... $_currentPhrase"
                                  : (weatherProvider.statusMessage ?? "Loading..."),
                              style: GoogleFonts.lato(
                                color: Colors.white,
                                fontSize: 12,
                                fontWeight: FontWeight.w500,
                              ),
                            ),
                          ],
                        ),
                      ),
                    ),
                  ),

                // Regenerate Button
                if (!weatherProvider.isLoading && weatherProvider.city != null && !weatherProvider.isPresetLoaded)
                  Positioned(
                    bottom: 30,
                    right: 20,
                    child: FloatingActionButton.small(
                      onPressed: () {
                        weatherProvider.fetchWeather(city: weatherProvider.city!);
                      },
                      backgroundColor: Colors.white.withOpacity(0.2),
                      foregroundColor: Colors.white,
                      elevation: 0,
                      child: const Icon(Icons.refresh),
                    ),
                  ),

                // Error Message
                if (weatherProvider.error != null)
                  Positioned(
                    top: 10,
                    left: 20,
                    right: 20,
                    child: Container(
                      padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 8),
                      decoration: BoxDecoration(
                        color: colorScheme.errorContainer,
                        borderRadius: BorderRadius.circular(8),
                      ),
                      child: Row(
                        children: [
                          Expanded(
                            child: Text(
                              weatherProvider.error!,
                              style: TextStyle(color: colorScheme.onErrorContainer),
                              textAlign: TextAlign.center,
                            ),
                          ),
                          IconButton(
                            icon: Icon(Icons.close, color: colorScheme.onErrorContainer, size: 20),
                            onPressed: () {
                              weatherProvider.clearError();
                            },
                            padding: EdgeInsets.zero,
                            constraints: const BoxConstraints(),
                          ),
                        ],
                      ),
                    ),
                  ),
              ],
            ),
          ),
        ),
      ),
    );
  }
}
