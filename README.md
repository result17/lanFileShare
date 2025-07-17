# lanFileSharer

A tool for sharing files directly between devices on a local network.

## How It Works

The connection process is designed to be robust and efficient, combining direct IP communication with the flexibility of mDNS hostname resolution. It works in two main phases:

### Phase 1: Discovery and Initial Handshake (DNS-SD + HTTP)

1.  **Service Discovery**: When a receiver application starts, it broadcasts its presence on the local network using mDNS/DNS-SD.
2.  **IP Resolution**: A sender application discovers the receiver via this broadcast and resolves its `.local` hostname to a specific IP address (e.g., `192.168.1.55`).
3.  **HTTP Handshake**: The sender then initiates a direct HTTP connection to the receiver using the discovered IP address. This connection is used as the primary signaling channel to exchange essential metadata, such as the structure of the files to be transferred and the initial WebRTC session information (SDP Offer/Answer).

### Phase 2: High-Speed Data Transfer (WebRTC)

1.  **PeerConnection Establishment**: Once the handshake is complete, both devices proceed to establish a WebRTC `PeerConnection`.
2.  **P2P Transfer**: This connection is used for the actual high-speed, peer-to-peer transfer of file data, leveraging the performance of WebRTC's data channels.

### Robustness Through `SetMulticastDNSMode`

To make the connection process resilient against potential IP address changes (e.g., due to DHCP lease renewals), the underlying WebRTC `SettingEngine` is configured with `MulticastDNSModeEnabled`.

-   **What it does**: This setting empowers the ICE agent within WebRTC to resolve `.local` hostnames directly during the connectivity check phase.
-   **Why it's important**: While the initial connection uses a pre-resolved IP, the ICE negotiation includes candidates with `.local` hostnames. If the initial IP has become stale or invalid, the ICE agent can use mDNS to re-resolve the hostname to the device's current IP address. This acts as a powerful fallback mechanism, ensuring a connection can still be established, making the application significantly more robust.

This dual approach leverages the speed of direct IP communication for the initial handshake while retaining the flexibility and resilience of mDNS for the underlying P2P data connection.
