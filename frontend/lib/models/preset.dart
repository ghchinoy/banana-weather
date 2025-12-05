class Preset {
  final String id;
  final String name;
  final String? category;
  final String imageUrl;
  final String videoUrl;
  final DateTime? lastUpdated;

  Preset({
    required this.id,
    required this.name,
    this.category,
    required this.imageUrl,
    required this.videoUrl,
    this.lastUpdated,
  });

  factory Preset.fromJson(Map<String, dynamic> json) {
    return Preset(
      id: json['id'],
      name: json['name'],
      category: json['category'],
      imageUrl: json['image_url'],
      videoUrl: json['video_url'],
      lastUpdated: json['last_updated'] != null 
          ? DateTime.parse(json['last_updated']) 
          : null,
    );
  }
}
