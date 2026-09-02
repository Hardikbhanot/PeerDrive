package com.example.mobile

import io.flutter.embedding.android.FlutterActivity
import io.flutter.embedding.engine.FlutterEngine
import android.os.Bundle
import daemon.Daemon

class MainActivity : FlutterActivity() {
    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        
        try {
            val configDir = context.filesDir.absolutePath + "/.peerdrive"
            // Start the Go Daemon on Android!
            Daemon.newDaemon(configDir, 8080, "")
            println("PeerDrive Daemon started successfully on port 8080")
        } catch (e: Exception) {
            println("Failed to start PeerDrive Daemon: " + e.message)
        }
    }
}
