# PeerDrive Architecture

PeerDrive is designed to provide seamless peer-to-peer file synchronization and sharing across personal devices and with friends, leveraging BitTorrent-like technology under the hood.

## Core Concepts

The system is explicitly separated into two main concepts:

### 1. My Drive (Private Devices)
*   **Purpose:** Sync private files across your own trusted devices.
*   **Trust Model:** Devices within this group (e.g., Laptop, Phone, Desktop) are trusted.
*   **Behavior:** Files sync seamlessly between these devices.

### 2. Shared Files (Isolated Swarms)
*   **Purpose:** Share specific files or folders with others.
*   **Trust Model:** Temporary, permission-controlled access for specific users (e.g., Friend A, Friend B).
*   **Behavior:**
    *   **Isolated Swarms:** Each shared file or folder gets its own unique Share ID / Torrent metadata. Sharing one file does *not* expose the rest of the device's filesystem.
    *   **Multi-Device Seeding:** If a shared file exists on multiple trusted devices (e.g., Laptop, Phone, Desktop), all of them can seed the file to the recipient simultaneously, speeding up the transfer.
    *   **Granular Control:** The client does not advertise other files; it only responds to requests for authorized Share IDs.

## Sharing Mechanics

When a file or folder is shared:
1.  **Metadata Generation:** A unique Content ID and Share ID are generated.
2.  **Authorization:** The recipient's client requests to join the Share ID. The sender's client verifies authorization and then BitTorrent handles the piece transfer.

## Advanced Share Controls

Shares are designed to be highly configurable and revocable, making PeerDrive a controlled sharing environment rather than a standard open torrent client:
*   **Revoke Access:** Stop seeding a specific share to a specific person instantly.
*   **Expiration:** Shares can be set to expire after a certain time (e.g., 24 hours).
*   **Password Protection:** Require a password to join the share.
*   **Maximum Downloads:** Limit the number of times a file can be downloaded.
*   **Read-Only:** Restrict recipients from modifying shared folders.
*   **Re-sharing Rules:** Control whether the recipient is allowed to re-share the files.

## Summary

PeerDrive acts as an intelligent coordinator: "I choose a file, choose who gets it, and PeerDrive figures out the P2P transfer," ensuring complete separation between private device storage and explicit, secure shares.
