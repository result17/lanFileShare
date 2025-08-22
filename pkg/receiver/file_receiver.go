package receiver

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/rescp17/lanFileSharer/internal/app_events/receiver"
	"github.com/rescp17/lanFileSharer/pkg/fileInfo"
	"github.com/rescp17/lanFileSharer/pkg/transfer"
)

// FileReceiver manages the reception and reconstruction of files
type FileReceiver struct {
	serializer   transfer.MessageSerializer
	currentFiles map[string]*FileReception // filePath -> FileReception
	outputDir    string
	mu           sync.RWMutex
	uiMessages   chan<- tea.Msg // Channel to send status updates to UI

	// Session tracking
	expectedFiles   int  // Total number of files expected in this session
	completedFiles  int  // Number of files completed
	sessionComplete bool // Whether the entire session is complete
}

// ReceptionStatus represents the current status of file reception
type ReceptionStatus int

const (
	StatusPending ReceptionStatus = iota
	StatusReceiving
	StatusVerifying
	StatusCompleted
	StatusFailed
	StatusCancelled
)

// FileReception tracks the state of receiving a single file
type FileReception struct {
	FilePath     string
	FileName     string
	TotalSize    int64
	ReceivedSize int64
	ExpectedHash string
	File         *os.File
	// Remove Chunks cache, support out-of-order direct writing
	ReceivedChunks  map[uint32]bool // Track received chunk sequence numbers
	mu              sync.RWMutex    // Protect concurrent writes
	IsComplete      bool
	Status          ReceptionStatus
	VerificationErr error
	OutputPath      string // Full path to the output file
}

// NewFileReceiver creates a new file receiver
func NewFileReceiver(outputDir string, uiMessages chan<- tea.Msg) *FileReceiver {
	return &FileReceiver{
		serializer:   transfer.NewJSONSerializer(),
		currentFiles: make(map[string]*FileReception),
		outputDir:    outputDir,
		uiMessages:   uiMessages,
	}
}

// SetExpectedFiles sets the total number of files expected in this session
func (fr *FileReceiver) SetExpectedFiles(count int) {
	fr.mu.Lock()
	defer fr.mu.Unlock()
	fr.expectedFiles = count
	slog.Info("Set expected files for session", "count", count)
}

// ProcessChunk processes a single chunk message
func (fr *FileReceiver) ProcessChunk(data []byte) error {
	// Deserialize the chunk message
	chunkMsg, err := fr.serializer.Unmarshal(data)
	if err != nil {
		return fmt.Errorf("failed to unmarshal chunk message: %w", err)
	}

	fr.mu.Lock()
	defer fr.mu.Unlock()

	// Get or create file reception
	fileReception, exists := fr.currentFiles[chunkMsg.FileID]
	if !exists {
		// Create output file path
		// Sanitize the filename to prevent path traversal
		cleanFileName := filepath.Base(chunkMsg.FileName)
		outputPath := filepath.Join(fr.outputDir, cleanFileName)

		if !strings.HasPrefix(outputPath, filepath.Clean(fr.outputDir)) {
			return fmt.Errorf("invalid output path: %s", outputPath)
		}

		// Create new file reception
		fileReception = &FileReception{
			FilePath:       chunkMsg.FileID,
			FileName:       chunkMsg.FileName,
			TotalSize:      chunkMsg.TotalSize,
			ExpectedHash:   chunkMsg.ExpectedHash,
			ReceivedChunks: make(map[uint32]bool),
			Status:         StatusReceiving,
			OutputPath:     outputPath,
		}

		// Create output file
		file, err := os.Create(outputPath)
		if err != nil {
			fileReception.Status = StatusFailed
			return fmt.Errorf("failed to create output file %s: %w", outputPath, err)
		}
		fileReception.File = file
		fr.currentFiles[chunkMsg.FileID] = fileReception

		slog.Info("Started receiving file", "fileName", chunkMsg.FileName, "totalSize", chunkMsg.TotalSize)
		if fr.uiMessages != nil {
			fr.uiMessages <- receiver.StatusUpdateMsg{Message: fmt.Sprintf("Receiving file: %s", chunkMsg.FileName)}
		}
	}

	// Use offset to write chunk directly, supporting out-of-order writes
	if err := fr.writeChunkAtOffset(fileReception, chunkMsg); err != nil {
		return fmt.Errorf("failed to write chunk at offset: %w", err)
	}

	// Check if file is complete
	if fileReception.ReceivedSize >= fileReception.TotalSize {
		if err := fr.completeFile(fileReception); err != nil {
			return fmt.Errorf("failed to complete file: %w", err)
		}
		delete(fr.currentFiles, chunkMsg.FileID)

		// Increment completed files counter
		fr.completedFiles++
		slog.Info("File reception completed", "fileName", fileReception.FileName,
			"completed", fr.completedFiles, "expected", fr.expectedFiles)

		// Check if all files are completed
		if fr.expectedFiles > 0 && fr.completedFiles >= fr.expectedFiles && !fr.sessionComplete {
			fr.sessionComplete = true
			slog.Info("All files received successfully", "totalFiles", fr.completedFiles)
			if fr.uiMessages != nil {
				fr.uiMessages <- receiver.TransferFinishedMsg{}
			}
		}
	}

	return nil
}

// writeChunkAtOffset writes chunk directly to file at specified offset (supports out-of-order writes)
func (fr *FileReceiver) writeChunkAtOffset(fileReception *FileReception, chunkMsg *transfer.ChunkMessage) error {
	// Lock to protect concurrent writes
	fileReception.mu.Lock()
	defer fileReception.mu.Unlock()

	// Check if this chunk has already been received
	if fileReception.ReceivedChunks[chunkMsg.SequenceNo] {
		slog.Debug("Chunk already received, skipping", "fileID", chunkMsg.FileID, "sequence", chunkMsg.SequenceNo)
		return nil // Duplicate chunk, skip directly
	}

	// Use offset to seek to the correct position in the file
	if _, err := fileReception.File.Seek(chunkMsg.Offset, io.SeekStart); err != nil {
		return fmt.Errorf("failed to seek to offset %d: %w", chunkMsg.Offset, err)
	}

	// Write chunk data
	bytesWritten, err := fileReception.File.Write(chunkMsg.Data)
	if err != nil {
		return fmt.Errorf("failed to write chunk %d at offset %d: %w", chunkMsg.SequenceNo, chunkMsg.Offset, err)
	}

	// Verify the number of bytes written
	if bytesWritten != len(chunkMsg.Data) {
		return fmt.Errorf("incomplete write: expected %d bytes, wrote %d bytes", len(chunkMsg.Data), bytesWritten)
	}

	// Force flush to disk (optional, improves data safety)
	if err := fileReception.File.Sync(); err != nil {
		slog.Warn("Failed to sync file to disk", "error", err)
	}

	// Mark chunk as received
	fileReception.ReceivedChunks[chunkMsg.SequenceNo] = true
	fileReception.ReceivedSize += int64(len(chunkMsg.Data))

	slog.Debug("Chunk written successfully",
		"fileID", chunkMsg.FileID,
		"sequence", chunkMsg.SequenceNo,
		"offset", chunkMsg.Offset,
		"size", len(chunkMsg.Data),
		"progress", fmt.Sprintf("%.1f%%", float64(fileReception.ReceivedSize)/float64(fileReception.TotalSize)*100))

	return nil
}

// completeFile finalizes the file reception with integrity verification
func (fr *FileReceiver) completeFile(fileReception *FileReception) error {
	// Close the file first
	if err := fileReception.File.Close(); err != nil {
		fileReception.Status = StatusFailed
		return fmt.Errorf("failed to close file: %w", err)
	}

	// Perform integrity verification if expected hash is provided
	if fileReception.ExpectedHash != "" {
		fileReception.Status = StatusVerifying
		slog.Info("Starting file integrity verification", "fileName", fileReception.FileName, "expectedHash", fileReception.ExpectedHash)

		if fr.uiMessages != nil {
			fr.uiMessages <- receiver.StatusUpdateMsg{Message: fmt.Sprintf("Verifying integrity of file: %s", fileReception.FileName)}
		}

		if err := fr.verifyFileIntegrity(fileReception); err != nil {
			fileReception.Status = StatusFailed
			fileReception.VerificationErr = err

			// Clean up corrupted file
			if cleanupErr := fr.cleanupCorruptedFile(fileReception); cleanupErr != nil {
				slog.Error("Failed to cleanup corrupted file", "fileName", fileReception.FileName, "error", cleanupErr)
			}

			slog.Error("File integrity verification failed", "fileName", fileReception.FileName, "error", err)
			if fr.uiMessages != nil {
				fr.uiMessages <- receiver.StatusUpdateMsg{Message: fmt.Sprintf("File verification failed: %s - %v", fileReception.FileName, err)}
			}

			return fmt.Errorf("file integrity verification failed for %s: %w", fileReception.FileName, err)
		}

		slog.Info("File integrity verification successful", "fileName", fileReception.FileName)
		if fr.uiMessages != nil {
			fr.uiMessages <- receiver.StatusUpdateMsg{Message: fmt.Sprintf("File verified successfully: %s", fileReception.FileName)}
		}
	}

	// Mark as completed
	fileReception.Status = StatusCompleted
	fileReception.IsComplete = true

	if fr.uiMessages != nil {
		fr.uiMessages <- receiver.StatusUpdateMsg{Message: fmt.Sprintf("File reception completed: %s", fileReception.FileName)}
	}

	return nil
}

// verifyFileIntegrity verifies the integrity of a received file using SHA256 hash
func (fr *FileReceiver) verifyFileIntegrity(fileReception *FileReception) error {
	// Create a FileNode for the received file to use existing VerifySHA256 method
	fileNode := &fileInfo.FileNode{
		Path: fileReception.OutputPath,
	}

	// Verify the file hash using the existing VerifySHA256 method
	isValid, err := fileNode.VerifySHA256(fileReception.ExpectedHash)
	if err != nil {
		return fmt.Errorf("failed to calculate file hash: %w", err)
	}

	if !isValid {
		return fmt.Errorf("file hash mismatch - file may be corrupted during transmission")
	}

	return nil
}

// cleanupCorruptedFile removes a corrupted file and logs the cleanup
func (fr *FileReceiver) cleanupCorruptedFile(fileReception *FileReception) error {
	if fileReception.OutputPath == "" {
		return nil // No file to cleanup
	}

	slog.Info("Cleaning up corrupted file", "fileName", fileReception.FileName, "path", fileReception.OutputPath)

	if err := os.Remove(fileReception.OutputPath); err != nil {
		if !os.IsNotExist(err) {
			return fmt.Errorf("failed to remove corrupted file %s: %w", fileReception.OutputPath, err)
		}
	}

	return nil
}
