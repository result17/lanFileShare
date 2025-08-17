package transfer

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ExampleBadTestIsolation demonstrates the problems with shared state in tests
// This is an example of what NOT to do - it shows how state leakage can cause tests to fail
func ExampleBadTestIsolation(t *testing.T) {
	// BAD: Shared manager instance across all sub-tests
	manager := NewTransferStatusManager()

	tests := []struct {
		name       string
		sessionID  string
		totalFiles int
		totalBytes int64
	}{
		{"session1", "test-1", 1, 1000},
		{"session2", "test-2", 2, 2000},
		{"session3", "test-3", 3, 3000},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// This test will fail because the manager already has a session
			// from the previous sub-test iteration
			err := manager.InitializeSession(test.sessionID, test.totalFiles, test.totalBytes)
			
			// The second and third iterations will fail because InitializeSession
			// returns an error when a session already exists
			require.NoError(t, err, "Should be able to initialize session")
			
			status, err := manager.GetSessionStatus()
			require.NoError(t, err)
			assert.Equal(t, test.sessionID, status.SessionID)
		})
	}
}

// ExampleGoodTestIsolation demonstrates proper test isolation
// This is the correct way to write tests with complete isolation
func ExampleGoodTestIsolation(t *testing.T) {
	tests := []struct {
		name       string
		sessionID  string
		totalFiles int
		totalBytes int64
	}{
		{"session1", "test-1", 1, 1000},
		{"session2", "test-2", 2, 2000},
		{"session3", "test-3", 3, 3000},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// GOOD: Fresh manager instance for each sub-test
			manager := NewTransferStatusManager()
			
			err := manager.InitializeSession(test.sessionID, test.totalFiles, test.totalBytes)
			require.NoError(t, err, "Should be able to initialize session")
			
			status, err := manager.GetSessionStatus()
			require.NoError(t, err)
			assert.Equal(t, test.sessionID, status.SessionID)
		})
	}
}

// ExampleStateLeakageDemo demonstrates how state leakage can cause subtle bugs
func ExampleStateLeakageDemo(t *testing.T) {
	// This example shows how shared state can cause tests to pass when they should fail
	// or fail when they should pass, depending on execution order
	
	t.Run("with_shared_state", func(t *testing.T) {
		// BAD: Shared manager
		manager := NewTransferStatusManager()
		
		t.Run("first_test", func(t *testing.T) {
			err := manager.InitializeSession("session-1", 1, 1000)
			require.NoError(t, err)
			
			_, err = manager.StartFileTransfer("/file1.txt", 1000)
			require.NoError(t, err)
			
			// This test passes and leaves the manager in an "active file" state
		})
		
		t.Run("second_test", func(t *testing.T) {
			// This test might behave differently because the manager
			// already has an active file from the previous test
			
			// If we try to start another file transfer without completing the first,
			// it should fail, but the test might not catch this properly
			_, err := manager.StartFileTransfer("/file2.txt", 2000)
			
			// This assertion might fail or pass depending on the implementation
			// and the state left by the previous test
			if err != nil {
				t.Logf("Expected behavior: got error due to active file: %v", err)
			} else {
				t.Error("Unexpected: should have failed due to active file from previous test")
			}
		})
	})
	
	t.Run("with_isolated_state", func(t *testing.T) {
		t.Run("first_test", func(t *testing.T) {
			// GOOD: Fresh manager for each test
			manager := NewTransferStatusManager()
			
			err := manager.InitializeSession("session-1", 1, 1000)
			require.NoError(t, err)
			
			_, err = manager.StartFileTransfer("/file1.txt", 1000)
			require.NoError(t, err)
			
			// This test passes and the manager state is isolated
		})
		
		t.Run("second_test", func(t *testing.T) {
			// GOOD: Fresh manager for this test too
			manager := NewTransferStatusManager()
			
			err := manager.InitializeSession("session-2", 1, 2000)
			require.NoError(t, err)
			
			// This should work fine because we have a clean manager
			_, err = manager.StartFileTransfer("/file2.txt", 2000)
			require.NoError(t, err, "Should work with fresh manager")
		})
	})
}

// TestIsolationBenefits demonstrates the benefits of proper test isolation
func TestIsolationBenefits(t *testing.T) {
	t.Run("parallel_execution", func(t *testing.T) {
		// With proper isolation, tests can run in parallel safely
		tests := []struct {
			name string
			id   string
		}{
			{"test1", "session-1"},
			{"test2", "session-2"},
			{"test3", "session-3"},
		}
		
		for _, test := range tests {
			test := test // Capture range variable
			t.Run(test.name, func(t *testing.T) {
				t.Parallel() // This is safe with proper isolation
				
				manager := NewTransferStatusManager()
				err := manager.InitializeSession(test.id, 1, 1000)
				require.NoError(t, err)
				
				status, err := manager.GetSessionStatus()
				require.NoError(t, err)
				assert.Equal(t, test.id, status.SessionID)
			})
		}
	})
	
	t.Run("order_independence", func(t *testing.T) {
		// With proper isolation, test order doesn't matter
		// These tests can run in any order and still pass
		
		t.Run("z_last_alphabetically", func(t *testing.T) {
			manager := NewTransferStatusManager()
			err := manager.InitializeSession("last", 1, 1000)
			require.NoError(t, err)
		})
		
		t.Run("a_first_alphabetically", func(t *testing.T) {
			manager := NewTransferStatusManager()
			err := manager.InitializeSession("first", 1, 1000)
			require.NoError(t, err)
		})
		
		t.Run("m_middle_alphabetically", func(t *testing.T) {
			manager := NewTransferStatusManager()
			err := manager.InitializeSession("middle", 1, 1000)
			require.NoError(t, err)
		})
	})
}

// TestIsolationBestPractices demonstrates best practices for test isolation
func TestIsolationBestPractices(t *testing.T) {
	t.Run("use_setup_helpers", func(t *testing.T) {
		// Helper function to create a properly initialized manager
		setupManager := func(sessionID string) *TransferStatusManager {
			manager := NewTransferStatusManager()
			err := manager.InitializeSession(sessionID, 1, 1000)
			require.NoError(t, err)
			return manager
		}
		
		t.Run("test_with_helper", func(t *testing.T) {
			manager := setupManager("test-session")
			
			status, err := manager.GetSessionStatus()
			require.NoError(t, err)
			assert.Equal(t, "test-session", status.SessionID)
		})
	})
	
	t.Run("cleanup_resources", func(t *testing.T) {
		manager := NewTransferStatusManager()
		
		// Use t.Cleanup for proper resource cleanup
		t.Cleanup(func() {
			// Any cleanup code would go here
			// For our manager, it doesn't need explicit cleanup,
			// but this is where you'd close files, connections, etc.
		})
		
		err := manager.InitializeSession("cleanup-test", 1, 1000)
		require.NoError(t, err)
	})
}
