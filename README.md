# PeerDrive

PeerDrive is a peer-to-peer file sharing and synchronization application. It allows you to seamlessly sync files across your own trusted devices ("My Drive") and securely share specific files or folders with others using isolated, controlled P2P swarms.

## Key Features

*   **My Drive:** Keep your personal files synced across your laptop, phone, and desktop.
*   **Isolated Sharing:** Share specific files without exposing your entire drive. Each share acts as its own isolated BitTorrent swarm.
*   **Multi-Device Seeding:** Your trusted devices work together to seed shared files to your friends faster.
*   **Granular Access Control:** 
    *   Revoke access at any time.
    *   Set expiration dates (e.g., 24 hours).
    *   Require passwords.
    *   Limit the number of downloads.
    *   Prevent re-sharing.

See [ARCHITECTURE.md](ARCHITECTURE.md) for more details on the system design.
