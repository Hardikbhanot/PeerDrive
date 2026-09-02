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
    FilePickerResult? result = await FilePicker.platform.pickFiles();
    if (result != null && result.files.single.path != null) {
      final path = result.files.single.path!;
      
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

  Future<void> _scanQR() async {
    final result = await Navigator.push(
      context,
      MaterialPageRoute(builder: (context) => const QRScannerScreen()),
    );
    
    if (result != null && result is String) {
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(content: Text('Scanned: $result')),
      );
      // Here we will parse the P2P connection string and call /api/download
    }
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
          ElevatedButton.icon(
            onPressed: isConnected ? _scanQR : null,
            icon: const Icon(Icons.qr_code_scanner),
            label: const Text('Scan QR to Join Share'),
          ),
          const SizedBox(height: 16),
          const Divider(),
          const Text('Active Shares', style: TextStyle(fontSize: 18, fontWeight: FontWeight.bold)),
          Expanded(
            child: ListView.builder(
              itemCount: shares.length,
              itemBuilder: (context, index) {
                final share = shares[index];
                return ListTile(
                  leading: const Icon(Icons.file_present),
                  title: Text(share['filename']),
                  subtitle: Text(_formatBytes(share['size'])),
                  trailing: IconButton(
                    icon: const Icon(Icons.delete, color: Colors.red),
                    onPressed: () {},
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
