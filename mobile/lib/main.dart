import 'dart:convert';
import 'dart:math' as math;
import 'package:flutter/material.dart';
import 'package:http/http.dart' as http;
import 'package:file_picker/file_picker.dart';
import 'qr_scanner.dart';

void main() {
  runApp(const PeerDriveApp());
}

class PeerDriveApp extends StatelessWidget {
  const PeerDriveApp({super.key});

  @override
  Widget build(BuildContext context) {
    return MaterialApp(
      title: 'PeerDrive',
      theme: ThemeData(
        colorScheme: ColorScheme.fromSeed(seedColor: Colors.deepPurple),
        useMaterial3: true,
      ),
      home: const DashboardScreen(),
    );
  }
}

class DashboardScreen extends StatefulWidget {
  const DashboardScreen({super.key});

  @override
  State<DashboardScreen> createState() => _DashboardScreenState();
}

class _DashboardScreenState extends State<DashboardScreen> {
  String peerId = "Connecting to daemon...";
  bool isConnected = false;
  List<dynamic> shares = [];

  @override
  void initState() {
    super.initState();
    _fetchStatus();
    _fetchShares();
  }

  Future<void> _fetchStatus() async {
    try {
      final response = await http.get(Uri.parse('http://localhost:8080/api/status'));
      if (response.statusCode == 200) {
        final data = json.decode(response.body);
        setState(() {
          peerId = data['peer_id'];
          isConnected = true;
        });
      }
    } catch (e) {
      setState(() {
        peerId = "Daemon not reachable ($e)";
        isConnected = false;
      });
      Future.delayed(const Duration(seconds: 2), _fetchStatus);
    }
  }

  Future<void> _fetchShares() async {
    if (!isConnected) {
      Future.delayed(const Duration(seconds: 2), _fetchShares);
      return;
    }
    try {
      final response = await http.get(Uri.parse('http://localhost:8080/api/shares'));
      if (response.statusCode == 200) {
        setState(() {
          shares = json.decode(response.body);
        });
      }
    } catch (e) {
      print("Failed to fetch shares: $e");
    }
    Future.delayed(const Duration(seconds: 2), _fetchShares);
  }

  Future<void> _shareFile() async {
    final PlatformFile? file = await FilePicker.pickFile();
    if (file != null && file.path != null) {
      final path = file.path!;
      
      // Call daemon API to share file
      try {
        final response = await http.post(
          Uri.parse('http://localhost:8080/api/shares'),
          headers: {'Content-Type': 'application/json'},
          body: json.encode({'path': path}),
        );
        
        if (response.statusCode == 200) {
          ScaffoldMessenger.of(context).showSnackBar(
            const SnackBar(content: Text('File shared successfully!')),
          );
          _fetchShares();
        } else {
          ScaffoldMessenger.of(context).showSnackBar(
            SnackBar(content: Text('Failed to share: ${response.body}')),
          );
        }
      } catch (e) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text('Error communicating with daemon: $e')),
        );
      }
    }
  }

  Future<void> _joinShare(String uri) async {
    // Expected format: peerdrive://<peerId>/<shareId>
    if (!uri.startsWith('peerdrive://')) {
      ScaffoldMessenger.of(context).showSnackBar(const SnackBar(content: Text('Invalid PeerDrive link')));
      return;
    }
    
    try {
      final parts = uri.replaceFirst('peerdrive://', '').split('/');
      if (parts.length < 2) throw Exception('Invalid link format');
      
      final hostPeerId = parts[0];
      final shareId = parts[1];
      
      final response = await http.post(
        Uri.parse('http://localhost:8080/api/download'),
        headers: {'Content-Type': 'application/json'},
        body: json.encode({
          'share_id': shareId,
          'host_peer_id': hostPeerId,
        }),
      );
      
      if (response.statusCode == 200) {
        ScaffoldMessenger.of(context).showSnackBar(const SnackBar(content: Text('Joining Swarm...')));
      } else {
        ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text('Failed: ${response.body}')));
      }
    } catch (e) {
      ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text('Error: $e')));
    }
  }

  Future<void> _scanQR() async {
    final result = await Navigator.push(
      context,
      MaterialPageRoute(builder: (context) => const QRScannerScreen()),
    );
    
    if (result != null && result is String) {
      _joinShare(result);
    }
  }

  Future<void> _pasteLink() async {
    TextEditingController controller = TextEditingController();
    showDialog(
      context: context,
      builder: (context) => AlertDialog(
        title: const Text('Paste Share Link'),
        content: TextField(
          controller: controller,
          decoration: const InputDecoration(hintText: 'peerdrive://...'),
        ),
        actions: [
          TextButton(
            onPressed: () => Navigator.pop(context),
            child: const Text('Cancel'),
          ),
          ElevatedButton(
            onPressed: () {
              Navigator.pop(context);
              if (controller.text.isNotEmpty) {
                _joinShare(controller.text);
              }
            },
            child: const Text('Join'),
          ),
        ],
      ),
    );
  }

  String _formatBytes(int bytes) {
    if (bytes <= 0) return "0 B";
    const suffixes = ["B", "KB", "MB", "GB", "TB"];
    var i = (math.log(bytes) / math.log(1024)).floor();
    return ((bytes / math.pow(1024, i)).toStringAsFixed(2)) + ' ' + suffixes[i];
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(
        title: const Text('PeerDrive Mobile'),
        backgroundColor: Theme.of(context).colorScheme.inversePrimary,
      ),
      body: Column(
        children: [
          Container(
            padding: const EdgeInsets.all(16.0),
            color: Colors.deepPurple.withOpacity(0.05),
            child: Column(
              children: [
                Icon(
                  isConnected ? Icons.cloud_done : Icons.cloud_sync, 
                  size: 48, 
                  color: isConnected ? Colors.green : Colors.deepPurple
                ),
                const SizedBox(height: 8),
                SelectableText(
                  peerId,
                  textAlign: TextAlign.center,
                  style: TextStyle(
                    fontFamily: 'monospace', 
                    fontSize: 12,
                    color: isConnected ? Colors.black87 : Colors.grey
                  ),
                ),
              ],
            ),
          ),
          const SizedBox(height: 16),
          Row(
            mainAxisAlignment: MainAxisAlignment.center,
            children: [
              ElevatedButton.icon(
                onPressed: isConnected ? _scanQR : null,
                icon: const Icon(Icons.qr_code_scanner),
                label: const Text('Scan QR'),
              ),
              const SizedBox(width: 16),
              ElevatedButton.icon(
                onPressed: isConnected ? _pasteLink : null,
                icon: const Icon(Icons.link),
                label: const Text('Paste Link'),
              ),
            ],
          ),
          const SizedBox(height: 16),
          const Divider(),
          const Text('Active Shares', style: TextStyle(fontSize: 18, fontWeight: FontWeight.bold)),
          Expanded(
            child: ListView.builder(
              itemCount: shares.length,
              itemBuilder: (context, index) {
                final share = shares[index];
                final int size = share['file_size'] ?? 0;
                final int downloaded = share['downloaded_bytes'] ?? 0;
                final double progress = size > 0 ? downloaded / size : 0.0;
                final bool isDownloading = downloaded > 0 && downloaded < size;

                return Card(
                  margin: const EdgeInsets.symmetric(horizontal: 16, vertical: 8),
                  child: ListTile(
                    leading: Icon(
                      isDownloading ? Icons.downloading : Icons.file_present,
                      color: isDownloading ? Colors.blue : Colors.deepPurple,
                    ),
                    title: Text(share['filename'] ?? 'Unknown File'),
                    subtitle: Column(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: [
                        Text(_formatBytes(size)),
                        if (isDownloading) ...[
                          const SizedBox(height: 8),
                          LinearProgressIndicator(value: progress),
                          const SizedBox(height: 4),
                          Text('${(progress * 100).toStringAsFixed(1)}% downloaded'),
                        ],
                      ],
                    ),
                    trailing: PopupMenuButton<String>(
                      onSelected: (value) async {
                        if (value == 'peers') {
                          try {
                            final res = await http.get(Uri.parse('http://localhost:8080/api/shares/${share['id']}/peers'));
                            if (res.statusCode == 200) {
                              final data = json.decode(res.body);
                              showDialog(
                                context: context,
                                builder: (context) => AlertDialog(
                                  title: const Text('Swarm Info'),
                                  content: Text('Seeders: ${data['seeders']}\nLeechers: ${data['leechers']}'),
                                  actions: [
                                    TextButton(
                                      onPressed: () => Navigator.pop(context),
                                      child: const Text('OK'),
                                    ),
                                  ],
                                ),
                              );
                            }
                          } catch (e) {
                            ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text('Error: $e')));
                          }
                        } else if (value == 'copy') {
                          final link = 'peerdrive://$peerId/${share['id']}';
                          ScaffoldMessenger.of(context).showSnackBar(
                            SnackBar(content: Text('Copied P2P Link: $link')),
                          );
                        }
                      },
                      itemBuilder: (BuildContext context) => [
                        const PopupMenuItem(
                          value: 'copy',
                          child: Text('Copy Share Link'),
                        ),
                        const PopupMenuItem(
                          value: 'peers',
                          child: Text('View Seeders & Leechers'),
                        ),
                      ],
                    ),
                  ),
                );
              },
            ),
          )
        ],
      ),
      floatingActionButton: FloatingActionButton(
        onPressed: isConnected ? _shareFile : null,
        tooltip: 'Share New File',
        child: const Icon(Icons.add),
      ),
    );
  }
}
