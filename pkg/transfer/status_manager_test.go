package transfer

import (
	"errors"
	"fmt"
	"sync"
	"testing"
)

func TestNewTransferStatusManager(t *testing.T) {
	manager := NewTransferStatusManager()
	
	if manager == nil {
		t.Fatal("NewTransferStatusManager returned nil")
	}
	
	if manager.transfers == nil {
		t.Error("transfers map should be initialized")
	}
	
	if manager.config == nil {
		t.Error("config should be initialized")
	}
	
	// Test that default config is used
	defaultConfig := DefaultTransferConfig()
	if manager.config.ChunkSize != defaultConfig.ChunkSize {
		t.Errorf("Expected ChunkSize %d, got %d", defaultConfig.ChunkSize, manager.config.ChunkSize)
	}
}

func TestNewTransferStatusManagerWithConfig(t *testing.T) {
	customConfig := &TransferConfig{
		ChunkSize:              32 * 1024,
		MinChunkSize:           MinChunkSize,
		MaxChunkSize:           MaxChunkSize,
		MaxConcurrentTransfers: 5,
		MaxConcurrentChunks:    25,
		BufferSize:             4096,
		DefaultRetryPolicy:     DefaultRetryPolicy(),
		EventBufferSize:        50,
	}
	
	manager := NewTransferStatusManagerWithConfig(customConfig)
	
	if manager == nil {
		t.Fatal("NewTransferStatusManagerWithConfig returned nil")
	}
	
	if manager.config.ChunkSize != customConfig.ChunkSize {
		t.Errorf("Expected ChunkSize %d, got %d", customConfig.ChunkSize, manager.config.ChunkSize)
	}
	
	if manager.config.MaxConcurrentTransfers != customConfig.MaxConcurrentTransfers {
		t.Errorf("Expected MaxConcurrentTransfers %d, got %d", 
			customConfig.MaxConcurrentTransfers, manager.config.MaxConcurrentTransfers)
	}
}

func TestNewTransferStatusManagerWithNilConfig(t *testing.T) {
	manager := NewTransferStatusManagerWithConfig(nil)
	
	if manager == nil {
		t.Fatal("NewTransferStatusManagerWithConfig with nil config returned nil")
	}
	
	// Should use default config
	defaultConfig := DefaultTransferConfig()
	if manager.config.ChunkSize != defaultConfig.ChunkSize {
		t.Errorf("Expected default ChunkSize %d, got %d", defaultConfig.ChunkSize, manager.config.ChunkSize)
	}
}

func TestTransferStatusManager_StartTransfer(t *testing.T) {
	manager := NewTransferStatusManager()
	
	tests := []struct {
		name        string
		filePath    string
		totalSize   int64
		expectError bool
		errorType   error
	}{
		{
			name:        "valid transfer",
			filePath:    "/test/file.txt",
			totalSize:   1024,
			expectError: false,
		},
		{
			name:        "empty file path",
			filePath:    "",
			totalSize:   1024,
			expectError: true,
		},
		{
			name:        "negative size",
			filePath:    "/test/file.txt",
			totalSize:   -1,
			expectError: true,
		},
		{
			name:        "zero size file",
			filePath:    "/test/empty.txt",
			totalSize:   0,
			expectError: false,
		},
	}
	
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			status, err := manager.StartTransfer(test.filePath, test.totalSize)
			
			if test.expectError {
				if err == nil {
					t.Error("Expected error, but got nil")
				}
				if status != nil {
					t.Error("Expected nil status on error")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if status == nil {
					t.Error("Expected status, but got nil")
				} else {
					if status.FilePath != test.filePath {
						t.Errorf("Expected FilePath %s, got %s", test.filePath, status.FilePath)
					}
					if status.TotalBytes != test.totalSize {
						t.Errorf("Expected TotalBytes %d, got %d", test.totalSize, status.TotalBytes)
					}
					if status.State != TransferStatePending {
						t.Errorf("Expected state %s, got %s", TransferStatePending, status.State)
					}
				}
			}
		})
	}
}

func TestTransferStatusManager_StartTransfer_Duplicate(t *testing.T) {
	manager := NewTransferStatusManager()
	
	filePath := "/test/file.txt"
	totalSize := int64(1024)
	
	// Start first transfer
	_, err := manager.StartTransfer(filePath, totalSize)
	if err != nil {
		t.Fatalf("First StartTransfer failed: %v", err)
	}
	
	// Try to start duplicate transfer
	_, err = manager.StartTransfer(filePath, totalSize)
	if err == nil {
		t.Error("Expected error for duplicate transfer")
	}
	if !errors.Is(err, ErrTransferAlreadyExists) {
		t.Errorf("Expected ErrTransferAlreadyExists, got %v", err)
	}
}

func TestTransferStatusManager_GetTransferStatus(t *testing.T) {
	manager := NewTransferStatusManager()
	
	filePath := "/test/file.txt"
	totalSize := int64(1024)
	
	// Test getting non-existent transfer
	_, err := manager.GetTransferStatus(filePath)
	if err == nil {
		t.Error("Expected error for non-existent transfer")
	}
	if !errors.Is(err, ErrTransferNotFound) {
		t.Errorf("Expected ErrTransferNotFound, got %v", err)
	}
	
	// Start a transfer
	originalStatus, err := manager.StartTransfer(filePath, totalSize)
	if err != nil {
		t.Fatalf("StartTransfer failed: %v", err)
	}
	
	// Get the transfer status
	retrievedStatus, err := manager.GetTransferStatus(filePath)
	if err != nil {
		t.Errorf("GetTransferStatus failed: %v", err)
	}
	
	// Verify the status matches
	if retrievedStatus.FilePath != originalStatus.FilePath {
		t.Errorf("Expected FilePath %s, got %s", originalStatus.FilePath, retrievedStatus.FilePath)
	}
	if retrievedStatus.TotalBytes != originalStatus.TotalBytes {
		t.Errorf("Expected TotalBytes %d, got %d", originalStatus.TotalBytes, retrievedStatus.TotalBytes)
	}
	if retrievedStatus.State != originalStatus.State {
		t.Errorf("Expected State %s, got %s", originalStatus.State, retrievedStatus.State)
	}
}

func TestTransferStatusManager_GetTransferStatus_EmptyPath(t *testing.T) {
	manager := NewTransferStatusManager()
	
	_, err := manager.GetTransferStatus("")
	if err == nil {
		t.Error("Expected error for empty file path")
	}
}

func TestTransferStatusManager_UpdateProgress(t *testing.T) {
	manager := NewTransferStatusManager()
	
	filePath := "/test/file.txt"
	totalSize := int64(1024)
	
	// Start a transfer
	_, err := manager.StartTransfer(filePath, totalSize)
	if err != nil {
		t.Fatalf("StartTransfer failed: %v", err)
	}
	
	// Update progress
	bytesSent := int64(512)
	err = manager.UpdateProgress(filePath, bytesSent)
	if err != nil {
		t.Errorf("UpdateProgress failed: %v", err)
	}
	
	// Verify progress was updated
	status, err := manager.GetTransferStatus(filePath)
	if err != nil {
		t.Fatalf("GetTransferStatus failed: %v", err)
	}
	
	if status.BytesSent != bytesSent {
		t.Errorf("Expected BytesSent %d, got %d", bytesSent, status.BytesSent)
	}
	
	// Test invalid updates
	tests := []struct {
		name      string
		bytesSent int64
		expectErr bool
	}{
		{"negative bytes", -1, true},
		{"exceeding total", totalSize + 1, true},
		{"valid progress", totalSize, false},
	}
	
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := manager.UpdateProgress(filePath, test.bytesSent)
			if test.expectErr && err == nil {
				t.Error("Expected error, but got nil")
			}
			if !test.expectErr && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
		})
	}
}
func TestTransferStatusManager_CompleteTransfer(t *testing.T) {
	manager := NewTransferStatusManager()
	
	filePath := "/test/file.txt"
	totalSize := int64(1024)
	
	// Test completing non-existent transfer
	err := manager.CompleteTransfer(filePath)
	if err == nil {
		t.Error("Expected error for non-existent transfer")
	}
	if !errors.Is(err, ErrTransferNotFound) {
		t.Errorf("Expected ErrTransferNotFound, got %v", err)
	}
	
	// Start a transfer
	_, err = manager.StartTransfer(filePath, totalSize)
	if err != nil {
		t.Fatalf("StartTransfer failed: %v", err)
	}
	
	// Set state to active (pending can't directly complete)
	err = manager.ResumeTransfer(filePath)
	if err != nil {
		t.Fatalf("ResumeTransfer failed: %v", err)
	}
	
	// Complete the transfer
	err = manager.CompleteTransfer(filePath)
	if err != nil {
		t.Errorf("CompleteTransfer failed: %v", err)
	}
	
	// Verify the transfer is completed
	status, err := manager.GetTransferStatus(filePath)
	if err != nil {
		t.Fatalf("GetTransferStatus failed: %v", err)
	}
	
	if status.State != TransferStateCompleted {
		t.Errorf("Expected state %s, got %s", TransferStateCompleted, status.State)
	}
	
	if status.CompletionTime == nil {
		t.Error("CompletionTime should be set")
	}
	
	if status.BytesSent != status.TotalBytes {
		t.Errorf("Expected BytesSent to equal TotalBytes (%d), got %d", status.TotalBytes, status.BytesSent)
	}
}

func TestTransferStatusManager_FailTransfer(t *testing.T) {
	manager := NewTransferStatusManager()
	
	filePath := "/test/file.txt"
	totalSize := int64(1024)
	testError := errors.New("test error")
	
	// Start a transfer
	_, err := manager.StartTransfer(filePath, totalSize)
	if err != nil {
		t.Fatalf("StartTransfer failed: %v", err)
	}
	
	// Set state to active
	err = manager.ResumeTransfer(filePath)
	if err != nil {
		t.Fatalf("ResumeTransfer failed: %v", err)
	}
	
	// Fail the transfer
	err = manager.FailTransfer(filePath, testError)
	if err != nil {
		t.Errorf("FailTransfer failed: %v", err)
	}
	
	// Verify the transfer is failed
	status, err := manager.GetTransferStatus(filePath)
	if err != nil {
		t.Fatalf("GetTransferStatus failed: %v", err)
	}
	
	if status.State != TransferStateFailed {
		t.Errorf("Expected state %s, got %s", TransferStateFailed, status.State)
	}
	
	if status.LastError == nil {
		t.Error("LastError should be set")
	} else if status.LastError.Error() != testError.Error() {
		t.Errorf("Expected error %v, got %v", testError, status.LastError)
	}
	
	if status.CompletionTime == nil {
		t.Error("CompletionTime should be set")
	}
}

func TestTransferStatusManager_CancelTransfer(t *testing.T) {
	manager := NewTransferStatusManager()
	
	filePath := "/test/file.txt"
	totalSize := int64(1024)
	
	// Start a transfer
	_, err := manager.StartTransfer(filePath, totalSize)
	if err != nil {
		t.Fatalf("StartTransfer failed: %v", err)
	}
	
	// Cancel the transfer
	err = manager.CancelTransfer(filePath)
	if err != nil {
		t.Errorf("CancelTransfer failed: %v", err)
	}
	
	// Verify the transfer is cancelled
	status, err := manager.GetTransferStatus(filePath)
	if err != nil {
		t.Fatalf("GetTransferStatus failed: %v", err)
	}
	
	if status.State != TransferStateCancelled {
		t.Errorf("Expected state %s, got %s", TransferStateCancelled, status.State)
	}
	
	if status.LastError == nil {
		t.Error("LastError should be set")
	}
	
	if status.CompletionTime == nil {
		t.Error("CompletionTime should be set")
	}
}

func TestTransferStatusManager_PauseResumeTransfer(t *testing.T) {
	manager := NewTransferStatusManager()
	
	filePath := "/test/file.txt"
	totalSize := int64(1024)
	
	// Start a transfer
	_, err := manager.StartTransfer(filePath, totalSize)
	if err != nil {
		t.Fatalf("StartTransfer failed: %v", err)
	}
	
	// Set to active state first
	err = manager.ResumeTransfer(filePath)
	if err != nil {
		t.Fatalf("ResumeTransfer failed: %v", err)
	}
	
	// Pause the transfer
	err = manager.PauseTransfer(filePath)
	if err != nil {
		t.Errorf("PauseTransfer failed: %v", err)
	}
	
	// Verify the transfer is paused
	status, err := manager.GetTransferStatus(filePath)
	if err != nil {
		t.Fatalf("GetTransferStatus failed: %v", err)
	}
	
	if status.State != TransferStatePaused {
		t.Errorf("Expected state %s, got %s", TransferStatePaused, status.State)
	}
	
	// Resume the transfer
	err = manager.ResumeTransfer(filePath)
	if err != nil {
		t.Errorf("ResumeTransfer failed: %v", err)
	}
	
	// Verify the transfer is active again
	status, err = manager.GetTransferStatus(filePath)
	if err != nil {
		t.Fatalf("GetTransferStatus failed: %v", err)
	}
	
	if status.State != TransferStateActive {
		t.Errorf("Expected state %s, got %s", TransferStateActive, status.State)
	}
}

func TestTransferStatusManager_GetAllTransfers(t *testing.T) {
	manager := NewTransferStatusManager()
	
	// Initially should be empty
	transfers := manager.GetAllTransfers()
	if len(transfers) != 0 {
		t.Errorf("Expected 0 transfers, got %d", len(transfers))
	}
	
	// Add some transfers
	filePaths := []string{"/test/file1.txt", "/test/file2.txt", "/test/file3.txt"}
	for _, filePath := range filePaths {
		_, err := manager.StartTransfer(filePath, 1024)
		if err != nil {
			t.Fatalf("StartTransfer failed for %s: %v", filePath, err)
		}
	}
	
	// Get all transfers
	transfers = manager.GetAllTransfers()
	if len(transfers) != len(filePaths) {
		t.Errorf("Expected %d transfers, got %d", len(filePaths), len(transfers))
	}
	
	// Verify all file paths are present
	foundPaths := make(map[string]bool)
	for _, transfer := range transfers {
		foundPaths[transfer.FilePath] = true
	}
	
	for _, expectedPath := range filePaths {
		if !foundPaths[expectedPath] {
			t.Errorf("Expected to find transfer for %s", expectedPath)
		}
	}
}
func TestTransferStatusManager_GetOverallProgress(t *testing.T) {
	manager := NewTransferStatusManager()
	
	// Test with no transfers
	progress := manager.GetOverallProgress()
	if progress.TotalTransfers != 0 {
		t.Errorf("Expected 0 total transfers, got %d", progress.TotalTransfers)
	}
	if progress.TotalBytes != 0 {
		t.Errorf("Expected 0 total bytes, got %d", progress.TotalBytes)
	}
	
	// Add some transfers
	filePaths := []string{"/test/file1.txt", "/test/file2.txt", "/test/file3.txt"}
	totalSizes := []int64{1000, 2000, 3000}
	
	for i, filePath := range filePaths {
		_, err := manager.StartTransfer(filePath, totalSizes[i])
		if err != nil {
			t.Fatalf("StartTransfer failed for %s: %v", filePath, err)
		}
	}
	
	// Update progress for some transfers
	manager.UpdateProgress(filePaths[0], 500)  // 50% of file1
	manager.UpdateProgress(filePaths[1], 1000) // 50% of file2
	
	// Get overall progress
	progress = manager.GetOverallProgress()
	
	expectedTotalBytes := int64(6000) // 1000 + 2000 + 3000
	expectedBytesSent := int64(1500)  // 500 + 1000 + 0
	
	if progress.TotalTransfers != 3 {
		t.Errorf("Expected 3 total transfers, got %d", progress.TotalTransfers)
	}
	
	if progress.TotalBytes != expectedTotalBytes {
		t.Errorf("Expected total bytes %d, got %d", expectedTotalBytes, progress.TotalBytes)
	}
	
	if progress.BytesSent != expectedBytesSent {
		t.Errorf("Expected bytes sent %d, got %d", expectedBytesSent, progress.BytesSent)
	}
	
	expectedPercentage := float64(expectedBytesSent) / float64(expectedTotalBytes) * 100.0
	if progress.OverallPercentage != expectedPercentage {
		t.Errorf("Expected percentage %.2f, got %.2f", expectedPercentage, progress.OverallPercentage)
	}
}

func TestTransferStatusManager_RemoveTransfer(t *testing.T) {
	manager := NewTransferStatusManager()
	
	filePath := "/test/file.txt"
	totalSize := int64(1024)
	
	// Test removing non-existent transfer
	err := manager.RemoveTransfer(filePath)
	if err == nil {
		t.Error("Expected error for non-existent transfer")
	}
	if !errors.Is(err, ErrTransferNotFound) {
		t.Errorf("Expected ErrTransferNotFound, got %v", err)
	}
	
	// Start a transfer
	_, err = manager.StartTransfer(filePath, totalSize)
	if err != nil {
		t.Fatalf("StartTransfer failed: %v", err)
	}
	
	// Try to remove active transfer (should fail)
	err = manager.RemoveTransfer(filePath)
	if err == nil {
		t.Error("Expected error when removing active transfer")
	}
	
	// Complete the transfer
	manager.ResumeTransfer(filePath)
	manager.CompleteTransfer(filePath)
	
	// Now removal should succeed
	err = manager.RemoveTransfer(filePath)
	if err != nil {
		t.Errorf("RemoveTransfer failed: %v", err)
	}
	
	// Verify transfer is removed
	_, err = manager.GetTransferStatus(filePath)
	if err == nil {
		t.Error("Expected error after removing transfer")
	}
	if !errors.Is(err, ErrTransferNotFound) {
		t.Errorf("Expected ErrTransferNotFound, got %v", err)
	}
}

func TestTransferStatusManager_GetActiveTransferCount(t *testing.T) {
	manager := NewTransferStatusManager()
	
	// Initially should be 0
	count := manager.GetActiveTransferCount()
	if count != 0 {
		t.Errorf("Expected 0 active transfers, got %d", count)
	}
	
	// Add some transfers
	filePaths := []string{"/test/file1.txt", "/test/file2.txt", "/test/file3.txt"}
	for _, filePath := range filePaths {
		_, err := manager.StartTransfer(filePath, 1024)
		if err != nil {
			t.Fatalf("StartTransfer failed for %s: %v", filePath, err)
		}
	}
	
	// Still 0 because they're pending
	count = manager.GetActiveTransferCount()
	if count != 0 {
		t.Errorf("Expected 0 active transfers (pending), got %d", count)
	}
	
	// Resume some transfers
	manager.ResumeTransfer(filePaths[0])
	manager.ResumeTransfer(filePaths[1])
	
	// Should be 2 active
	count = manager.GetActiveTransferCount()
	if count != 2 {
		t.Errorf("Expected 2 active transfers, got %d", count)
	}
	
	// Complete one transfer
	manager.CompleteTransfer(filePaths[0])
	
	// Should be 1 active
	count = manager.GetActiveTransferCount()
	if count != 1 {
		t.Errorf("Expected 1 active transfer, got %d", count)
	}
}

func TestTransferStatusManager_ConcurrentAccess(t *testing.T) {
	// Use a config with higher limits for this test
	config := DefaultTransferConfig()
	config.MaxConcurrentTransfers = 100
	manager := NewTransferStatusManagerWithConfig(config)
	
	// Test concurrent access to the manager
	var wg sync.WaitGroup
	numGoroutines := 10
	transfersPerGoroutine := 5
	
	// Start multiple goroutines that create transfers
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()
			
			for j := 0; j < transfersPerGoroutine; j++ {
				filePath := fmt.Sprintf("/test/goroutine%d_file%d.txt", goroutineID, j)
				_, err := manager.StartTransfer(filePath, 1024)
				if err != nil {
					t.Errorf("StartTransfer failed: %v", err)
					return
				}
				
				// Update progress
				err = manager.UpdateProgress(filePath, 512)
				if err != nil {
					t.Errorf("UpdateProgress failed: %v", err)
					return
				}
				
				// Get status
				_, err = manager.GetTransferStatus(filePath)
				if err != nil {
					t.Errorf("GetTransferStatus failed: %v", err)
					return
				}
			}
		}(i)
	}
	
	wg.Wait()
	
	// Verify all transfers were created
	expectedCount := numGoroutines * transfersPerGoroutine
	actualCount := manager.GetTransferCount()
	if actualCount != expectedCount {
		t.Errorf("Expected %d transfers, got %d", expectedCount, actualCount)
	}
}

func TestTransferStatusManager_MaxConcurrentTransfers(t *testing.T) {
	config := DefaultTransferConfig()
	config.MaxConcurrentTransfers = 2
	manager := NewTransferStatusManagerWithConfig(config)
	
	// Start transfers up to the limit
	filePaths := []string{"/test/file1.txt", "/test/file2.txt", "/test/file3.txt"}
	
	// First two should succeed
	for i := 0; i < 2; i++ {
		_, err := manager.StartTransfer(filePaths[i], 1024)
		if err != nil {
			t.Fatalf("StartTransfer %d failed: %v", i, err)
		}
	}
	
	// Third should fail due to limit
	_, err := manager.StartTransfer(filePaths[2], 1024)
	if err == nil {
		t.Error("Expected error due to concurrent transfer limit")
	}
	if !errors.Is(err, ErrMaxTransfersExceeded) {
		t.Errorf("Expected ErrMaxTransfersExceeded, got %v", err)
	}
}

func TestTransferStatusManager_Clear(t *testing.T) {
	manager := NewTransferStatusManager()
	
	// Add some transfers
	filePaths := []string{"/test/file1.txt", "/test/file2.txt"}
	for _, filePath := range filePaths {
		_, err := manager.StartTransfer(filePath, 1024)
		if err != nil {
			t.Fatalf("StartTransfer failed for %s: %v", filePath, err)
		}
	}
	
	// Verify transfers exist
	if manager.GetTransferCount() != 2 {
		t.Errorf("Expected 2 transfers before clear, got %d", manager.GetTransferCount())
	}
	
	// Clear all transfers
	manager.Clear()
	
	// Verify all transfers are removed
	if manager.GetTransferCount() != 0 {
		t.Errorf("Expected 0 transfers after clear, got %d", manager.GetTransferCount())
	}
	
	transfers := manager.GetAllTransfers()
	if len(transfers) != 0 {
		t.Errorf("Expected empty transfers list after clear, got %d", len(transfers))
	}
}

func TestTransferStatusManager_UpdateConfig(t *testing.T) {
	manager := NewTransferStatusManager()
	
	// Test updating with valid config
	newConfig := &TransferConfig{
		ChunkSize:              32 * 1024,
		MinChunkSize:           MinChunkSize,
		MaxChunkSize:           MaxChunkSize,
		MaxConcurrentTransfers: 5,
		MaxConcurrentChunks:    25,
		BufferSize:             4096,
		DefaultRetryPolicy:     DefaultRetryPolicy(),
		EventBufferSize:        50,
	}
	
	err := manager.UpdateConfig(newConfig)
	if err != nil {
		t.Errorf("UpdateConfig failed: %v", err)
	}
	
	// Verify config was updated
	currentConfig := manager.GetConfig()
	if currentConfig.ChunkSize != newConfig.ChunkSize {
		t.Errorf("Expected ChunkSize %d, got %d", newConfig.ChunkSize, currentConfig.ChunkSize)
	}
	
	// Test updating with nil config
	err = manager.UpdateConfig(nil)
	if err == nil {
		t.Error("Expected error for nil config")
	}
	if !errors.Is(err, ErrInvalidConfiguration) {
		t.Errorf("Expected ErrInvalidConfiguration, got %v", err)
	}
	
	// Test updating with invalid config
	invalidConfig := &TransferConfig{
		ChunkSize: -1, // Invalid
	}
	
	err = manager.UpdateConfig(invalidConfig)
	if err == nil {
		t.Error("Expected error for invalid config")
	}
	if !errors.Is(err, ErrInvalidConfiguration) {
		t.Errorf("Expected ErrInvalidConfiguration, got %v", err)
	}
}