import 'dart:convert';
import 'package:flutter/foundation.dart';
import 'package:flutter/material.dart';
import 'package:http/http.dart' as http;
import 'package:geolocator/geolocator.dart';
import '../models/preset.dart';

class WeatherProvider with ChangeNotifier {
  String? _city;
  String? _imageBase64;
  String? _imageUrl; // For presets
  bool _isLoading = false;
  String? _error;
  String? _statusMessage;
  String? _videoUrl;
  List<Preset> _presets = [];

  String? get city => _city;
  String? get imageBase64 => _imageBase64;
  String? get imageUrl => _imageUrl;
  bool get isLoading => _isLoading;
  String? get error => _error;
  String? get statusMessage => _statusMessage;
  String? get videoUrl => _videoUrl;
  List<Preset> get presets => _presets;

  void clearError() {
    _error = null;
    notifyListeners();
  }

  void loadPreset(Preset p) {
    _city = p.name;
    _imageUrl = p.imageUrl;
    _videoUrl = p.videoUrl;
    _imageBase64 = null; // Clear generated image
    _error = null;
    _statusMessage = null;
    notifyListeners();
  }

  Future<void> fetchPresets() async {
    try {
      final String baseUrl = kDebugMode ? 'http://localhost:8080' : '';
      final response = await http.get(Uri.parse('$baseUrl/api/presets'));
      if (response.statusCode == 200) {
        final List<dynamic> data = json.decode(response.body);
        _presets = data.map((json) => Preset.fromJson(json)).toList();
        notifyListeners();
      }
    } catch (e) {
      print("Failed to fetch presets: $e");
    }
  }

  Future<void> fetchCurrentLocation() async {
    try {
      bool serviceEnabled;
      LocationPermission permission;

      serviceEnabled = await Geolocator.isLocationServiceEnabled();
      if (!serviceEnabled) {
        await fetchWeather(city: 'San Francisco');
        return;
      }

      permission = await Geolocator.checkPermission();
      if (permission == LocationPermission.denied) {
        permission = await Geolocator.requestPermission();
        if (permission == LocationPermission.denied) {
          await fetchWeather(city: 'San Francisco');
          return;
        }
      }
      
      if (permission == LocationPermission.deniedForever) {
        await fetchWeather(city: 'San Francisco');
        return;
      } 

      Position position = await Geolocator.getCurrentPosition();
      await fetchWeather(lat: position.latitude, lng: position.longitude);
    } catch (e) {
       await fetchWeather(city: 'San Francisco');
    }
  }

  Future<void> fetchWeather({String? city, double? lat, double? lng}) async {
    _isLoading = true;
    _error = null;
    _statusMessage = "Connecting...";
    _videoUrl = null;
    _imageUrl = null; // Clear preset image
    _imageBase64 = null;
    notifyListeners();

    try {
      final String baseUrl = kDebugMode ? 'http://localhost:8080' : '';

      final Uri uri;
      if (lat != null && lng != null) {
        uri = Uri.parse('$baseUrl/api/weather?lat=$lat&lng=$lng');
      } else if (city != null && city.isNotEmpty) {
        uri = Uri.parse('$baseUrl/api/weather?city=$city');
      } else {
         uri = Uri.parse('$baseUrl/api/weather?city=San Francisco');
      }
      
      final request = http.Request('GET', uri);
      request.headers['Accept'] = 'text/event-stream';

      final client = http.Client();
      final response = await client.send(request);

      if (response.statusCode != 200) {
        _error = 'Failed to connect: ${response.statusCode}';
        _isLoading = false;
        notifyListeners();
        return;
      }

      String currentEvent = '';

      response.stream
          .transform(const Utf8Decoder())
          .transform(const LineSplitter())
          .listen(
        (line) {
          if (line.startsWith('event:')) {
            currentEvent = line.substring(6).trim();
          } else if (line.startsWith('data:')) {
            final data = line.substring(5).trim();
            _handleEvent(currentEvent, data);
          }
        },
        onError: (e) {
          _error = 'Stream error: $e';
          _isLoading = false;
          notifyListeners();
        },
        onDone: () {
          // If we finished without a result, that might be an issue, 
          // but usually 'result' event handles the completion.
        },
      );

    } catch (e) {
      _error = 'Error: $e';
      _isLoading = false;
      notifyListeners();
    }
  }

  void _handleEvent(String event, String data) {
    switch (event) {
      case 'status':
        _statusMessage = data;
        notifyListeners();
        break;
      case 'error':
        _error = data;
        _isLoading = false;
        _statusMessage = null;
        notifyListeners();
        break;
      case 'result':
        try {
          final jsonData = json.decode(data);
          _city = jsonData['city'];
          _imageBase64 = jsonData['image_base64']; // Null if missing
          _imageUrl = jsonData['image_url'];       // Null if missing
          _isLoading = false;
          _statusMessage = null;
          notifyListeners();
        } catch (e) {
          _error = "Failed to parse result";
          _isLoading = false;
          notifyListeners();
        }
        break;
      case 'video':
        _videoUrl = data;
        // If we receive a video, we are effectively "done" with the heavy lifting for this session
        _statusMessage = null; 
        notifyListeners();
        break;
    }
  }
}
