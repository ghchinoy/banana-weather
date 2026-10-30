import 'dart:js_interop';
import 'package:flutter/foundation.dart';
import 'package:http/http.dart' as http;
import 'package:web/web.dart' as web;

/// Downloads a file from [url] and saves it as [filename].
/// Uses Blob to support custom filenames and handle CORS correctly.
Future<void> downloadFile(String url, String filename) async {
  try {
    // 1. Fetch the data as bytes
    final response = await http.get(Uri.parse(url));

    if (response.statusCode == 200) {
      // 2. Create a Blob from the bytes
      final blob = web.Blob([response.bodyBytes.toJS].toJS);

      // 3. Create an Object URL
      final objectUrl = web.URL.createObjectURL(blob);

      // 4. Create a hidden anchor tag
      final anchor = web.document.createElement('a') as web.HTMLAnchorElement
        ..href = objectUrl
        ..download = filename
        ..style.display = 'none';

      // 5. Append, Click, Remove
      web.document.body?.appendChild(anchor);
      anchor.click();
      web.document.body?.removeChild(anchor);

      // 6. Revoke URL to free memory
      web.URL.revokeObjectURL(objectUrl);
    } else {
      throw Exception("Server responded with ${response.statusCode}");
    }
  } catch (e) {
    debugPrint("Download failed: $e");
    rethrow;
  }
}
