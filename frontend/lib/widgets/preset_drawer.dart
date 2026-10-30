import 'package:flutter/material.dart';
import 'package:provider/provider.dart';
import 'package:google_fonts/google_fonts.dart';
import '../providers/weather_provider.dart';

class PresetDrawer extends StatefulWidget {
  const PresetDrawer({super.key});

  @override
  State<PresetDrawer> createState() => _PresetDrawerState();
}

class _PresetDrawerState extends State<PresetDrawer> {
  @override
  void initState() {
    super.initState();
    // Fetch presets when drawer opens
    WidgetsBinding.instance.addPostFrameCallback((_) {
      Provider.of<WeatherProvider>(context, listen: false).fetchPresets();
    });
  }

  @override
  Widget build(BuildContext context) {
    final weatherProvider = Provider.of<WeatherProvider>(context);

    // Group presets by category
    final Map<String, List<dynamic>> grouped = {};
    for (var p in weatherProvider.presets) {
      final cat = (p.category != null && p.category!.isNotEmpty) ? p.category! : "General";
      if (!grouped.containsKey(cat)) {
        grouped[cat] = [];
      }
      grouped[cat]!.add(p);
    }

    // Sort categories (Optional: Custom order or Alphabetical)
    final sortedKeys = grouped.keys.toList()..sort();

    return Drawer(
      backgroundColor: Colors.black.withValues(alpha: 0.9),
      child: Column(
        children: [
          DrawerHeader(
            decoration: BoxDecoration(
              color: Colors.black,
              border: Border(bottom: BorderSide(color: Colors.white.withValues(alpha: 0.1))),
            ),
            child: Center(
              child: Text(
                "Destinations",
                style: GoogleFonts.lato(
                  color: Colors.white,
                  fontSize: 24,
                  fontWeight: FontWeight.bold,
                ),
              ),
            ),
          ),
          ListTile(
            leading: const Icon(Icons.my_location, color: Colors.white),
            title: Text(
              "Current Location", 
              style: GoogleFonts.lato(color: Colors.white, fontWeight: FontWeight.bold)
            ),
            onTap: () {
              Navigator.pop(context);
              Provider.of<WeatherProvider>(context, listen: false).fetchCurrentLocation();
            },
          ),
          Divider(color: Colors.white.withValues(alpha: 0.1), height: 1),
          Expanded(
            child: ListView.builder(
              itemCount: sortedKeys.length,
              itemBuilder: (context, index) {
                final category = sortedKeys[index];
                final presets = grouped[category]!;
                
                return ExpansionTile(
                  title: Text(
                    category,
                    style: GoogleFonts.lato(
                      color: Colors.white,
                      fontWeight: FontWeight.bold,
                      fontSize: 16,
                    ),
                  ),
                  collapsedIconColor: Colors.white70,
                  iconColor: Colors.yellowAccent,
                  children: presets.map<Widget>((preset) {
                    return ListTile(
                      leading: ClipRRect(
                        borderRadius: BorderRadius.circular(4),
                        child: Image.network(
                          preset.imageUrl,
                          width: 40,
                          height: 40,
                          fit: BoxFit.cover,
                          errorBuilder: (c, e, s) => const Icon(Icons.image, color: Colors.white54),
                        ),
                      ),
                      title: Text(
                        preset.name,
                        style: GoogleFonts.lato(color: Colors.white70),
                      ),
                      onTap: () {
                        weatherProvider.loadPreset(preset);
                        Navigator.pop(context); // Close drawer
                      },
                    );
                  }).toList(),
                );
              },
            ),
          ),
        ],
      ),
    );
  }
}
