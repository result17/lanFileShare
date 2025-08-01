# Requirements Document

## Introduction

This feature aims to implement a secure file transfer mechanism for systems with established WebRTC connections. The functionality will build upon existing signaling exchange and handshake processes, adding file transfer capabilities with checksum verification to ensure file integrity and digital signatures to ensure file structure consistency with requested file structure.

## Requirements

### Requirement 1

**User Story:** As a sender user, I want to implement the actual file transfer over WebRTC data channels, so that files can be transmitted efficiently using the existing checksum infrastructure.

#### Acceptance Criteria

1. WHEN WebRTC connection is established THEN system SHALL create data channels for file transfer
2. WHEN file transfer begins THEN system SHALL split files into chunks suitable for WebRTC data channel transmission
3. WHEN transmitting file chunks THEN system SHALL include chunk metadata (sequence number, file ID, chunk hash)
4. WHEN all chunks for a file are sent THEN system SHALL send file completion message with expected checksum
5. WHEN transfer completes THEN system SHALL verify all files using existing CalcChecksum() method

### Requirement 2

**User Story:** As a receiver user, I want to receive and reconstruct files from WebRTC data channel chunks, so that I can verify file integrity using existing checksum validation.

#### Acceptance Criteria

1. WHEN receiving file chunks THEN system SHALL buffer and order chunks by sequence number
2. WHEN file completion message is received THEN system SHALL reconstruct complete file from chunks
3. WHEN file reconstruction completes THEN system SHALL verify file checksum using VerifySHA256() method
4. IF file checksum verification fails THEN system SHALL reject file and notify sender
5. WHEN file verification succeeds THEN system SHALL save file to designated download location

### Requirement 3

**User Story:** As a system administrator, I want file structure to be cryptographically signed, so that receivers can verify the authenticity of the declared file structure and prevent tampering.

#### Acceptance Criteria

1. WHEN sender prepares file transfer THEN system SHALL generate RSA/ECDSA key pair for session
2. WHEN creating file structure payload THEN system SHALL sign the FileNode tree structure with private key
3. WHEN sending /ask request THEN system SHALL include public key and signature in payload
4. WHEN receiver processes /ask request THEN system SHALL verify file structure signature using provided public key
5. IF signature verification fails THEN system SHALL reject transfer request immediately

### Requirement 4

**User Story:** As a user, I want real-time progress tracking and robust error handling during file transfer, so that I can monitor transfer status and handle failures gracefully.

#### Acceptance Criteria

1. WHEN file transfer begins THEN system SHALL track and display progress per file and overall transfer
2. WHEN chunk transmission fails THEN system SHALL implement exponential backoff retry mechanism
3. WHEN WebRTC connection drops THEN system SHALL attempt to re-establish connection and resume transfer
4. WHEN user cancels transfer THEN system SHALL send cancellation message and clean up partial files
5. WHEN transfer completes THEN system SHALL display summary with verification results for each file

### Requirement 5

**User Story:** As a developer, I want file transfer to integrate seamlessly with existing WebRTC and signaling infrastructure, so that the implementation builds upon current architecture patterns.

#### Acceptance Criteria

1. WHEN implementing SendFiles method THEN system SHALL use existing WebRTC PeerConnection from SenderConn
2. WHEN creating data channels THEN system SHALL use existing CreateDataChannel method in SenderConn
3. WHEN handling transfer events THEN system SHALL send updates through existing uiMessages channel pattern
4. WHEN transfer state changes THEN system SHALL integrate with existing app event system (sender/receiver packages)
5. WHEN errors occur THEN system SHALL use existing error handling patterns and logging infrastructure