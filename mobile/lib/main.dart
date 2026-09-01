import 'dart:convert';
import 'package:flutter/material.dart';
import 'package:http/http.dart' as http;

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

  @override
  void initState() {
    super.initState();
    _fetchStatus();
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
      // Retry after a short delay since daemon might take a second to boot
      Future.delayed(const Duration(seconds: 2), _fetchStatus);
    }
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(
        title: const Text('PeerDrive Mobile'),
        backgroundColor: Theme.of(context).colorScheme.inversePrimary,
      ),
      body: Center(
        child: Column(
          mainAxisAlignment: MainAxisAlignment.center,
          children: <Widget>[
            Icon(
              isConnected ? Icons.cloud_done : Icons.cloud_sync, 
              size: 64, 
              color: isConnected ? Colors.green : Colors.deepPurple
            ),
            const SizedBox(height: 16),
            const Text(
              'Your Peer ID:',
              style: TextStyle(fontSize: 18, fontWeight: FontWeight.bold),
            ),
            Padding(
              padding: const EdgeInsets.all(16.0),
              child: SelectableText(
                peerId,
                textAlign: TextAlign.center,
                style: TextStyle(
                  fontFamily: 'monospace', 
                  color: isConnected ? Colors.black87 : Colors.grey
                ),
              ),
            ),
            const SizedBox(height: 32),
            ElevatedButton.icon(
              onPressed: isConnected ? () {} : null,
              icon: const Icon(Icons.qr_code_scanner),
              label: const Text('Scan QR to Join Share'),
            )
          ],
        ),
      ),
      floatingActionButton: FloatingActionButton(
        onPressed: isConnected ? () {} : null,
        tooltip: 'Share New File',
        child: const Icon(Icons.add),
      ),
    );
  }
}
