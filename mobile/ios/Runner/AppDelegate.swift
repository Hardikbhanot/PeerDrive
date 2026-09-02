import Flutter
import UIKit
import Daemon

@main
@objc class AppDelegate: FlutterAppDelegate {
  override func application(
    _ application: UIApplication,
    didFinishLaunchingWithOptions launchOptions: [UIApplication.LaunchOptionsKey: Any]?
  ) -> Bool {
    
    // Start the Go Daemon!
    var error: NSError?
    let paths = NSSearchPathForDirectoriesInDomains(.documentDirectory, .userDomainMask, true)
    let configDir = paths[0] + "/.peerdrive"
    
    // Start daemon on port 8080 (same as desktop)
    DaemonNewDaemon(configDir, 8080, "", &error)
    if let error = error {
        print("Failed to start PeerDrive Daemon: \(error.localizedDescription)")
    } else {
        print("PeerDrive Daemon started successfully on port 8080")
    }

    GeneratedPluginRegistrant.register(with: self)
    return super.application(application, didFinishLaunchingWithOptions: launchOptions)
  }

  func didInitializeImplicitFlutterEngine(_ engineBridge: FlutterImplicitEngineBridge) {
    GeneratedPluginRegistrant.register(with: engineBridge.pluginRegistry)
  }
}
