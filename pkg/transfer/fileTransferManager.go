package transfer

import (
	"context"
	"fmt"
	"runtime"
	"sync"

	"github.com/rescp17/lanFileSharer/pkg/fileInfo"
	"golang.org/x/sync/semaphore"
)

const (
	// MaxSupportedFiles defines the maximum number of files that can be managed
	// This prevents potential integer overflow and memory issues
	MaxSupportedFiles = 1000000
)

type FileTransferManager struct {
	chunkers        map[string]*Chunker
	mu              sync.RWMutex
	maxConcurrency  int64  // Dynamic concurrency limit
}

func NewFileTransferManager() *FileTransferManager {
	return &FileTransferManager{
		chunkers:       make(map[string]*Chunker),
		maxConcurrency: calculateOptimalConcurrency(),
	}
}

// calculateOptimalConcurrency dynamically determines the optimal concurrency level
func calculateOptimalConcurrency() int64 {
	numCPU := runtime.NumCPU()
	
	// Base concurrency on CPU count with intelligent scaling
	var concurrency int64
	
	switch {
	case numCPU <= 2:
		// Low-end systems: conservative approach
		concurrency = int64(numCPU * 2)
	case numCPU <= 8:
		// Mid-range systems: moderate scaling
		concurrency = int64(numCPU * 3)
	case numCPU <= 16:
		// High-end systems: aggressive scaling
		concurrency = int64(numCPU * 2)
	default:
		// Server-class systems: cap to prevent resource exhaustion
		concurrency = 32
	}
	
	// Ensure minimum and maximum bounds
	if concurrency < 2 {
		concurrency = 2
	}
	if concurrency > 64 {
		concurrency = 64
	}
	
	return concurrency
}

func (ftm *FileTransferManager) AddFileNode(node *fileInfo.FileNode) error {
	if node == nil {
		return fmt.Errorf("node cannot be nil")
	}

	if node.IsDir {
		return ftm.processDirConcurrent(node)
	}
	return ftm.addSingleFileWithLimitCheck(node)
}

// addSingleFileWithLimitCheck atomically checks the limit and adds the file
func (ftm *FileTransferManager) addSingleFileWithLimitCheck(node *fileInfo.FileNode) error {
	chunker, err := NewChunkerFromFileNode(node, int32(DefaultChunkSize))
	if err != nil {
		return err
	}

	ftm.mu.Lock()
	defer ftm.mu.Unlock()

	// Atomic check and add under the same lock
	if len(ftm.chunkers) >= MaxSupportedFiles {
		chunker.Close() // Clean up the chunker we created
		return fmt.Errorf("file limit exceeded: maximum %d files supported", MaxSupportedFiles)
	}

	if oldChunker, exists := ftm.chunkers[node.Path]; exists {
		oldChunker.Close()
	}
	ftm.chunkers[node.Path] = chunker
	return nil
}

func (ftm *FileTransferManager) processDirConcurrent(node *fileInfo.FileNode) error {
	// Adaptive concurrency based on workload characteristics
	concurrency := ftm.calculateAdaptiveConcurrency(node)
	semaphore := semaphore.NewWeighted(concurrency)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var wg sync.WaitGroup
	errChan := make(chan error, len(node.Children))

	for i := range node.Children {
		wg.Add(1)

		go func(child *fileInfo.FileNode) {
			defer wg.Done()
			if err := semaphore.Acquire(ctx, 1); err != nil {
				if err != context.Canceled {
					errChan <- err
				}
				return
			}
			defer semaphore.Release(1)

			select {
			case <-ctx.Done():
				return
			default:
			}

			if child.IsDir {
				if err := ftm.AddFileNode(child); err != nil {
					errChan <- err
					cancel()
					return
				}
			} else {
				if err := ftm.addSingleFileWithLimitCheck(child); err != nil {
					errChan <- err
					cancel()
					return
				}
			}
		}(&node.Children[i])

	}
	wg.Wait()
	close(errChan)

	for err := range errChan {
		if err != nil {
			return err
		}
	}

	return nil
}

// calculateAdaptiveConcurrency determines optimal concurrency based on workload
func (ftm *FileTransferManager) calculateAdaptiveConcurrency(node *fileInfo.FileNode) int64 {
	ftm.mu.RLock()
	baseConcurrency := ftm.maxConcurrency
	ftm.mu.RUnlock()
	
	childCount := int64(len(node.Children))
	
	// Adaptive scaling based on workload size
	var adaptiveConcurrency int64
	
	switch {
	case childCount <= 10:
		// Small workload: use fewer goroutines to reduce overhead
		adaptiveConcurrency = min(baseConcurrency/2, childCount)
	case childCount <= 100:
		// Medium workload: use moderate concurrency
		adaptiveConcurrency = min(baseConcurrency, childCount)
	case childCount <= 1000:
		// Large workload: use full concurrency
		adaptiveConcurrency = baseConcurrency
	default:
		// Very large workload: may need to cap to prevent resource exhaustion
		adaptiveConcurrency = baseConcurrency
	}
	
	// Ensure minimum concurrency
	if adaptiveConcurrency < 1 {
		adaptiveConcurrency = 1
	}
	
	return adaptiveConcurrency
}

// min returns the minimum of two int64 values
func min(a, b int64) int64 {
	if a < b {
		return a
	}
	return b
}

func (ftm *FileTransferManager) GetChunker(filePath string) (*Chunker, bool) {
	ftm.mu.RLock()
	chunker, exists := ftm.chunkers[filePath]
	ftm.mu.RUnlock()
	return chunker, exists
}

// SetMaxConcurrency allows runtime adjustment of concurrency level
func (ftm *FileTransferManager) SetMaxConcurrency(maxConcurrency int64) {
	if maxConcurrency < 1 {
		maxConcurrency = 1
	}
	if maxConcurrency > 128 {
		maxConcurrency = 128
	}
	
	ftm.mu.Lock()
	ftm.maxConcurrency = maxConcurrency
	ftm.mu.Unlock()
}

// GetMaxConcurrency returns the current concurrency limit
func (ftm *FileTransferManager) GetMaxConcurrency() int64 {
	ftm.mu.RLock()
	defer ftm.mu.RUnlock()
	return ftm.maxConcurrency
}

// GetStats returns current statistics about the file transfer manager
func (ftm *FileTransferManager) GetStats() map[string]interface{} {
	ftm.mu.RLock()
	defer ftm.mu.RUnlock()
	
	return map[string]interface{}{
		"total_files":      len(ftm.chunkers),
		"max_concurrency":  ftm.maxConcurrency,
		"cpu_count":        runtime.NumCPU(),
		"goroutines":       runtime.NumGoroutine(),
	}
}

func (ftm *FileTransferManager) Close() error {
	ftm.mu.Lock()
	defer ftm.mu.Unlock()

	for _, chunker := range ftm.chunkers {
		chunker.Close()
	}
	ftm.chunkers = make(map[string]*Chunker)
	return nil
}
