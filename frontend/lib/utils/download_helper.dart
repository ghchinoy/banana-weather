import 'dart:html' as html;
import 'package:http/http.dart' as http;

/// Downloads a file from [url] and saves it as [filename].
/// Uses Blob to support custom filenames and handle CORS correctly.
Future<void> downloadFile(String url, String filename) async {
  try {
    // 1. Fetch the data as bytes
    final response = await http.get(Uri.parse(url));
    
    if (response.statusCode == 200) {
      // 2. Create a Blob from the bytes
      final blob = html.Blob([response.bodyBytes]);
      
      // 3. Create an Object URL
      final objectUrl = html.Url.createObjectUrlFromBlob(blob);
      
      // 4. Create a hidden anchor tag
      final anchor = html.AnchorElement(href: objectUrl)
        ..setAttribute("download", filename)
        ..style.display = 'none';
      
      // 5. Append, Click, Remove
      html.document.body?.children.add(anchor);
      anchor.click();
      html.document.body?.children.remove(anchor);
      
      // 6. Revoke URL to free memory
      html.Url.revokeObjectUrl(objectUrl);
    } else {
      throw Exception("Server responded with ${response.statusCode}");
    }
  } catch (e) {
    print("Download failed: $e");
    rethrow;
  }
}
